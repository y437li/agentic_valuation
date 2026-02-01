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
		analysis.Level1.RevenueGrowth = calcGrowth(getRevenue(current), getRevenue(prior))
		analysis.Level1.OperatingIncomeGrowth = calcGrowth(getOpIncome(current), getOpIncome(prior))
		analysis.Level1.NetIncomeGrowth = calcGrowth(getNetIncome(current), getNetIncome(prior))
		analysis.Level1.EPSGrowth = calcGrowth(getEPS(current), getEPS(prior))

		// Free Cash Flow Approx: NetCashOperating - Capex
		currFCF := getNetCashOperating(current) - getCapex(current)
		priorFCF := getNetCashOperating(prior) - getCapex(prior)
		analysis.Level1.FCFGrowth = calcGrowth(currFCF, priorFCF)
	}

	// --- Level 2: Returns (DuPont) ---
	rev := getRevenue(current)
	netIncome := getNetIncome(current)

	totalAssets := getTotalAssets(current)
	totalEquity := getTotalEquity(current)

	avgAssets := totalAssets
	avgEquity := totalEquity

	if prior != nil {
		avgAssets = (totalAssets + getTotalAssets(prior)) / 2
		avgEquity = (totalEquity + getTotalEquity(prior)) / 2
	}

	analysis.Level2.GrossMargin = safeDiv(getGrossProfit(current), rev)
	analysis.Level2.OperatingMargin = safeDiv(getOpIncome(current), rev)
	analysis.Level2.NetMargin = safeDiv(netIncome, rev)
	analysis.Level2.AssetTurnover = safeDiv(rev, avgAssets)
	analysis.Level2.FinancialLeverage = safeDiv(avgAssets, avgEquity)
	analysis.Level2.ROA = analysis.Level2.NetMargin * analysis.Level2.AssetTurnover
	analysis.Level2.ROE = analysis.Level2.ROA * analysis.Level2.FinancialLeverage // DuPont Identity

	// Simple ROIC proxy: NOPAT / (Equity + Debt - Cash)
	ebit := getOpIncome(current)
	taxExp := getIncomeTaxExpense(current)
	preTaxIncome := getIncomeBeforeTax(current)

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

	debt := getLongTermDebt(current) + getShortTermDebt(current)
	cash := getCash(current)
	investedCapital := avgEquity + debt - cash
	analysis.Level2.ROIC = safeDiv(nopat, investedCapital)

	// --- Level 3: Risk ---
	ca := getTotalCurrentAssets(current)
	cl := getTotalCurrentLiabilities(current)
	inv := getInventory(current)
	interest := getInterestExpense(current)
	re := getRetainedEarnings(current)
	ta := getTotalAssets(current)
	tl := getTotalLiabilities(current)

	// Market Value of Equity
	shares := getSharesOutstanding(current)
	price := getSharePrice(current)
	mve := shares * price
	if mve == 0 {
		mve = getTotalEquity(current) // Fallback to Book Value
	}

	analysis.Level3.CurrentRatio = safeDiv(ca, cl)
	analysis.Level3.QuickRatio = safeDiv(ca-inv, cl)
	analysis.Level3.DebtToEquity = safeDiv(debt, getTotalEquity(current))
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
		totalDebt := getLongTermDebt(d) + getShortTermDebt(d)
		c := getCash(d)
		return totalDebt - c
	}

	calcEquity := func(d *edgar.FSAPDataResponse) float64 {
		return getTotalEquity(d)
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

	netInterest := getInterestExpense(current)
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

// =================================================================================================
// Safe Accessors - Ensure no panics on nil pointers
// =================================================================================================

// Helper to get float from FSAPValue pointer safely
func getVal(v *edgar.FSAPValue) float64 {
	if v == nil || v.Value == nil {
		return 0
	}
	return *v.Value
}

// Helper for float pointer safely
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

// --- Specific Field Accessors ---

func getRevenue(d *edgar.FSAPDataResponse) float64 {
	if d == nil || d.IncomeStatement.GrossProfitSection == nil {
		return 0
	}
	return getVal(d.IncomeStatement.GrossProfitSection.Revenues)
}

func getGrossProfit(d *edgar.FSAPDataResponse) float64 {
	if d == nil || d.IncomeStatement.GrossProfitSection == nil {
		return 0
	}
	return getVal(d.IncomeStatement.GrossProfitSection.GrossProfit)
}

func getOpIncome(d *edgar.FSAPDataResponse) float64 {
	if d == nil || d.IncomeStatement.OperatingCostSection == nil {
		return 0
	}
	return getVal(d.IncomeStatement.OperatingCostSection.OperatingIncome)
}

func getNetIncome(d *edgar.FSAPDataResponse) float64 {
	if d == nil || d.IncomeStatement.NetIncomeSection == nil {
		return 0
	}
	return getVal(d.IncomeStatement.NetIncomeSection.NetIncomeToCommon)
}

func getEPS(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.SupplementalData.EPSDiluted)
}

func getNetCashOperating(d *edgar.FSAPDataResponse) float64 {
	if d == nil || d.CashFlowStatement.CashSummary == nil {
		return 0
	}
	return getVal(d.CashFlowStatement.CashSummary.NetCashOperating)
}

func getCapex(d *edgar.FSAPDataResponse) float64 {
	if d == nil || d.CashFlowStatement.InvestingActivities == nil {
		return 0
	}
	return getVal(d.CashFlowStatement.InvestingActivities.Capex)
}

func getTotalAssets(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.BalanceSheet.ReportedForValidation.TotalAssets)
}

func getTotalEquity(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.BalanceSheet.ReportedForValidation.TotalEquity)
}

func getTotalLiabilities(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.BalanceSheet.ReportedForValidation.TotalLiabilities)
}

func getIncomeTaxExpense(d *edgar.FSAPDataResponse) float64 {
	if d == nil || d.IncomeStatement.TaxAdjustments == nil {
		return 0
	}
	return getVal(d.IncomeStatement.TaxAdjustments.IncomeTaxExpense)
}

func getIncomeBeforeTax(d *edgar.FSAPDataResponse) float64 {
	if d == nil || d.IncomeStatement.NonOperatingSection == nil {
		return 0
	}
	return getVal(d.IncomeStatement.NonOperatingSection.IncomeBeforeTax)
}

func getLongTermDebt(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.BalanceSheet.NoncurrentLiabilities.LongTermDebt)
}

func getShortTermDebt(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.BalanceSheet.CurrentLiabilities.NotesPayableShortTermDebt)
}

func getCash(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.BalanceSheet.CurrentAssets.CashAndEquivalents)
}

func getTotalCurrentAssets(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.BalanceSheet.ReportedForValidation.TotalCurrentAssets)
}

func getTotalCurrentLiabilities(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.BalanceSheet.ReportedForValidation.TotalCurrentLiabilities)
}

func getInventory(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.BalanceSheet.CurrentAssets.Inventories)
}

func getInterestExpense(d *edgar.FSAPDataResponse) float64 {
	if d == nil || d.IncomeStatement.NonOperatingSection == nil {
		return 0
	}
	return getVal(d.IncomeStatement.NonOperatingSection.InterestExpense)
}

func getRetainedEarnings(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.BalanceSheet.Equity.RetainedEarningsDeficit)
}

func getSharesOutstanding(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.SupplementalData.SharesOutstandingDiluted)
}

func getSharePrice(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getPtrVal(d.SupplementalData.SharePriceYearEnd)
}

// --- Additional Safe Accessors for Beneish M-Score ---

func getReceivables(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	// CurrentAssets is a struct, checks are safe structurally
	return getVal(d.BalanceSheet.CurrentAssets.AccountsReceivableNet)
}

func getPPE(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	return getVal(d.BalanceSheet.NoncurrentAssets.PPENet)
}

func getDepreciation(d *edgar.FSAPDataResponse) float64 {
	if d == nil {
		return 0
	}
	val := getVal(d.SupplementalData.DepreciationExpense)
	if val == 0 && d.CashFlowStatement.OperatingActivities != nil {
		val = getVal(d.CashFlowStatement.OperatingActivities.DepreciationAmortization)
	}
	return val
}

func getSGA(d *edgar.FSAPDataResponse) float64 {
	if d == nil || d.IncomeStatement.OperatingCostSection == nil {
		return 0
	}
	return getVal(d.IncomeStatement.OperatingCostSection.SGAExpenses)
}
