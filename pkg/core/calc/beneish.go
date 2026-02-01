package calc

import (
	"agentic_valuation/pkg/core/edgar"
)

// BeneishMScoreResult holds the 8 variables and final score
type BeneishMScoreResult struct {
	DSRI  float64 `json:"dsri"`  // Days Sales in Receivables Index
	GMI   float64 `json:"gmi"`   // Gross Margin Index
	AQI   float64 `json:"aqi"`   // Asset Quality Index
	SGI   float64 `json:"sgi"`   // Sales Growth Index
	DEPI  float64 `json:"depi"`  // Depreciation Index
	SGAI  float64 `json:"sgai"`  // SG&A Expenses Index
	LVGI  float64 `json:"lvgi"`  // Leverage Index
	TATA  float64 `json:"tata"`  // Total Accruals to Total Assets
	Score float64 `json:"score"` // Probability of Manipulation ( > -1.78 is suspect)
	Risk  string  `json:"risk"`  // "High Probability" or "Low Probability"
}

// CalculateBeneishMScore computes the 8-variable M-Score.
// Requires Current and Prior year data.
// Formula: -4.84 + 0.92*DSRI + 0.528*GMI + 0.404*AQI + 0.892*SGI + 0.115*DEPI - 0.172*SGAI + 4.679*TATA - 0.327*LVGI
// Note: Coefficients vary slightly in literature; using original 1999 paper values commonly cited.
func CalculateBeneishMScore(current, prior *edgar.FSAPDataResponse) *BeneishMScoreResult {
	if current == nil || prior == nil {
		return nil
	}

	// 1. DSRI: (Net Receivables_t / Sales_t) / (Net Receivables_t-1 / Sales_t-1)
	recCurr := getReceivables(current)
	salesCurr := getRevenue(current)
	recPrior := getReceivables(prior)
	salesPrior := getRevenue(prior)

	dsri := safeDiv(safeDiv(recCurr, salesCurr), safeDiv(recPrior, salesPrior))

	// 2. GMI: [(Sales_t-1 - COGS_t-1) / Sales_t-1] / [(Sales_t - COGS_t) / Sales_t]
	// Actually GMI is (Gross Margin_t-1 / Gross Margin_t). If GM deteriorates, GMI > 1 (bad signal).
	// GM = (Sales - COGS) / Sales = GrossProfit / Sales
	gpCurr := getGrossProfit(current)
	gpPrior := getGrossProfit(prior)

	gmCurr := safeDiv(gpCurr, salesCurr)
	gmPrior := safeDiv(gpPrior, salesPrior)

	gmi := safeDiv(gmPrior, gmCurr)

	// 3. AQI: [1 - (Current Assets_t + PP&E_t + Securities_t) / Total Assets_t] / [1 - ((Current Assets_t-1 + PP&E_t-1 + Securities_t-1) / Total Assets_t-1)]
	// AQI measures the proportion of non-current assets other than PP&E.
	// CA + PPE = "Solid Assets".  Total - Solid = "Soft Assets" (Intangibles, Deferred charges, etc.)
	// High AQI > 1 indicates potentially excessive capitalization of costs.

	calcSoftAssetsRatio := func(d *edgar.FSAPDataResponse) float64 {
		ta := getTotalAssets(d)
		ca := getTotalCurrentAssets(d)
		ppe := getPPE(d)
		// Assuming Securities are in Current Assets or Investments?
		// Beneish defines "PP&E" strictly. Let's stick to CA + PPE as main realizable assets.
		if ta == 0 {
			return 0
		}
		return 1.0 - ((ca + ppe) / ta)
	}

	aqi := safeDiv(calcSoftAssetsRatio(current), calcSoftAssetsRatio(prior))

	// 4. SGI: Sales_t / Sales_t-1
	sgi := safeDiv(salesCurr, salesPrior)

	// 5. DEPI: (Depreciation_t-1 / (PPE_t-1 + Dep_t-1)) / (Depreciation_t / (PPE_t + Dep_t))
	// DEPI > 1 means depreciation rate slowed down (income increasing).
	// Dep Rate = Dep Exp / (Net PPE + Dep Exp) -> Dep Exp / Gross PPE
	calcDepRate := func(d *edgar.FSAPDataResponse) float64 {
		dep := getDepreciation(d)
		ppeNet := getPPE(d)
		return safeDiv(dep, ppeNet+dep)
	}

	depi := safeDiv(calcDepRate(prior), calcDepRate(current))

	// 6. SGAI: (SGA_t / Sales_t) / (SGA_t-1 / Sales_t-1)
	// SGAI > 1 means decreasing administrative efficiency (or reclassification).
	calcSGARatio := func(d *edgar.FSAPDataResponse) float64 {
		sga := getSGA(d)
		sal := getRevenue(d)
		return safeDiv(sga, sal)
	}

	sgai := safeDiv(calcSGARatio(current), calcSGARatio(prior))

	// 7. LVGI: [(LTD_t + CL_t) / TA_t] / [(LTD_t-1 + CL_t-1) / TA_t-1]
	// Liability / Assets leverage.
	calcLev := func(d *edgar.FSAPDataResponse) float64 {
		tl := getTotalLiabilities(d)
		ta := getTotalAssets(d)
		return safeDiv(tl, ta)
	}

	lvgi := safeDiv(calcLev(current), calcLev(prior))

	// 8. TATA: (Income from Contin Op - Cash from Ops) / Total Assets
	// Measures total accruals.
	income := getNetIncome(current) // Or IncomeBeforeExtraordinary
	cfo := getNetCashOperating(current)
	taCurr := getTotalAssets(current)

	tata := safeDiv(income-cfo, taCurr)

	// Formula (8 variable):
	// M = -4.84 + 0.92*DSRI + 0.528*GMI + 0.404*AQI + 0.892*SGI + 0.115*DEPI - 0.172*SGAI + 4.679*TATA - 0.327*LVGI

	score := -4.84 +
		0.920*dsri +
		0.528*gmi +
		0.404*aqi +
		0.892*sgi +
		0.115*depi -
		0.172*sgai +
		4.679*tata -
		0.327*lvgi

	risk := "Low Probability"
	if score > -1.78 {
		risk = "High Probability"
	}

	return &BeneishMScoreResult{
		DSRI: dsri, GMI: gmi, AQI: aqi, SGI: sgi,
		DEPI: depi, SGAI: sgai, LVGI: lvgi, TATA: tata,
		Score: score,
		Risk:  risk,
	}
}
