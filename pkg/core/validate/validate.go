// Package validate provides reusable financial validation utilities.
// These functions can be called from tests, API handlers, or agent code
// to verify data integrity and calculate derived metrics.
package validate

import (
	"fmt"
	"math"
)

// =============================================================================
// YEAR-OVER-YEAR (YoY) CALCULATIONS
// =============================================================================

// YoYResult holds the result of a YoY calculation.
type YoYResult struct {
	CurrentYear  int
	PriorYear    int
	CurrentValue float64
	PriorValue   float64
	ChangeAbs    float64 // Absolute change
	ChangePct    float64 // Percentage change
	Label        string  // e.g., "Revenue", "Net Income"
}

// CalculateYoY calculates year-over-year change between two values.
// Returns percentage change: (current - prior) / prior * 100
func CalculateYoY(current, prior float64) float64 {
	if prior == 0 {
		if current == 0 {
			return 0
		}
		return math.Inf(1) // Infinite growth from zero
	}
	return (current - prior) / prior * 100
}

// YoYFromMap calculates YoY change from a year->value map.
func YoYFromMap(years map[int]float64, currentYear, priorYear int, label string) (*YoYResult, error) {
	current, okCur := years[currentYear]
	prior, okPri := years[priorYear]

	if !okCur {
		return nil, fmt.Errorf("missing data for year %d", currentYear)
	}
	if !okPri {
		return nil, fmt.Errorf("missing data for year %d", priorYear)
	}

	return &YoYResult{
		CurrentYear:  currentYear,
		PriorYear:    priorYear,
		CurrentValue: current,
		PriorValue:   prior,
		ChangeAbs:    current - prior,
		ChangePct:    CalculateYoY(current, prior),
		Label:        label,
	}, nil
}

// YoYFromStringMap is like YoYFromMap but accepts string year keys.
func YoYFromStringMap(years map[string]float64, currentYear, priorYear int, label string) (*YoYResult, error) {
	curStr := fmt.Sprintf("%d", currentYear)
	priStr := fmt.Sprintf("%d", priorYear)

	current, okCur := years[curStr]
	prior, okPri := years[priStr]

	if !okCur {
		return nil, fmt.Errorf("missing data for year %s", curStr)
	}
	if !okPri {
		return nil, fmt.Errorf("missing data for year %s", priStr)
	}

	return &YoYResult{
		CurrentYear:  currentYear,
		PriorYear:    priorYear,
		CurrentValue: current,
		PriorValue:   prior,
		ChangeAbs:    current - prior,
		ChangePct:    CalculateYoY(current, prior),
		Label:        label,
	}, nil
}

// =============================================================================
// CAGR (Compound Annual Growth Rate)
// =============================================================================

// CAGRResult holds the result of a CAGR calculation.
type CAGRResult struct {
	StartYear  int
	EndYear    int
	StartValue float64
	EndValue   float64
	Years      int
	CAGR       float64 // As percentage
}

// CalculateCAGR calculates compound annual growth rate.
// CAGR = ((EndValue / StartValue) ^ (1/years)) - 1
func CalculateCAGR(startValue, endValue float64, years int) float64 {
	if startValue <= 0 || years <= 0 {
		return 0
	}
	return (math.Pow(endValue/startValue, 1.0/float64(years)) - 1) * 100
}

// CAGRFromMap calculates CAGR from a year->value map.
func CAGRFromMap(years map[int]float64, startYear, endYear int, label string) (*CAGRResult, error) {
	start, okStart := years[startYear]
	end, okEnd := years[endYear]

	if !okStart {
		return nil, fmt.Errorf("missing start year %d", startYear)
	}
	if !okEnd {
		return nil, fmt.Errorf("missing end year %d", endYear)
	}

	numYears := endYear - startYear
	if numYears <= 0 {
		return nil, fmt.Errorf("end year must be after start year")
	}

	return &CAGRResult{
		StartYear:  startYear,
		EndYear:    endYear,
		StartValue: start,
		EndValue:   end,
		Years:      numYears,
		CAGR:       CalculateCAGR(start, end, numYears),
	}, nil
}

// =============================================================================
// FINANCIAL RATIO VALIDATION
// =============================================================================

// BalanceCheck verifies Assets = Liabilities + Equity.
type BalanceCheck struct {
	TotalAssets      float64
	TotalLiabilities float64
	TotalEquity      float64
	ComputedAssets   float64 // L + E
	Difference       float64
	IsBalanced       bool
	Tolerance        float64
}

// CheckBalanceEquation validates A = L + E within tolerance.
func CheckBalanceEquation(assets, liabilities, equity, tolerance float64) *BalanceCheck {
	computed := liabilities + equity
	diff := assets - computed

	return &BalanceCheck{
		TotalAssets:      assets,
		TotalLiabilities: liabilities,
		TotalEquity:      equity,
		ComputedAssets:   computed,
		Difference:       diff,
		IsBalanced:       math.Abs(diff) <= tolerance,
		Tolerance:        tolerance,
	}
}

// =============================================================================
// CASH FLOW VALIDATION
// =============================================================================

// CashFlowCheck verifies CFO + CFI + CFF = Net Change in Cash.
type CashFlowCheck struct {
	CFO           float64
	CFI           float64
	CFF           float64
	ComputedTotal float64
	ReportedTotal float64
	Difference    float64
	IsBalanced    bool
	Tolerance     float64
}

// CheckCashFlowEquation validates CFO + CFI + CFF = Net Change.
func CheckCashFlowEquation(cfo, cfi, cff, reportedNetChange, tolerance float64) *CashFlowCheck {
	computed := cfo + cfi + cff
	diff := reportedNetChange - computed

	return &CashFlowCheck{
		CFO:           cfo,
		CFI:           cfi,
		CFF:           cff,
		ComputedTotal: computed,
		ReportedTotal: reportedNetChange,
		Difference:    diff,
		IsBalanced:    math.Abs(diff) <= tolerance,
		Tolerance:     tolerance,
	}
}

// =============================================================================
// OUTLIER DETECTION
// =============================================================================

// OutlierCheck identifies suspicious values.
type OutlierCheck struct {
	Item       string
	Value      float64
	PriorValue float64
	ChangePct  float64
	IsOutlier  bool
	Reason     string
	Threshold  float64
}

// CheckForOutlier identifies if a value change is suspicious.
func CheckForOutlier(item string, current, prior, thresholdPct float64) *OutlierCheck {
	changePct := CalculateYoY(current, prior)

	check := &OutlierCheck{
		Item:       item,
		Value:      current,
		PriorValue: prior,
		ChangePct:  changePct,
		Threshold:  thresholdPct,
		IsOutlier:  false,
	}

	// Check for zero when prior was non-zero (likely extraction error)
	if current == 0 && prior > 0 {
		check.IsOutlier = true
		check.Reason = "Value dropped to zero (likely extraction error)"
		return check
	}

	// Check for extreme change
	if math.Abs(changePct) > thresholdPct {
		check.IsOutlier = true
		check.Reason = fmt.Sprintf("Change of %.1f%% exceeds threshold of %.1f%%", changePct, thresholdPct)
		return check
	}

	return check
}

// =============================================================================
// FREE CASH FLOW
// =============================================================================

// CalculateFCF computes Free Cash Flow = CFO + CapEx (CapEx is typically negative).
func CalculateFCF(cfo, capex float64) float64 {
	return cfo + capex
}

// CalculateFCFE computes Free Cash Flow to Equity.
// FCFE = FCF - Interest*(1-Tax) + Net Borrowing
func CalculateFCFE(fcf, interestExpense, taxRate, netBorrowing float64) float64 {
	afterTaxInterest := interestExpense * (1 - taxRate)
	return fcf - afterTaxInterest + netBorrowing
}
