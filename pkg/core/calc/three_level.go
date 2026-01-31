package calc

import (
	"agentic_valuation/pkg/core/edgar"
	"math"
)

// ThreeLevelAnalysis aggregates financial health across three depths:
// Level 1: Growth & Momentum
// Level 2: Efficiency & Returns (DuPont)
// Level 3: Risk & Solvency
type ThreeLevelAnalysis struct {
	Level1 Level1Growth `json:"level_1_growth"`
	Level2 Level2Return `json:"level_2_return"` // DuPont & Margins
	Level3 Level3Risk   `json:"level_3_risk"`   // Liquidity & Solvency
}

type Level1Growth struct {
	RevenueGrowth         float64 `json:"revenue_growth"`
	OperatingIncomeGrowth float64 `json:"operating_income_growth"`
	NetIncomeGrowth       float64 `json:"net_income_growth"`
	EPSGrowth             float64 `json:"eps_growth"`
	FCFGrowth             float64 `json:"fcf_growth"`
}

type Level2Return struct {
	GrossMargin       float64 `json:"gross_margin"`
	OperatingMargin   float64 `json:"operating_margin"`
	NetMargin         float64 `json:"net_margin"`
	AssetTurnover     float64 `json:"asset_turnover"`
	FinancialLeverage float64 `json:"financial_leverage"`
	ROA               float64 `json:"roa"`
	ROE               float64 `json:"roe"` // DuPont Identity
	ROIC              float64 `json:"roic"`
}

type Level3Risk struct {
	CurrentRatio     float64 `json:"current_ratio"`
	QuickRatio       float64 `json:"quick_ratio"`
	DebtToEquity     float64 `json:"debt_to_equity"`
	InterestCoverage float64 `json:"interest_coverage"`
	AltmanZScore     float64 `json:"altman_z_score"`  // Manufacturing Z
	BeneishMScore    float64 `json:"beneish_m_score"` // Future Implementation
}

// PerformThreeLevelAnalysis runs the full diagnostic for a specific year
func PerformThreeLevelAnalysis(current *edgar.FSAPDataResponse, prior *edgar.FSAPDataResponse) *ThreeLevelAnalysis {
	if current == nil {
		return nil
	}

	analysis := &ThreeLevelAnalysis{}

	// --- Level 1: Growth ---
	if prior != nil {
		analysis.Level1.RevenueGrowth = calcGrowth(getVal(current.IncomeStatement.Revenues), getVal(prior.IncomeStatement.Revenues))
		analysis.Level1.OperatingIncomeGrowth = calcGrowth(getVal(current.IncomeStatement.OperatingCostSection.OperatingIncome), getVal(prior.IncomeStatement.OperatingCostSection.OperatingIncome))
		analysis.Level1.NetIncomeGrowth = calcGrowth(getVal(current.IncomeStatement.NetIncomeSection.NetIncomeToCommon), getVal(prior.IncomeStatement.NetIncomeSection.NetIncomeToCommon))
		analysis.Level1.EPSGrowth = calcGrowth(getVal(current.SupplementalData.EPSDiluted), getVal(prior.SupplementalData.EPSDiluted))

		// Free Cash Flow Approx: NetCashOperating - Capex
		currFCF := getVal(current.CashFlowStatement.CashSummary.NetCashOperating) - getVal(current.CashFlowStatement.InvestingActivities.Capex)
		priorFCF := getVal(prior.CashFlowStatement.CashSummary.NetCashOperating) - getVal(prior.CashFlowStatement.InvestingActivities.Capex)
		analysis.Level1.FCFGrowth = calcGrowth(currFCF, priorFCF)
	}

	// --- Level 2: Returns (DuPont) ---
	rev := getVal(current.IncomeStatement.Revenues)
	netIncome := getVal(current.IncomeStatement.NetIncomeSection.NetIncomeToCommon)
	avgAssets := getVal(current.BalanceSheet.ReportedForValidation.TotalAssets) // Should strictly be average
	avgEquity := getVal(current.BalanceSheet.ReportedForValidation.TotalEquity) // Should strictly be average

	if prior != nil {
		avgAssets = (getVal(current.BalanceSheet.ReportedForValidation.TotalAssets) + getVal(prior.BalanceSheet.ReportedForValidation.TotalAssets)) / 2
		avgEquity = (getVal(current.BalanceSheet.ReportedForValidation.TotalEquity) + getVal(prior.BalanceSheet.ReportedForValidation.TotalEquity)) / 2
	}

	analysis.Level2.GrossMargin = safeDiv(getVal(current.IncomeStatement.GrossProfitSection.GrossProfit), rev)
	analysis.Level2.OperatingMargin = safeDiv(getVal(current.IncomeStatement.OperatingCostSection.OperatingIncome), rev)
	analysis.Level2.NetMargin = safeDiv(netIncome, rev)
	analysis.Level2.AssetTurnover = safeDiv(rev, avgAssets)
	analysis.Level2.FinancialLeverage = safeDiv(avgAssets, avgEquity)
	analysis.Level2.ROA = analysis.Level2.NetMargin * analysis.Level2.AssetTurnover
	analysis.Level2.ROE = analysis.Level2.ROA * analysis.Level2.FinancialLeverage // DuPont Identity

	// Simple ROIC proxy: NOPAT / (Equity + Debt - Cash)
	nopat := getVal(current.IncomeStatement.OperatingCostSection.OperatingIncome) * (1 - 0.21) // Assume 21% Tax
	debt := getVal(current.BalanceSheet.NoncurrentLiabilities.LongTermDebt) + getVal(current.BalanceSheet.CurrentLiabilities.NotesPayableShortTermDebt)
	cash := getVal(current.BalanceSheet.CurrentAssets.CashAndEquivalents)
	investedCapital := avgEquity + debt - cash
	analysis.Level2.ROIC = safeDiv(nopat, investedCapital)

	// --- Level 3: Risk ---
	ca := getVal(current.BalanceSheet.ReportedForValidation.TotalCurrentAssets)
	cl := getVal(current.BalanceSheet.ReportedForValidation.TotalCurrentLiabilities)
	inv := getVal(current.BalanceSheet.CurrentAssets.Inventories)
	ebit := getVal(current.IncomeStatement.OperatingCostSection.OperatingIncome)
	interest := getVal(current.IncomeStatement.NonOperatingSection.InterestExpense)
	re := getVal(current.BalanceSheet.Equity.RetainedEarningsDeficit)
	ta := getVal(current.BalanceSheet.ReportedForValidation.TotalAssets)
	tl := getVal(current.BalanceSheet.ReportedForValidation.TotalLiabilities)

	// Market Value of Equity
	shares := getVal(current.SupplementalData.SharesOutstandingDiluted)
	price := getPtrVal(current.SupplementalData.SharePriceYearEnd)
	mve := shares * price
	if mve == 0 {
		mve = getVal(current.BalanceSheet.ReportedForValidation.TotalEquity) // Fallback to Book Value
	}

	analysis.Level3.CurrentRatio = safeDiv(ca, cl)
	analysis.Level3.QuickRatio = safeDiv(ca-inv, cl)
	analysis.Level3.DebtToEquity = safeDiv(debt, getVal(current.BalanceSheet.ReportedForValidation.TotalEquity))
	analysis.Level3.InterestCoverage = safeDiv(ebit, math.Abs(interest)) // Interest often negative

	// Altman Z-Score
	// WC = CA - CL
	wc := ca - cl
	analysis.Level3.AltmanZScore = AltmanZScore(wc, re, ebit, mve, rev, ta, tl)

	return analysis
}

// Helper to get float from FSAPValue pointer
func getVal(v *edgar.FSAPValue) float64 {
	if v == nil || v.Value == nil {
		return 0
	}
	return *v.Value
}

// Helper for float pointer
func getPtrVal(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func calcGrowth(curr, prior float64) float64 {
	if prior == 0 {
		return 0
	}
	return (curr - prior) / math.Abs(prior)
}
