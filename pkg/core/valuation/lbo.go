package valuation

import (
	"math"
)

// LBOInput parameters for Ability-To-Pay analysis
type LBOInput struct {
	TargetEBITDA       float64
	LeverageRatio      float64 // Debt / EBITDA (e.g. 5.0x)
	InterestRate       float64 // Cost of Debt
	TaxRate            float64
	EntryMultiple      float64 // If 0, we solve for it based on TargetIRR? No, LBO usually outputs IRR or Price.
	ExitMultiple       float64
	HoldingPeriod      int       // Years (e.g. 5)
	ProjectedEBITDA    []float64 // Stream of EBITDA for 5 years
	ProjectedCapex     []float64
	ProjectedChangeNWC []float64
	TargetIRR          float64 // e.g. 0.20
}

// LBOResult
type LBOResult struct {
	MaxEntryEV           float64
	ImpliedEntryMultiple float64
	EquityCheck          float64
	DebtRaised           float64
	ExitEquityValue      float64
	ArchiveIRR           float64 // If entry multiple is fixed
}

// CalculateLBO determines the specific price a sponsor can pay to achieve TargetIRR
func CalculateLBO(input LBOInput) LBOResult {
	// 1. Calculate Debt Calculation
	initialDebt := input.TargetEBITDA * input.LeverageRatio

	// 2. Build Cash Flow Waterfall to find Exit Equity
	// Assume clean waterfall: FCF pays down debt.
	currentDebt := initialDebt

	for i := 0; i < input.HoldingPeriod; i++ {
		ebitda := input.ProjectedEBITDA[i]
		capex := input.ProjectedCapex[i]
		nwc := input.ProjectedChangeNWC[i]

		// Simplified Tax: (EBITDA - D&A - Interest) * t
		// We approx Interest = currentDebt * Rate
		interest := currentDebt * input.InterestRate

		// Pre-tax Cash Flow (Proxy)
		// Usually: EBITDA - Interest - Taxes - CapEx - NWC
		// Simplified Tax Base: EBITDA - Interest (Ignoring D&A tax shield for simplicity or assume D&A ~ Capex if not provided)
		// Better: Input provides FCF directly? No, we have projected components.
		// Let's assume D&A is roughly equal to CapEx for tax shield if not provided.

		taxableIncome := ebitda - interest // - D&A
		taxes := taxableIncome * input.TaxRate
		if taxes < 0 {
			taxes = 0
		}

		fcf := ebitda - interest - taxes - capex - nwc

		// Debt Paydown (Sweep)
		if fcf > 0 {
			currentDebt -= fcf
			if currentDebt < 0 {
				currentDebt = 0
			}
		} else {
			// Deficit funded by revolver?
			currentDebt += (-fcf)
		}
	}

	// 3. Exit
	finalEBITDA := input.ProjectedEBITDA[input.HoldingPeriod-1]
	exitEV := finalEBITDA * input.ExitMultiple
	exitEquity := exitEV - currentDebt

	// 4. Solve for Entry Equity (Backward Induction)
	// targetIRR = (ExitEquity / EntryEquity)^(1/T) - 1
	// (1+IRR)^T = Exit / Entry
	// Entry = Exit / (1+IRR)^T

	requiredEquity := exitEquity / math.Pow(1.0+input.TargetIRR, float64(input.HoldingPeriod))

	// 5. Total EV
	maxEntryEV := requiredEquity + initialDebt

	return LBOResult{
		MaxEntryEV:           maxEntryEV,
		ImpliedEntryMultiple: maxEntryEV / input.TargetEBITDA,
		EquityCheck:          requiredEquity,
		DebtRaised:           initialDebt,
		ExitEquityValue:      exitEquity,
	}
}
