package calc

import (
	"agentic_valuation/pkg/core/edgar"
)

// NormalizeIncomeStatementSigns ensures all expense items are stored as negative values.
// This allows the calculation layer to simply sum values: GrossProfit = Revenue + COGS (where COGS is negative)
// Call this after LLM extraction and before calculation.
func NormalizeIncomeStatementSigns(is *edgar.IncomeStatement) {
	if is == nil {
		return
	}

	// Section 1: Gross Profit - COGS should be negative
	if is.GrossProfitSection != nil {
		gps := is.GrossProfitSection
		makeNegative(gps.CostOfGoodsSold)
	}

	// Section 2: Operating Costs - all expenses should be negative
	if is.OperatingCostSection != nil {
		ops := is.OperatingCostSection
		makeNegative(ops.SGAExpenses)
		makeNegative(ops.RDExpenses)
		makeNegative(ops.AdvertisingExpenses)
		makeNegative(ops.OtherOperatingExpenses)
		for i := range ops.AdditionalItems {
			makeNegativeAdditional(&ops.AdditionalItems[i])
		}
	}

	// Section 3: Non-Operating - Interest Expense should be negative
	if is.NonOperatingSection != nil {
		nos := is.NonOperatingSection
		makeNegative(nos.InterestExpense)
		// OtherIncomeExpense can be +/-, keep as-is
	}

	// Section 4: Tax - Tax Expense should be negative
	if is.TaxAdjustments != nil {
		tas := is.TaxAdjustments
		makeNegative(tas.IncomeTaxExpense)
	}
}

// NormalizeCashFlowSigns ensures outflow items are stored as negative values.
func NormalizeCashFlowSigns(cf *edgar.CashFlowStatement) {
	if cf == nil {
		return
	}

	// Investing Activities - outflows should be negative
	if cf.InvestingActivities != nil {
		inv := cf.InvestingActivities
		makeNegative(inv.Capex)
		makeNegative(inv.AcquisitionsNet)
		makeNegative(inv.PurchasesSecurities)
		// Maturities/Sales are inflows, keep positive
	}

	// Financing Activities - outflows should be negative
	if cf.FinancingActivities != nil {
		fin := cf.FinancingActivities
		makeNegative(fin.DebtRepayments)
		makeNegative(fin.ShareRepurchases)
		makeNegative(fin.DividendsPaid)
		makeNegative(fin.TaxWithholdingPayments)
		// Proceeds are inflows, keep positive
	}
}

// makeNegative ensures the value is negative (for expenses/outflows).
// If value is positive, it makes it negative. If already negative, keeps it.
func makeNegative(item *edgar.FSAPValue) {
	if item == nil {
		return
	}

	// Handle primary value
	if item.Value != nil && *item.Value > 0 {
		neg := -*item.Value
		item.Value = &neg
	}

	// Handle Years map
	for year, val := range item.Years {
		if val > 0 {
			item.Years[year] = -val
		}
	}
}

// makeNegativeAdditional handles AdditionalItem sign normalization
// Supports both {value: {years}} and direct {years} formats from LLM
func makeNegativeAdditional(item *edgar.AdditionalItem) {
	if item == nil {
		return
	}
	// Handle wrapped format: {value: {years}}
	if item.Value != nil {
		makeNegative(item.Value)
	}
	// Handle direct format: {years} (LLM sometimes outputs this)
	for year, val := range item.Years {
		if val > 0 {
			item.Years[year] = -val
		}
	}
}
