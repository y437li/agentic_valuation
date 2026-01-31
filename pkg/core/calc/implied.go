package calc

import (
	"agentic_valuation/pkg/core/edgar"
	"math"
)

// ImpliedMetrics holds "Back-Solved" technical assumptions derived from historical data
type ImpliedMetrics struct {
	ImpliedUsefulLife     float64 `json:"implied_useful_life"`     // Gross PP&E / Depreciation Expense
	EffectiveTaxRate      float64 `json:"effective_tax_rate"`      // Income Tax Expense / Income Before Tax
	ImpliedInterestRate   float64 `json:"implied_interest_rate"`   // Interest Expense / Total Debt
	CapexPercentRevenue   float64 `json:"capex_percent_revenue"`   // Capex / Revenue
	DepreciationRateGross float64 `json:"depreciation_rate_gross"` // Depreciation / Gross PP&E (inverse of useful life concept)
}

// CalculateImpliedMetrics computes all derived technical assumptions for a given year
func CalculateImpliedMetrics(data *edgar.FSAPDataResponse) ImpliedMetrics {
	m := ImpliedMetrics{}

	// 1. Implied Useful Life (Gross PP&E / Depreciation)
	// We need Gross PP&E (PPE At Cost) and Depreciation Expense
	grossPPE := getValue(data.BalanceSheet.NoncurrentAssets.PPEAtCost)

	// Depreciation: try Cash Flow first (most accurate for 'actual' depreciation added back), then Supplemental, then IS
	depExp := 0.0
	if data.CashFlowStatement.OperatingActivities != nil {
		depExp = getValue(data.CashFlowStatement.OperatingActivities.DepreciationAmortization)
	}
	if depExp == 0 && data.SupplementalData.DepreciationExpense != nil {
		depExp = getValue(data.SupplementalData.DepreciationExpense)
	}

	if depExp > 0 && grossPPE > 0 {
		m.ImpliedUsefulLife = grossPPE / depExp // e.g., 79132 / 2986 = 26.5 years
		m.DepreciationRateGross = depExp / grossPPE
	}

	// 2. Effective Tax Rate (Tax / EBT)
	ebt := 0.0
	if data.IncomeStatement.NonOperatingSection != nil {
		ebt = getValue(data.IncomeStatement.NonOperatingSection.IncomeBeforeTax)
	}
	taxExp := 0.0
	if data.IncomeStatement.TaxAdjustments != nil {
		taxExp = math.Abs(getValue(data.IncomeStatement.TaxAdjustments.IncomeTaxExpense))
	}
	// Fallback to top-level if sections missing
	// Fallback logic removed - fields exist in sections
	if ebt != 0 {
		m.EffectiveTaxRate = taxExp / ebt
	}

	// 3. Implied Interest Rate (Interest Exp / Total Debt)
	intExp := 0.0
	if data.IncomeStatement.NonOperatingSection != nil {
		intExp = math.Abs(getValue(data.IncomeStatement.NonOperatingSection.InterestExpense))
		intExp = math.Abs(getValue(data.IncomeStatement.NonOperatingSection.InterestExpense))
	}

	totalDebt := 0.0
	// Current Debt
	totalDebt += getValue(data.BalanceSheet.CurrentLiabilities.NotesPayableShortTermDebt)
	totalDebt += getValue(data.BalanceSheet.CurrentLiabilities.CurrentMaturitiesLTD)
	// Long Term Debt
	totalDebt += getValue(data.BalanceSheet.NoncurrentLiabilities.LongTermDebt)

	if totalDebt > 0 {
		m.ImpliedInterestRate = intExp / totalDebt
	}

	// 4. Capex % Revenue
	revenue := 0.0
	if data.IncomeStatement.GrossProfitSection != nil {
		revenue = getValue(data.IncomeStatement.GrossProfitSection.Revenues)
		revenue = getValue(data.IncomeStatement.GrossProfitSection.Revenues)
	}

	capex := 0.0
	if data.CashFlowStatement.InvestingActivities != nil {
		capex = math.Abs(getValue(data.CashFlowStatement.InvestingActivities.Capex))
	}

	if revenue > 0 {
		m.CapexPercentRevenue = capex / revenue
	}

	return m
}
