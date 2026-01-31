package edgar

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// =============================================================================
// AGGREGATION VALIDATION - Simple sum validation for extracted values
// =============================================================================

// getLatestYear finds the latest (most recent) year from all values
func getLatestYear(values []*FSAPValue) string {
	years := make(map[string]bool)
	for _, v := range values {
		if v != nil && v.Years != nil {
			for y := range v.Years {
				years[y] = true
			}
		}
	}
	if len(years) == 0 {
		return ""
	}
	// Sort years and get the latest
	var yearList []string
	for y := range years {
		yearList = append(yearList, y)
	}
	sort.Strings(yearList)
	return yearList[len(yearList)-1] // Return max year
}

// getValueForYear gets value for a specific year, with fallback
func getValueForYear(v *FSAPValue, year string) float64 {
	if v == nil {
		return 0
	}
	if year != "" && v.Years != nil {
		if val, ok := v.Years[year]; ok {
			return val
		}
	}
	if v.Value != nil {
		return *v.Value
	}
	return 0
}

// ValidationResult holds the result of a balance sheet validation
type ValidationResult struct {
	IsValid         bool
	TotalAssets     float64
	TotalLiabEquity float64
	Difference      float64
	DiffPercent     float64
	Message         string
}

// ValidateBalanceSheet checks if Assets = Liabilities + Equity
// Looks for "total assets" and "total liabilities and equity" (or similar) labels
// If direct L+E match fails, falls back to summing Total Liabilities + Total Equity
func ValidateBalanceSheet(values []*FSAPValue, year string) *ValidationResult {
	// Auto-detect latest year if not specified
	if year == "" {
		year = getLatestYear(values)
	}

	var totalAssets, totalLiabEquity, totalLiabilities, totalEquity float64
	foundAssets, foundLiabEquity, foundLiabilities, foundEquity := false, false, false, false

	for _, v := range values {
		if v == nil {
			continue
		}
		labelLower := strings.ToLower(v.Label)
		val := getValueForYear(v, year)

		// Match Total Assets (not current assets, not part of L+E line)
		if strings.Contains(labelLower, "total assets") &&
			!strings.Contains(labelLower, "current") &&
			!strings.Contains(labelLower, "liabilities") {
			totalAssets = val
			foundAssets = true
		}

		// Match "Total liabilities and equity" or "Total liabilities and stockholders' equity"
		if strings.Contains(labelLower, "total liabilities") &&
			(strings.Contains(labelLower, "equity") || strings.Contains(labelLower, "stockholders")) {
			totalLiabEquity = val
			foundLiabEquity = true
		}

		// Match Total Liabilities (standalone, not part of L+E line)
		if strings.Contains(labelLower, "total liabilities") &&
			!strings.Contains(labelLower, "equity") &&
			!strings.Contains(labelLower, "stockholders") &&
			!strings.Contains(labelLower, "current") {
			totalLiabilities = val
			foundLiabilities = true
		}

		// Match Total Equity (standalone)
		if (strings.Contains(labelLower, "total") || strings.Contains(labelLower, "shareholders")) &&
			strings.Contains(labelLower, "equity") &&
			!strings.Contains(labelLower, "liabilities") {
			totalEquity = val
			foundEquity = true
		}
	}

	result := &ValidationResult{
		TotalAssets:     totalAssets,
		TotalLiabEquity: totalLiabEquity,
	}

	// Fallback: if L+E row value seems wrong, compute from components
	if foundAssets && foundLiabEquity {
		diff := math.Abs(totalAssets - totalLiabEquity)
		if diff > totalAssets*0.10 && foundLiabilities && foundEquity {
			// L+E row value is way off, use computed sum instead
			computed := totalLiabilities + totalEquity
			result.TotalLiabEquity = computed
			result.Message = "Using computed L+E (L=" + formatFloat(totalLiabilities) + " + E=" + formatFloat(totalEquity) + ")"
		}
	}

	if !foundAssets {
		result.IsValid = false
		result.Message = "Missing Total Assets"
		return result
	}
	if !foundLiabEquity && !(foundLiabilities && foundEquity) {
		result.IsValid = false
		result.Message = "Missing L+E row and components"
		return result
	}
	if !foundLiabEquity && foundLiabilities && foundEquity {
		result.TotalLiabEquity = totalLiabilities + totalEquity
	}

	result.Difference = totalAssets - result.TotalLiabEquity
	if totalAssets != 0 {
		result.DiffPercent = (result.Difference / totalAssets) * 100
	}

	// Allow 1% tolerance for rounding
	result.IsValid = math.Abs(result.DiffPercent) < 1.0
	if result.IsValid {
		if result.Message == "" {
			result.Message = "Balance sheet balances ✓"
		} else {
			result.Message = result.Message + " → Balances ✓"
		}
	} else {
		result.Message = "Balance sheet does NOT balance"
	}

	return result
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.0f", f)
}

// IncomeValidationResult holds income statement validation
type IncomeValidationResult struct {
	IsValid         bool
	Revenues        float64
	COGS            float64
	GrossProfit     float64
	GrossProfitCalc float64
	NetIncome       float64
	Message         string
}

// ValidateIncomeStatement checks Gross Profit = Revenue - COGS
func ValidateIncomeStatement(values []*FSAPValue, year string) *IncomeValidationResult {
	// Auto-detect latest year if not specified
	if year == "" {
		year = getLatestYear(values)
	}

	var revenues, cogs, grossProfit, netIncome float64

	for _, v := range values {
		if v == nil {
			continue
		}
		labelLower := strings.ToLower(v.Label)
		val := getValueForYear(v, year)

		// Match fields
		if strings.Contains(labelLower, "revenue") || strings.Contains(labelLower, "net sales") {
			if !strings.Contains(labelLower, "cost") {
				revenues = val
			}
		}
		if strings.Contains(labelLower, "cost of") && (strings.Contains(labelLower, "sales") || strings.Contains(labelLower, "goods") || strings.Contains(labelLower, "revenue")) {
			cogs = val
		}
		if strings.Contains(labelLower, "gross") && (strings.Contains(labelLower, "profit") || strings.Contains(labelLower, "margin")) {
			grossProfit = val
		}
		if strings.Contains(labelLower, "net income") && !strings.Contains(labelLower, "comprehensive") {
			netIncome = val
		}
	}

	result := &IncomeValidationResult{
		Revenues:        revenues,
		COGS:            cogs,
		GrossProfit:     grossProfit,
		GrossProfitCalc: revenues - cogs,
		NetIncome:       netIncome,
	}

	if revenues > 0 && cogs > 0 && grossProfit > 0 {
		diff := math.Abs(result.GrossProfitCalc - grossProfit)
		tolerance := grossProfit * 0.01 // 1% tolerance
		result.IsValid = diff < tolerance
		if result.IsValid {
			result.Message = "Gross profit calculation verified ✓"
		} else {
			result.Message = "Gross profit mismatch"
		}
	} else {
		result.IsValid = true
		result.Message = "Incomplete data for gross profit validation"
	}

	return result
}

// CashFlowValidationResult holds cash flow validation
type CashFlowValidationResult struct {
	IsValid         bool
	OperatingCF     float64
	InvestingCF     float64
	FinancingCF     float64
	NetChangeCF     float64
	NetChangeCFCalc float64
	Message         string
}

// ValidateCashFlow checks Net Change = Operating + Investing + Financing
func ValidateCashFlow(values []*FSAPValue, year string) *CashFlowValidationResult {
	// Auto-detect latest year if not specified
	if year == "" {
		year = getLatestYear(values)
	}

	var operating, investing, financing, netChange float64

	for _, v := range values {
		if v == nil {
			continue
		}
		labelLower := strings.ToLower(v.Label)
		val := getValueForYear(v, year)

		// Match section totals
		if strings.Contains(labelLower, "operating") && (strings.Contains(labelLower, "net cash") || strings.Contains(labelLower, "cash provided") || strings.Contains(labelLower, "cash generated")) {
			operating = val
		}
		if strings.Contains(labelLower, "investing") && (strings.Contains(labelLower, "net cash") || strings.Contains(labelLower, "cash provided") || strings.Contains(labelLower, "cash used")) {
			investing = val
		}
		if strings.Contains(labelLower, "financing") && (strings.Contains(labelLower, "net cash") || strings.Contains(labelLower, "cash provided") || strings.Contains(labelLower, "cash used")) {
			financing = val
		}
		if (strings.Contains(labelLower, "net increase") || strings.Contains(labelLower, "net decrease") || strings.Contains(labelLower, "net change")) && strings.Contains(labelLower, "cash") {
			netChange = val
		}
	}

	// Try to find FX effect to improve validation accuracy
	var fxEffect float64
	for _, v := range values {
		if v == nil {
			continue
		}
		labelLower := strings.ToLower(v.Label)
		// Look for "effect of exchange rate" or "foreign currency"
		if strings.Contains(labelLower, "exchange rate") || (strings.Contains(labelLower, "foreign currency") && strings.Contains(labelLower, "effect")) {
			// Ensure we are picking up the value for the correct year
			fxEffect = getValueForYear(v, year)
			break // Usually only one such line
		}
	}

	result := &CashFlowValidationResult{
		OperatingCF:     operating,
		InvestingCF:     investing,
		FinancingCF:     financing,
		NetChangeCF:     netChange,
		NetChangeCFCalc: operating + investing + financing + fxEffect,
	}

	if operating != 0 && investing != 0 && financing != 0 && netChange != 0 {
		// Allow larger tolerance for cash flow due to FX effects
		diff := math.Abs(result.NetChangeCFCalc - netChange)
		tolerance := math.Abs(netChange) * 0.05 // 5% tolerance (FX effects)
		if tolerance < 100 {
			tolerance = 100 // Min tolerance of $100M
		}
		result.IsValid = diff < tolerance
		if result.IsValid {
			result.Message = "Cash flow calculation verified ✓"
		} else {
			result.Message = "Cash flow mismatch (may be due to FX effects)"
		}
	} else {
		result.IsValid = true
		result.Message = "Incomplete data for cash flow validation"
	}

	return result
}
