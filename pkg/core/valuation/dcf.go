package valuation

import (
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/projection"
)

// DCFInput encapsulates all inputs required for a Discounted Cash Flow valuation
type DCFInput struct {
	Projections       []*projection.ProjectedFinancials
	WACC              float64   // Deprecated: Use PeriodWACC or single value as fallback
	PeriodWACCs       []float64 // Optional: WACC per projection year
	TerminalGrowth    float64   // e.g. 0.025
	SharesOutstanding float64   // Millions
	NetDebt           float64   // Millions
	TaxRate           float64   // Used for adjustment
}

// DCFResult holds the valuation outputs
type DCFResult struct {
	EnterpriseValue float64
	EquityValue     float64
	SharePrice      float64
	PV_FCF          float64
	PV_Terminal     float64
	ImpliedMultiple float64 // EV / EBITDA (Terminal Year)
}

// CalculateDCF performs a standard 2-stage DCF analysis
func CalculateDCF(input DCFInput) DCFResult {
	var pvFCF float64
	var terminalEBITDA float64
	var terminalUFCF float64

	// Track cumulative discount factor for dynamic WACC
	cumDiscountFactor := 1.0

	// Helper for safe value extraction
	getVal := func(v *edgar.FSAPValue) float64 {
		if v != nil && v.Value != nil {
			return *v.Value
		}
		return 0.0
	}

	for i, proj := range input.Projections {
		// 1. Calculate UFCF
		// Start with Cash Flow From Operations (CFO)
		var cfo float64
		if proj.CashFlow != nil {
			if proj.CashFlow.CashSummary != nil {
				cfo = getVal(proj.CashFlow.CashSummary.NetCashOperating)
			}
			// If missing, try to sum OperatingActivities?
			// For Projections, CashSummary is always populated by Engine.
		}

		// Add back tax-shielded interest (to get to Unlevered)
		var interest float64
		if proj.IncomeStatement != nil {
			if proj.IncomeStatement.NonOperatingSection != nil {
				interest = getVal(proj.IncomeStatement.NonOperatingSection.InterestExpense)
			}
		}
		interestAdj := interest * (1 - input.TaxRate)

		// Subtract CapEx (add signed negative)
		var capex float64
		if proj.CashFlow != nil {
			if proj.CashFlow.InvestingActivities != nil {
				capex = getVal(proj.CashFlow.InvestingActivities.Capex)
			}
		}

		// UFCF = CFO + Interest(1-t) + CapEx(Negative)
		ufcf := cfo + interestAdj + capex

		// 2. Discount (Dynamic WACC)
		wacc := input.WACC
		if len(input.PeriodWACCs) > i {
			wacc = input.PeriodWACCs[i]
		}

		cumDiscountFactor /= (1.0 + wacc)
		pvFCF += ufcf * cumDiscountFactor

		// Store last year metrics for Terminal Value
		if i == len(input.Projections)-1 {
			// Terminal UFCF
			terminalUFCF = ufcf * (1 + input.TerminalGrowth)

			// EBITDA for implied multiple
			var opIncome, depn float64
			if proj.IncomeStatement != nil && proj.IncomeStatement.OperatingCostSection != nil {
				opIncome = getVal(proj.IncomeStatement.OperatingCostSection.OperatingIncome)
			}
			if proj.CashFlow != nil && proj.CashFlow.OperatingActivities != nil {
				depn = getVal(proj.CashFlow.OperatingActivities.DepreciationAmortization)
			}
			terminalEBITDA = opIncome + depn
		}
	}

	// 3. Terminal Value (Gordon Growth)
	// TV = (TerminalUFCF) / (WACC_final - g)
	// Use final year WACC for capitalization
	finalWACC := input.WACC
	if len(input.PeriodWACCs) > 0 {
		finalWACC = input.PeriodWACCs[len(input.PeriodWACCs)-1]
	}

	tv := 0.0
	if finalWACC > input.TerminalGrowth {
		tv = terminalUFCF / (finalWACC - input.TerminalGrowth)
	}

	// Discount TV
	pvTerminal := tv * cumDiscountFactor

	// 4. Aggregation
	ev := pvFCF + pvTerminal
	eqVal := ev - input.NetDebt
	sharePrice := 0.0
	if input.SharesOutstanding != 0 {
		sharePrice = eqVal / input.SharesOutstanding
	}

	// Implied Exit Multiple
	impliedMultiple := 0.0
	if terminalEBITDA != 0 {
		impliedMultiple = tv / terminalEBITDA
	}

	return DCFResult{
		EnterpriseValue: ev,
		EquityValue:     eqVal,
		SharePrice:      sharePrice,
		PV_FCF:          pvFCF,
		PV_Terminal:     pvTerminal,
		ImpliedMultiple: impliedMultiple,
	}
}
