package valuation

import (
	"agentic_valuation/pkg/core/projection"
)

// GenerateDynamicWACCSeries calculates WACC for each projection year based on projected capital structure
func GenerateDynamicWACCSeries(
	baseInput WACCInput,
	projections []*projection.ProjectedFinancials,
) []float64 {
	waccs := make([]float64, len(projections))

	for i, proj := range projections {
		// 1. Get Debt
		debt := 0.0
		if proj.BalanceSheet != nil {
			ltd := getValSafe(proj.BalanceSheet.NoncurrentLiabilities.LongTermDebt)
			std := getValSafe(proj.BalanceSheet.CurrentLiabilities.NotesPayableShortTermDebt)
			debt = ltd + std
		}

		// 2. Get Equity (Book Value)
		// Note: Ideally use Market Value of Equity, but that requires iteration.
		// Using Book Value as proxy for leverage in projection is a common simplification
		// if Market Value is not converged.
		equity := 0.0
		if proj.BalanceSheet != nil {
			equity = getValSafe(proj.BalanceSheet.Equity.CommonStockAPIC) +
				getValSafe(proj.BalanceSheet.Equity.RetainedEarningsDeficit)
		}

		// 3. Calculate Leverage
		// If equity is negative or zero, fallback to TargetDebtEquity?
		currentDE := baseInput.DebtToEquityRatio // Default to target
		if equity > 0 {
			currentDE = debt / equity
		}

		// 4. Calculate WACC for this year
		yearInput := baseInput
		yearInput.DebtToEquityRatio = currentDE

		res := CalculateWACC(yearInput)
		waccs[i] = res.WACC
	}

	return waccs
}
