package valuation

// WACCInput parameters for calculating Cost of Capital
type WACCInput struct {
	UnleveredBeta     float64
	RiskFreeRate      float64
	MarketRiskPremium float64
	PreTaxCostOfDebt  float64
	TaxRate           float64
	DebtToEquityRatio float64 // Target Leverage (D/E)
}

// WACCResult holds the calculated rates
type WACCResult struct {
	LeveredBeta  float64
	CostOfEquity float64
	CostOfDebt   float64 // After-tax
	WACC         float64
	WeightDebt   float64
	WeightEquity float64
}

// CalculateWACC computes the Weighted Average Cost of Capital using CAPM and Hamada Equation
func CalculateWACC(input WACCInput) WACCResult {
	// 1. Re-lever Beta (Hamada)
	// BetaL = BetaU * (1 + (1-t)*(D/E))
	leveredBeta := input.UnleveredBeta * (1 + (1-input.TaxRate)*input.DebtToEquityRatio)

	// 2. Cost of Equity (CAPM)
	// Ke = Rf + BetaL * ERP
	ke := input.RiskFreeRate + leveredBeta*input.MarketRiskPremium

	// 3. Cost of Debt (After-tax)
	// Kd = PreTaxKd * (1 - t)
	kd := input.PreTaxCostOfDebt * (1 - input.TaxRate)

	// 4. Weights
	// D/E = x -> D = xE
	// V = D + E = xE + E = E(1+x)
	// Wd = D/V = xE / E(1+x) = x / (1+x)
	// We = E/V = E / E(1+x) = 1 / (1+x)
	wd := input.DebtToEquityRatio / (1 + input.DebtToEquityRatio)
	we := 1.0 / (1 + input.DebtToEquityRatio)

	// 5. WACC
	wacc := (ke * we) + (kd * wd)

	return WACCResult{
		LeveredBeta:  leveredBeta,
		CostOfEquity: ke,
		CostOfDebt:   kd,
		WACC:         wacc,
		WeightDebt:   wd,
		WeightEquity: we,
	}
}
