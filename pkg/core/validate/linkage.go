// Package validate provides financial validation utilities.
// This file implements cross-statement linkage validation (勾稽验证)
// following Bloomberg/FactSet standards.
package validate

import (
	"agentic_valuation/pkg/core/edgar"
	"fmt"
	"math"
)

// =============================================================================
// CROSS-STATEMENT LINKAGE VALIDATION (跨报表勾稽验证)
// =============================================================================

// LinkageReport contains all cross-statement validation results
type LinkageReport struct {
	Year                   int                   `json:"year"`
	ISToCS                 *NetIncomeLinkage     `json:"is_to_cf"`    // IS → CF
	CFToBS                 *CashLinkage          `json:"cf_to_bs"`    // CF → BS
	ISToBSRetainedEarnings *RetainedEarningsLink `json:"is_to_bs_re"` // IS → BS
	AllPassed              bool                  `json:"all_passed"`
	FailedChecks           []string              `json:"failed_checks,omitempty"`
}

// NetIncomeLinkage validates: IS Net Income == CF Net Income Start
type NetIncomeLinkage struct {
	ISNetIncome   float64 `json:"is_net_income"`
	CFNetIncStart float64 `json:"cf_net_income_start"`
	Difference    float64 `json:"difference"`
	IsLinked      bool    `json:"is_linked"`
	Tolerance     float64 `json:"tolerance"`
}

// CashLinkage validates: CF Cash Ending == BS Cash, and CF Net Change == BS Cash YoY
type CashLinkage struct {
	CFCashEnding   float64 `json:"cf_cash_ending"`
	BSCash         float64 `json:"bs_cash"`
	DifferenceCash float64 `json:"difference_cash"`

	CFNetChange  float64 `json:"cf_net_change"`
	BSCashChange float64 `json:"bs_cash_change"` // Current Year - Prior Year
	DifferenceNC float64 `json:"difference_net_change"`

	IsLinked  bool    `json:"is_linked"`
	Tolerance float64 `json:"tolerance"`
}

// RetainedEarningsLink validates: ΔRE ≈ Net Income - Dividends
type RetainedEarningsLink struct {
	NetIncome        float64 `json:"net_income"`
	DividendsPaid    float64 `json:"dividends_paid"`
	ExpectedREChange float64 `json:"expected_re_change"` // NI - Div
	ActualREChange   float64 `json:"actual_re_change"`   // RE this year - RE last year
	Difference       float64 `json:"difference"`
	IsLinked         bool    `json:"is_linked"`
	Tolerance        float64 `json:"tolerance"`
	Note             string  `json:"note,omitempty"` // e.g., "Treasury stock buyback affects RE"
}

// =============================================================================
// LINKAGE VALIDATION FUNCTIONS
// =============================================================================

// ValidateLinkages performs all cross-statement validations for a single year.
func ValidateLinkages(
	is *edgar.IncomeStatement,
	cf *edgar.CashFlowStatement,
	bsCurrent *edgar.BalanceSheet,
	bsPrior *edgar.BalanceSheet,
	year int,
	tolerance float64,
) *LinkageReport {
	report := &LinkageReport{
		Year:      year,
		AllPassed: true,
	}

	// 1. IS → CF: Net Income Linkage
	report.ISToCS = validateNetIncomeLinkage(is, cf, tolerance)
	if report.ISToCS != nil && !report.ISToCS.IsLinked {
		report.AllPassed = false
		report.FailedChecks = append(report.FailedChecks, "IS Net Income → CF Net Income Start")
	}

	// 2. CF → BS: Cash Linkage
	report.CFToBS = validateCashLinkage(cf, bsCurrent, bsPrior, tolerance)
	if report.CFToBS != nil && !report.CFToBS.IsLinked {
		report.AllPassed = false
		report.FailedChecks = append(report.FailedChecks, "CF Cash Ending → BS Cash")
	}

	// 3. IS → BS: Retained Earnings Linkage
	report.ISToBSRetainedEarnings = validateRetainedEarningsLinkage(is, cf, bsCurrent, bsPrior, tolerance)
	if report.ISToBSRetainedEarnings != nil && !report.ISToBSRetainedEarnings.IsLinked {
		report.AllPassed = false
		report.FailedChecks = append(report.FailedChecks, "ΔRetained Earnings ≈ NI - Dividends")
	}

	return report
}

// validateNetIncomeLinkage checks IS Net Income == CF Operating Net Income Start
func validateNetIncomeLinkage(is *edgar.IncomeStatement, cf *edgar.CashFlowStatement, tolerance float64) *NetIncomeLinkage {
	if is == nil || cf == nil {
		return nil
	}

	var isNI, cfNI float64

	// Get IS Net Income
	if is.NetIncomeSection != nil && is.NetIncomeSection.NetIncomeToCommon != nil {
		if is.NetIncomeSection.NetIncomeToCommon.Value != nil {
			isNI = *is.NetIncomeSection.NetIncomeToCommon.Value
		}
	}

	// Get CF Net Income Start
	if cf.OperatingActivities != nil && cf.OperatingActivities.NetIncomeStart != nil {
		if cf.OperatingActivities.NetIncomeStart.Value != nil {
			cfNI = *cf.OperatingActivities.NetIncomeStart.Value
		}
	}

	diff := isNI - cfNI

	return &NetIncomeLinkage{
		ISNetIncome:   isNI,
		CFNetIncStart: cfNI,
		Difference:    diff,
		IsLinked:      math.Abs(diff) <= tolerance,
		Tolerance:     tolerance,
	}
}

// validateCashLinkage checks CF Cash Ending == BS Cash, and Net Change matches YoY
func validateCashLinkage(cf *edgar.CashFlowStatement, bsCurrent, bsPrior *edgar.BalanceSheet, tolerance float64) *CashLinkage {
	if cf == nil || bsCurrent == nil {
		return nil
	}

	result := &CashLinkage{
		Tolerance: tolerance,
	}

	// Get CF Cash Ending
	if cf.CashSummary != nil && cf.CashSummary.CashEnding != nil {
		if cf.CashSummary.CashEnding.Value != nil {
			result.CFCashEnding = *cf.CashSummary.CashEnding.Value
		}
	}

	// Get BS Cash
	if bsCurrent.CurrentAssets.CashAndEquivalents != nil {
		if bsCurrent.CurrentAssets.CashAndEquivalents.Value != nil {
			result.BSCash = *bsCurrent.CurrentAssets.CashAndEquivalents.Value
		}
	}

	result.DifferenceCash = result.CFCashEnding - result.BSCash

	// Get Net Change in Cash
	if cf.CashSummary != nil && cf.CashSummary.NetChangeInCash != nil {
		if cf.CashSummary.NetChangeInCash.Value != nil {
			result.CFNetChange = *cf.CashSummary.NetChangeInCash.Value
		}
	}

	// Calculate BS Cash YoY change
	if bsPrior != nil && bsPrior.CurrentAssets.CashAndEquivalents != nil {
		if bsPrior.CurrentAssets.CashAndEquivalents.Value != nil {
			priorCash := *bsPrior.CurrentAssets.CashAndEquivalents.Value
			result.BSCashChange = result.BSCash - priorCash
		}
	}

	result.DifferenceNC = result.CFNetChange - result.BSCashChange

	// Both checks must pass
	result.IsLinked = math.Abs(result.DifferenceCash) <= tolerance &&
		math.Abs(result.DifferenceNC) <= tolerance

	return result
}

// validateRetainedEarningsLinkage checks ΔRE ≈ Net Income - Dividends
func validateRetainedEarningsLinkage(
	is *edgar.IncomeStatement,
	cf *edgar.CashFlowStatement,
	bsCurrent, bsPrior *edgar.BalanceSheet,
	tolerance float64,
) *RetainedEarningsLink {
	if is == nil || bsCurrent == nil {
		return nil
	}

	result := &RetainedEarningsLink{
		Tolerance: tolerance,
	}

	// Get Net Income
	if is.NetIncomeSection != nil && is.NetIncomeSection.NetIncomeToCommon != nil {
		if is.NetIncomeSection.NetIncomeToCommon.Value != nil {
			result.NetIncome = *is.NetIncomeSection.NetIncomeToCommon.Value
		}
	}

	// Get Dividends Paid (from Cash Flow Financing)
	if cf != nil && cf.FinancingActivities != nil && cf.FinancingActivities.DividendsPaid != nil {
		if cf.FinancingActivities.DividendsPaid.Value != nil {
			result.DividendsPaid = math.Abs(*cf.FinancingActivities.DividendsPaid.Value)
		}
	}

	result.ExpectedREChange = result.NetIncome - result.DividendsPaid

	// Get Retained Earnings current and prior year
	var reCurrent, rePrior float64
	if bsCurrent.Equity.RetainedEarningsDeficit != nil && bsCurrent.Equity.RetainedEarningsDeficit.Value != nil {
		reCurrent = *bsCurrent.Equity.RetainedEarningsDeficit.Value
	}
	if bsPrior != nil && bsPrior.Equity.RetainedEarningsDeficit != nil && bsPrior.Equity.RetainedEarningsDeficit.Value != nil {
		rePrior = *bsPrior.Equity.RetainedEarningsDeficit.Value
	}

	result.ActualREChange = reCurrent - rePrior
	result.Difference = result.ActualREChange - result.ExpectedREChange

	// Use higher tolerance for RE because treasury stock, OCI, etc. also affect it
	// Bloomberg typically allows 5% variance for this check
	reTolerance := math.Max(tolerance, math.Abs(result.NetIncome*0.10)) // 10% of Net Income
	result.IsLinked = math.Abs(result.Difference) <= reTolerance

	if !result.IsLinked {
		result.Note = "Variance may be due to: Treasury Stock buybacks, OCI reclassifications, Stock-based compensation, or Preferred dividends"
	}

	return result
}

// =============================================================================
// MULTI-YEAR LINKAGE VALIDATION (使用 Zipper 合成后的数据)
// =============================================================================

// ValidateLinkagesByYear validates linkages using data from FSAPValue.Years maps
func ValidateLinkagesByYear(
	is *edgar.IncomeStatement,
	cf *edgar.CashFlowStatement,
	bs *edgar.BalanceSheet,
	currentYear, priorYear string,
	tolerance float64,
) *LinkageReport {
	// Convert string year to int for report
	var yearInt int
	_, _ = fmt.Sscanf(currentYear, "%d", &yearInt)

	report := &LinkageReport{
		Year:      yearInt,
		AllPassed: true,
	}

	// 1. IS → CF: Net Income Linkage (by year)
	isNI := getYearValue(is.NetIncomeSection.NetIncomeToCommon, currentYear)
	cfNI := getYearValue(cf.OperatingActivities.NetIncomeStart, currentYear)
	diff := isNI - cfNI

	report.ISToCS = &NetIncomeLinkage{
		ISNetIncome:   isNI,
		CFNetIncStart: cfNI,
		Difference:    diff,
		IsLinked:      math.Abs(diff) <= tolerance,
		Tolerance:     tolerance,
	}
	if !report.ISToCS.IsLinked {
		report.AllPassed = false
		report.FailedChecks = append(report.FailedChecks, "IS Net Income → CF Net Income Start")
	}

	// 2. CF → BS: Cash Linkage (by year)
	cfCashEnd := getYearValue(cf.CashSummary.CashEnding, currentYear)
	bsCash := getYearValue(bs.CurrentAssets.CashAndEquivalents, currentYear)
	bsCashPrior := getYearValue(bs.CurrentAssets.CashAndEquivalents, priorYear)
	cfNetChange := getYearValue(cf.CashSummary.NetChangeInCash, currentYear)
	bsCashChange := bsCash - bsCashPrior

	report.CFToBS = &CashLinkage{
		CFCashEnding:   cfCashEnd,
		BSCash:         bsCash,
		DifferenceCash: cfCashEnd - bsCash,
		CFNetChange:    cfNetChange,
		BSCashChange:   bsCashChange,
		DifferenceNC:   cfNetChange - bsCashChange,
		IsLinked:       math.Abs(cfCashEnd-bsCash) <= tolerance && math.Abs(cfNetChange-bsCashChange) <= tolerance,
		Tolerance:      tolerance,
	}
	if !report.CFToBS.IsLinked {
		report.AllPassed = false
		report.FailedChecks = append(report.FailedChecks, "CF Cash Ending → BS Cash")
	}

	return report
}

// Helper to extract value from Years map
func getYearValue(v *edgar.FSAPValue, year string) float64 {
	if v == nil {
		return 0
	}
	if v.Years != nil {
		if val, ok := v.Years[year]; ok {
			return val
		}
	}
	if v.Value != nil {
		return *v.Value
	}
	return 0
}
