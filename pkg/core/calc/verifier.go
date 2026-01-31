package calc

import (
	"fmt"
	"math"
)

// FinancialStatement represents a simplified snapshot for validation
type FinancialStatement struct {
	TotalAssets      float64
	TotalLiabilities float64
	TotalEquity      float64
	NetIncome        float64
	OperatingCF      float64
	InvestingCF      float64
	FinancingCF      float64
	NetChangeInCash  float64
}

// VerificationResult holds the status of integrity checks
type VerificationResult struct {
	IsBalanced bool
	BalanceGap float64
	Warnings   []string
}

// CheckBalanceSheet verifies Asset = L + E
func CheckBalanceSheet(fs FinancialStatement) VerificationResult {
	gap := fs.TotalAssets - (fs.TotalLiabilities + fs.TotalEquity)
	isBalanced := math.Abs(gap) < 0.01

	var warnings []string
	if !isBalanced {
		warnings = append(warnings, fmt.Sprintf("Balance Sheet out of balance by %.2f", gap))
	}

	return VerificationResult{
		IsBalanced: isBalanced,
		BalanceGap: gap,
		Warnings:   warnings,
	}
}

// CheckCashFlow verifying Net Income roll-over to Cash
func CheckCashFlow(fs FinancialStatement) VerificationResult {
	calcChange := fs.OperatingCF + fs.InvestingCF + fs.FinancingCF
	gap := fs.NetChangeInCash - calcChange
	isBalanced := math.Abs(gap) < 0.01

	var warnings []string
	if !isBalanced {
		warnings = append(warnings, fmt.Sprintf("Cash Flow statement inconsistency by %.2f", gap))
	}

	return VerificationResult{
		IsBalanced: isBalanced,
		BalanceGap: gap,
		Warnings:   warnings,
	}
}
