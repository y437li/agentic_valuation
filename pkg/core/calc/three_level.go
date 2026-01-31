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

	// ROCE Decomposition (Penman Style: Operating vs Financing)
	ROCEAnalysis *ROCEDecomposition `json:"roce_analysis,omitempty"`
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
	CurrentRatio     float64              `json:"current_ratio"`
	QuickRatio       float64              `json:"quick_ratio"`
	DebtToEquity     float64              `json:"debt_to_equity"`
	InterestCoverage float64              `json:"interest_coverage"`
	AltmanZScore     float64              `json:"altman_z_score"`
	BeneishMScore    *BeneishMScoreResult `json:"beneish_m_score,omitempty"`
}

type ROCEDecomposition struct {
	// Input Variables
	NOPAT                       float64 `json:"nopat"`
	NetFinancingExpenseAfterTax float64 `json:"net_financing_expense_after_tax"`
	AverageNOA                  float64 `json:"average_noa"`             // Net Operating Assets
	AverageFinObligations       float64 `json:"average_fin_obligations"` // Net Debt
	AverageCommonEquity         float64 `json:"average_common_equity"`

	// Derived Metrics
	OperatingROA        float64 `json:"operating_roa"`          // NOPAT / Avg NOA
	ProfitMarginForROCE float64 `json:"profit_margin_for_roce"` // NOPAT / Revenues
	AssetTurnover       float64 `json:"asset_turnover"`         // Revenues / Avg NOA
	NetBorrowingRate    float64 `json:"net_borrowing_rate"`     // Net Fin Exp / Avg Fin Obs
	Spread              float64 `json:"spread"`                 // Op ROA - Net Borrowing Rate
	Leverage            float64 `json:"leverage"`               // Avg Fin Obs / Avg Equity

	// Final Result
	ROCE float64 `json:"roce"` // Operating ROA + (Leverage * Spread)
}

// PerformThreeLevelAnalysis runs the full diagnostic for a specific year
func PerformThreeLevelAnalysis(current *edgar.FSAPDataResponse, prior *edgar.FSAPDataResponse) *ThreeLevelAnalysis {
	if current == nil {
		return nil
	}

	analysis := &ThreeLevelAnalysis{}

	// --- Level 1: Growth ---
	if prior != nil {
		analysis.Level1.RevenueGrowth = calcGrowth(getVal(current.IncomeStatement.GrossProfitSection.Revenues), getVal(prior.IncomeStatement.GrossProfitSection.Revenues))
		analysis.Level1.OperatingIncomeGrowth = calcGrowth(getVal(current.IncomeStatement.OperatingCostSection.OperatingIncome), getVal(prior.IncomeStatement.OperatingCostSection.OperatingIncome))
		analysis.Level1.NetIncomeGrowth = calcGrowth(getVal(current.IncomeStatement.NetIncomeSection.NetIncomeToCommon), getVal(prior.IncomeStatement.NetIncomeSection.NetIncomeToCommon))
		analysis.Level1.EPSGrowth = calcGrowth(getVal(current.SupplementalData.EPSDiluted), getVal(prior.SupplementalData.EPSDiluted))

		// Free Cash Flow Approx: NetCashOperating - Capex
		currFCF := getVal(current.CashFlowStatement.CashSummary.NetCashOperating) - getVal(current.CashFlowStatement.InvestingActivities.Capex)
		priorFCF := getVal(prior.CashFlowStatement.CashSummary.NetCashOperating) - getVal(prior.CashFlowStatement.InvestingActivities.Capex)
		analysis.Level1.FCFGrowth = calcGrowth(currFCF, priorFCF)
	}

	// --- Level 2: Returns (DuPont) ---
	rev := getVal(current.IncomeStatement.GrossProfitSection.Revenues)
	netIncome := getVal(current.IncomeStatement.NetIncomeSection.NetIncomeToCommon)

	totalAssets := getVal(current.BalanceSheet.ReportedForValidation.TotalAssets)
	totalEquity := getVal(current.BalanceSheet.ReportedForValidation.TotalEquity)

	avgAssets := totalAssets
	avgEquity := totalEquity

	if prior != nil {
		avgAssets = (totalAssets + getVal(prior.BalanceSheet.ReportedForValidation.TotalAssets)) / 2
		avgEquity = (totalEquity + getVal(prior.BalanceSheet.ReportedForValidation.TotalEquity)) / 2
	}

	analysis.Level2.GrossMargin = safeDiv(getVal(current.IncomeStatement.GrossProfitSection.GrossProfit), rev)
	analysis.Level2.OperatingMargin = safeDiv(getVal(current.IncomeStatement.OperatingCostSection.OperatingIncome), rev)
	analysis.Level2.NetMargin = safeDiv(netIncome, rev)
	analysis.Level2.AssetTurnover = safeDiv(rev, avgAssets)
	analysis.Level2.FinancialLeverage = safeDiv(avgAssets, avgEquity)
	analysis.Level2.ROA = analysis.Level2.NetMargin * analysis.Level2.AssetTurnover
	analysis.Level2.ROE = analysis.Level2.ROA * analysis.Level2.FinancialLeverage // DuPont Identity

	// Simple ROIC proxy: NOPAT / (Equity + Debt - Cash)
	ebit := getVal(current.IncomeStatement.OperatingCostSection.OperatingIncome)
	taxExp := getVal(current.IncomeStatement.TaxAdjustments.IncomeTaxExpense)
	preTaxIncome := getVal(current.IncomeStatement.NonOperatingSection.IncomeBeforeTax)

	effectiveTaxRate := 0.21 // Default
	if preTaxIncome != 0 {
		effectiveTaxRate = math.Abs(taxExp / preTaxIncome)
	}
	// Cap tax rate at reasonable bounds [0, 1] for modeling
	if effectiveTaxRate < 0 {
		effectiveTaxRate = 0
	}
	if effectiveTaxRate > 0.4 {
		effectiveTaxRate = 0.4
	} // Cap at 40% to avoid outliers

	nopat := ebit * (1 - effectiveTaxRate)

	debt := getVal(current.BalanceSheet.NoncurrentLiabilities.LongTermDebt) + getVal(current.BalanceSheet.CurrentLiabilities.NotesPayableShortTermDebt)
	cash := getVal(current.BalanceSheet.CurrentAssets.CashAndEquivalents)
	investedCapital := avgEquity + debt - cash
	analysis.Level2.ROIC = safeDiv(nopat, investedCapital)

	// --- Level 3: Risk ---
	ca := getVal(current.BalanceSheet.ReportedForValidation.TotalCurrentAssets)
	cl := getVal(current.BalanceSheet.ReportedForValidation.TotalCurrentLiabilities)
	inv := getVal(current.BalanceSheet.CurrentAssets.Inventories)
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

	// Beneish M-Score
	analysis.Level3.BeneishMScore = CalculateBeneishMScore(current, prior)

	// --- ROCE Decomposition (Penman) ---
	// Need Prior data for averages. If no prior, use current.

	// 1. Calculate NOA and Net Debt for Current and Prior
	calcNetDebt := func(d *edgar.FSAPDataResponse) float64 {
		totalDebt := getVal(d.BalanceSheet.NoncurrentLiabilities.LongTermDebt) + getVal(d.BalanceSheet.CurrentLiabilities.NotesPayableShortTermDebt)
		c := getVal(d.BalanceSheet.CurrentAssets.CashAndEquivalents)
		return totalDebt - c
	}

	calcEquity := func(d *edgar.FSAPDataResponse) float64 {
		return getVal(d.BalanceSheet.ReportedForValidation.TotalEquity)
	}

	currNetDebt := calcNetDebt(current)
	currEquity := calcEquity(current)
	currNOA := currEquity + currNetDebt // Accounting Identity: NOA = Equity + Net Financial Obligations

	var avgNOA, avgFinObs, avgCommEquity float64

	if prior != nil {
		prevNetDebt := calcNetDebt(prior)
		prevEquity := calcEquity(prior)
		prevNOA := prevEquity + prevNetDebt

		avgNOA = (currNOA + prevNOA) / 2
		avgFinObs = (currNetDebt + prevNetDebt) / 2
		avgCommEquity = (currEquity + prevEquity) / 2
	} else {
		avgNOA = currNOA
		avgFinObs = currNetDebt
		avgCommEquity = currEquity
	}

	// 2. Metrics
	// NOPAT calculated above

	// Net Financing Expense After Tax
	// Interest Expense is usually negative in our FSAP. Make it positive for "Expense" concept.
	// Interest Income is positive.
	// Net Financing costs ~ -InterestExpense - InterestIncome.
	// If the line item is "NetInterestExpense" (checking schema):
	// In types it's under NonOperatingSection.InterestExpense. Usually net.

	netInterest := getVal(current.IncomeStatement.NonOperatingSection.InterestExpense)
	// If negative (expense), abs it.
	netFinancingExpPreTax := math.Abs(netInterest)
	netFinancingExpAfterTax := netFinancingExpPreTax * (1 - effectiveTaxRate)

	opROA := safeDiv(nopat, avgNOA)
	// Specific DuPont-style decomposition of RNOA
	pmROCE := safeDiv(nopat, rev)
	turnover := safeDiv(rev, avgNOA)

	netBorrowRate := safeDiv(netFinancingExpAfterTax, avgFinObs)
	leverage := safeDiv(avgFinObs, avgCommEquity)
	spread := opROA - netBorrowRate

	roce := opROA + (leverage * spread)

	analysis.ROCEAnalysis = &ROCEDecomposition{
		NOPAT:                       nopat,
		NetFinancingExpenseAfterTax: netFinancingExpAfterTax,
		AverageNOA:                  avgNOA,
		AverageFinObligations:       avgFinObs,
		AverageCommonEquity:         avgCommEquity,
		OperatingROA:                opROA,
		ProfitMarginForROCE:         pmROCE,
		AssetTurnover:               turnover,
		NetBorrowingRate:            netBorrowRate,
		Spread:                      spread,
		Leverage:                    leverage,
		ROCE:                        roce,
	}

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
