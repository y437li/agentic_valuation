package calc

import (
	"agentic_valuation/pkg/core/edgar"
	"math"
)

// =============================================================================
// FINANCIAL ANALYSIS ENGINE
// =============================================================================

// AnalyzeFinancials generates comprehensive analysis (Common Size + Growth)
func AnalyzeFinancials(current *edgar.FSAPDataResponse, history []*edgar.FSAPDataResponse) *CommonSizeAnalysis {
	if current == nil {
		return &CommonSizeAnalysis{
			IncomeStatement: make(map[string]*AnalysisResult),
			BalanceSheet:    make(map[string]*AnalysisResult),
			CashFlow:        make(map[string]*AnalysisResult),
		}
	}

	analysis := &CommonSizeAnalysis{
		IncomeStatement: make(map[string]*AnalysisResult),
		BalanceSheet:    make(map[string]*AnalysisResult),
		CashFlow:        make(map[string]*AnalysisResult),
	}

	// 1. Common Size Analysis (Vertical Analysis)
	// Base: Total Revenue for IS, Total Assets for BS
	var revenue, totalAssets float64
	if current.IncomeStatement.GrossProfitSection != nil && current.IncomeStatement.GrossProfitSection.Revenues != nil {
		revenue = getVal(current.IncomeStatement.GrossProfitSection.Revenues)
	}
	if current.BalanceSheet.ReportedForValidation.TotalAssets != nil {
		totalAssets = getVal(current.BalanceSheet.ReportedForValidation.TotalAssets)
	}

	// Helper to add IS analysis
	addIS := func(key string, val *edgar.FSAPValue) {
		if val == nil {
			return
		}
		v := getVal(val)
		pct := safeDiv(v, revenue)
		analysis.IncomeStatement[key] = &AnalysisResult{Value: pct}
	}

	// Helper to add BS analysis
	addBS := func(key string, val *edgar.FSAPValue) {
		if val == nil {
			return
		}
		v := getVal(val)
		pct := safeDiv(v, totalAssets)
		analysis.BalanceSheet[key] = &AnalysisResult{Value: pct}
	}

	// Income Statement Common Size
	is := current.IncomeStatement
	if is.GrossProfitSection != nil {
		addIS("cost_of_goods_sold", is.GrossProfitSection.CostOfGoodsSold)
		addIS("gross_profit", is.GrossProfitSection.GrossProfit)
	}
	if is.OperatingCostSection != nil {
		addIS("sga_expenses", is.OperatingCostSection.SGAExpenses)
		addIS("rd_expenses", is.OperatingCostSection.RDExpenses)
		addIS("operating_income", is.OperatingCostSection.OperatingIncome)
	}
	if is.NetIncomeSection != nil {
		addIS("net_income", is.NetIncomeSection.NetIncomeToCommon)
	}

	// Balance Sheet Common Size
	bs := current.BalanceSheet
	addBS("cash_and_equivalents", bs.CurrentAssets.CashAndEquivalents)
	addBS("accounts_receivable", bs.CurrentAssets.AccountsReceivableNet)
	addBS("inventory", bs.CurrentAssets.Inventories)
	addBS("ppe_net", bs.NoncurrentAssets.PPENet)
	addBS("goodwill", bs.NoncurrentAssets.Goodwill)
	addBS("accounts_payable", bs.CurrentLiabilities.AccountsPayable)
	addBS("long_term_debt", bs.NoncurrentLiabilities.LongTermDebt)
	addBS("total_equity", bs.ReportedForValidation.TotalEquity)

	// 2. Growth Analysis (Horizontal Analysis)
	// Compare current vs immediate prior year
	// NOTE: Ideally we iterate through full history, but for this struct we just want "Current Growth"
	// or we expand CommonSizeAnalysis to hold a map of growth rates.
	// For now, let's look for the prior year in 'history' list.
	var prior *edgar.FSAPDataResponse
	targetPriorYear := current.FiscalYear - 1
	for _, h := range history {
		if h.FiscalYear == targetPriorYear {
			prior = h
			break
		}
	}

	if prior != nil {
		// IS Growth
		calcGrowth := func(currVal, priorVal *edgar.FSAPValue) float64 {
			c := getVal(currVal)
			p := getVal(priorVal)
			return GrowthRate(c, p)
		}

		if is.GrossProfitSection != nil && prior.IncomeStatement.GrossProfitSection != nil {
			g := calcGrowth(is.GrossProfitSection.Revenues, prior.IncomeStatement.GrossProfitSection.Revenues)
			analysis.IncomeStatement["revenue_growth"] = &AnalysisResult{Value: g}
		}
		if is.NetIncomeSection != nil && prior.IncomeStatement.NetIncomeSection != nil {
			g := calcGrowth(is.NetIncomeSection.NetIncomeToCommon, prior.IncomeStatement.NetIncomeSection.NetIncomeToCommon)
			analysis.IncomeStatement["net_income_growth"] = &AnalysisResult{Value: g}
		}
	}

	return analysis
}


// =============================================================================
// ADVANCED PROFITABILITY (PENMAN FRAMEWORK)
// Decomposing ROCE into Operating and Financing Components
// =============================================================================

// NetOperatingAssets (NOA) = Operating Assets - Operating Liabilities
// Typically: (Total Assets - Cash - ST Investments) - (Total Liab - Total Debt)
// But strictly: Operating Assets - Operating Liabs
func NetOperatingAssets(totalAssets, cash, stInvest, totalLiabs, totalDebt float64) float64 {
	operatingAssets := totalAssets - cash - stInvest
	operatingLiabs := totalLiabs - totalDebt
	return operatingAssets - operatingLiabs
}

// NetFinancialObligations (NFO) = Total Debt - (Cash + ST Investments)
// Also called Net Debt.
func NetFinancialObligations(totalDebt, cash, stInvest float64) float64 {
	return totalDebt - (cash + stInvest)
}

// RNOA (Return on Net Operating Assets) = NOPAT / Average NOA
func RNOA(nopat, avgNOA float64) float64 {
	return safeDiv(nopat, avgNOA)
}

// NBC (Net Borrowing Cost) = Net Financial Expense (after tax) / Average NFO
func NBC(netInterestAfterTax, avgNFO float64) float64 {
	return safeDiv(netInterestAfterTax, avgNFO)
}

// FLEV (Financial Leverage) = Average NFO / Average Common Equity
func FLEV(avgNFO, avgEquity float64) float64 {
	return safeDiv(avgNFO, avgEquity)
}

// PenmanROCE Decomposition
// ROCE = RNOA + (FLEV * Spread)
// Spread = RNOA - NBC
type PenmanResult struct {
	RNOA   float64
	NBC    float64
	FLEV   float64
	Spread float64
	ROCE   float64
}

func CalculatePenmanDecomposition(nopat, netInterestAT, avgNOA, avgNFO, avgEquity float64) PenmanResult {
	rnoa := RNOA(nopat, avgNOA)
	nbc := NBC(netInterestAT, avgNFO)
	flev := FLEV(avgNFO, avgEquity)
	spread := rnoa - nbc
	return PenmanResult{
		RNOA:   rnoa,
		NBC:    nbc,
		FLEV:   flev,
		Spread: spread,
		ROCE:   rnoa + (flev * spread),
	}
}

// =============================================================================
// RISK MODELS
// =============================================================================

// Altman Z-Score (Manufacturing)
// Z = 1.2A + 1.4B + 3.3C + 0.6D + 1.0E
// A = Working Capital / Total Assets
// B = Retained Earnings / Total Assets
// C = EBIT / Total Assets
// D = Market Value of Equity / Total Liabilities
// E = Sales / Total Assets
// Note: Requires Market Value of Equity (MVE) which is NOT in 10-K. Must be passed in.
func AltmanZScore(wc, re, ebit, mve, sales, ta, tl float64) float64 {
	return AltmanZScoreManufacturing(wc, re, ebit, mve, sales, ta, tl)
}

// Corrected Altman Z-Score
func AltmanZScoreManufacturing(wc, re, ebit, mve, sales, ta, tl float64) float64 {
	if ta == 0 || tl == 0 {
		return 0
	}
	A := wc / ta
	B := re / ta
	C := ebit / ta
	D := mve / tl
	E := sales / ta

	return 1.2*A + 1.4*B + 3.3*C + 0.6*D + 1.0*E
}

// Beneish M-Score Variables
type BeneishInput struct {
	DSRI float64 // Days Sales in Receivables Index
	GMI  float64 // Gross Margin Index
	AQI  float64 // Asset Quality Index
	SGI  float64 // Sales Growth Index
	DEPI float64 // Depreciation Index
	SGAI float64 // SGA Index
	LVGI float64 // Leverage Index
	TATA float64 // Total Accruals to Total Assets
}

// Calculate Beneish M-Score
// M = -4.84 + 0.92*DSRI + 0.528*GMI + 0.404*AQI + 0.892*SGI + 0.115*DEPI - 0.172*SGAI + 4.679*TATA - 0.327*LVGI
func BeneishMScore(i BeneishInput) float64 {
	return -4.84 +
		0.92*i.DSRI +
		0.528*i.GMI +
		0.404*i.AQI +
		0.892*i.SGI +
		0.115*i.DEPI -
		0.172*i.SGAI +
		4.679*i.TATA -
		0.327*i.LVGI
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func safeDiv(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

// =============================================================================
// LEGACY PROFITABILITY RATIOS (Basic DuPont)
// =============================================================================

func ProfitMarginForROA(netIncome, taxRate, interestExpense, revenue float64) float64 {
	if revenue == 0 {
		return 0
	}
	afterTaxInterest := (1 - taxRate) * math.Abs(interestExpense)
	return (netIncome + afterTaxInterest) / revenue
}

func AssetTurnover(revenue, avgTotalAssets float64) float64 {
	return safeDiv(revenue, avgTotalAssets)
}

func ROA(profitMargin, assetTurnover float64) float64 {
	return profitMargin * assetTurnover
}

// =============================================================================
// LEGACY LIQUIDITY & SOLVENCY
// =============================================================================

func CurrentRatio(currentAssets, currentLiabilities float64) float64 {
	return safeDiv(currentAssets, currentLiabilities)
}

func QuickRatio(cash, stInvestments, accountsReceivable, currentLiabilities float64) float64 {
	return safeDiv(cash+stInvestments+accountsReceivable, currentLiabilities)
}

func LTDebtToCapital(ltDebt, totalEquity float64) float64 {
	return safeDiv(ltDebt, ltDebt+totalEquity)
}

func InterestCoverageRatio(operatingIncome, interestExpense float64) float64 {
	return safeDiv(operatingIncome+math.Abs(interestExpense), math.Abs(interestExpense))
}

// =============================================================================
// LEGACY GROWTH METRICS
// =============================================================================

func GrowthRate(current, prior float64) float64 {
	if prior == 0 {
		return 0
	}
	return (current - prior) / math.Abs(prior)
}

func CAGR(endingValue, beginningValue float64, years int) float64 {
	if beginningValue == 0 || years == 0 {
		return 0
	}
	return math.Pow(endingValue/beginningValue, 1.0/float64(years)) - 1
}

// =============================================================================
// LEGACY DUPONT
// =============================================================================

type DuPontResult struct {
	ProfitMargin      float64
	AssetTurnover     float64
	FinancialLeverage float64
	ROE               float64
}

func DuPontROE(netIncome, revenue, avgAssets, avgEquity float64) DuPontResult {
	pm := safeDiv(netIncome, revenue)
	at := safeDiv(revenue, avgAssets)
	fl := safeDiv(avgAssets, avgEquity)
	return DuPontResult{
		ProfitMargin:      pm,
		AssetTurnover:     at,
		FinancialLeverage: fl,
		ROE:               pm * at * fl,
	}
}
