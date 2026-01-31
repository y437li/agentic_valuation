package valuation

import (
	"agentic_valuation/pkg/core/projection"
)

// MasterValuationInput aggregates all inputs needed for the full suite of models
type MasterValuationInput struct {
	Projections       []*projection.ProjectedFinancials
	CurrentBookValue  float64 // B_0 (Equity)
	SharesOutstanding float64
	NetDebt           float64

	// Rates
	WACC           float64
	PeriodWACCs    []float64 // Added for dynamic WACC
	CostOfEquity   float64
	TerminalGrowth float64
	TaxRate        float64
}

// ValuationLineItem represents one row in the summary table (like the user's image)
type ValuationLineItem struct {
	ModelName  string
	SharePrice float64
}

// RunAllExecution performs DDM, RI, FCFE, and FCFF
func RunAllValuations(input MasterValuationInput) []ValuationLineItem {
	results := []ValuationLineItem{}

	// 1. Prepare Equity Inputs
	eqInput := EquityModelInput{
		Projections:       input.Projections,
		CostOfEquity:      input.CostOfEquity,
		TerminalGrowth:    input.TerminalGrowth,
		SharesOutstanding: input.SharesOutstanding,
		CurrentBookValue:  input.CurrentBookValue,
	}

	// 2. Prepare FCFF Input (DCF)
	dcfInput := DCFInput{
		Projections:       input.Projections,
		WACC:              input.WACC,
		PeriodWACCs:       input.PeriodWACCs,
		TerminalGrowth:    input.TerminalGrowth,
		SharesOutstanding: input.SharesOutstanding,
		NetDebt:           input.NetDebt,
		TaxRate:           input.TaxRate,
	}

	// --- Execute Models ---

	// 1. Dividend Based Valuation
	ddmRes := CalculateDDM(eqInput)
	results = append(results, ValuationLineItem{
		ModelName:  "Dividend Based Valuation",
		SharePrice: ddmRes.SharePrice,
	})

	// 2. Free Cash Flow Valuation (FCFE)
	// Note: User image lists "Free Cash Flow Valuation" separate from "All Debt and Equity"
	fcfeRes := CalculateFCFE(eqInput)
	results = append(results, ValuationLineItem{
		ModelName:  "Free Cash Flow Valuation (FCFE)",
		SharePrice: fcfeRes.SharePrice,
	})

	// 3. Residual Income Valuation
	riRes := CalculateResidualIncome(eqInput)
	results = append(results, ValuationLineItem{
		ModelName:  "Residual Income Valuation",
		SharePrice: riRes.SharePrice,
	})

	// 4. Residual Income Market-to-Book Valuation
	// (Using same logic as RI for now, per standard equivalence)
	riMtbRes := CalculateMarketToBookRI(eqInput)
	results = append(results, ValuationLineItem{
		ModelName:  "Residual Income Market-to-Book Valuation",
		SharePrice: riMtbRes.SharePrice,
	})

	// 5. Free Cash Flow for All Debt and Equity Valuation (FCFF)
	dcfRes := CalculateDCF(dcfInput)
	results = append(results, ValuationLineItem{
		ModelName:  "Free Cash Flow for All Debt and Equity Valuation",
		SharePrice: dcfRes.SharePrice,
	})

	return results
}
