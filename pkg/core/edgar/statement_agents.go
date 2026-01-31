package edgar

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// DetectedUnits holds scale information extracted from financial table headers
type DetectedUnits struct {
	Scale      float64 // 1000, 1000000, or 1
	ScaleLabel string  // "thousands", "millions", "dollars"
	ShareScale float64 // Separate scale for share counts (may differ from dollar amounts)
	Currency   string  // "USD", "EUR", etc.
}

// DetectUnits scans markdown for unit declarations in table headers.
// This replaces hardcoded "millions" assumptions in prompts.
// Returns detected scale factor and label.
func DetectUnits(markdown string) *DetectedUnits {
	result := &DetectedUnits{
		Scale:      1000000, // Default to millions if not detected
		ScaleLabel: "millions",
		ShareScale: 1000000,
		Currency:   "USD",
	}

	// Unit detection patterns (priority order - first match wins)
	patterns := []struct {
		regex      string
		scale      float64
		scaleLabel string
	}{
		// Explicit "in thousands" patterns
		{`(?i)in\s+thousands`, 1000, "thousands"},
		{`(?i)\(\s*in\s+thousands\s*\)`, 1000, "thousands"},
		{`(?i)\$\s*000s?`, 1000, "thousands"},
		{`(?i)amounts\s+in\s+thousands`, 1000, "thousands"},
		{`(?i)thousands\s+of\s+dollars`, 1000, "thousands"},

		// Explicit "in millions" patterns
		{`(?i)in\s+millions`, 1000000, "millions"},
		{`(?i)\(\s*in\s+millions\s*\)`, 1000000, "millions"},
		{`(?i)\$\s*MM`, 1000000, "millions"},
		{`(?i)\$M`, 1000000, "millions"},
		{`(?i)amounts\s+in\s+millions`, 1000000, "millions"},
		{`(?i)millions\s+of\s+dollars`, 1000000, "millions"},

		// Explicit "in billions" patterns
		{`(?i)in\s+billions`, 1000000000, "billions"},
		{`(?i)\(\s*in\s+billions\s*\)`, 1000000000, "billions"},

		// Raw dollars (no scale)
		{`(?i)amounts\s+in\s+dollars`, 1, "dollars"},
	}

	// Scan first 5000 chars (where unit declarations typically appear)
	scanRegion := markdown
	if len(scanRegion) > 5000 {
		scanRegion = scanRegion[:5000]
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.regex)
		if re.MatchString(scanRegion) {
			result.Scale = p.scale
			result.ScaleLabel = p.scaleLabel
			break
		}
	}

	// Check for separate share scale (e.g., "shares in thousands")
	sharePatterns := []struct {
		regex string
		scale float64
	}{
		{`(?i)shares?\s+in\s+thousands`, 1000},
		{`(?i)shares?\s+in\s+millions`, 1000000},
		{`(?i)except\s+(?:per\s+)?share`, 1}, // Indicates share data is in single units
	}

	for _, p := range sharePatterns {
		re := regexp.MustCompile(p.regex)
		if re.MatchString(scanRegion) {
			result.ShareScale = p.scale
			break
		}
	}

	return result
}

// StatementType represents different financial statement types
type StatementType string

const (
	BalanceSheetType    StatementType = "BALANCE_SHEET"
	IncomeStatementType StatementType = "INCOME_STATEMENT"
	CashFlowType        StatementType = "CASH_FLOW"
	SupplementalType    StatementType = "SUPPLEMENTAL"
	BusinessType        StatementType = "BUSINESS"
	RiskFactorsType     StatementType = "RISK_FACTORS"
	MDAType             StatementType = "MDA"
	NotesType           StatementType = "NOTES"
)

// ParseStatementType converts a string to StatementType
func ParseStatementType(s string) (StatementType, bool) {
	switch strings.ToLower(s) {
	case "bs", "balance_sheet", "balance sheet":
		return BalanceSheetType, true
	case "is", "income_statement", "income statement":
		return IncomeStatementType, true
	case "cf", "cash_flow", "cash flow":
		return CashFlowType, true
	case "sp", "supplemental":
		return SupplementalType, true
	default:
		return "", false
	}
}

// truncateForLog truncates a string for logging purposes
func truncateForLog(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// populateValuesFromYears populates the Value field from the Years map for calc package compatibility.
// The v2.0 extractor fills Years map, but pkg/core/calc reads the Value field directly.
func populateValuesFromYears(resp *FSAPDataResponse) {
	if resp == nil {
		return
	}
	targetYear := fmt.Sprintf("%d", resp.FiscalYear)

	// Process each statement
	populateStruct(&resp.BalanceSheet, targetYear)
	populateStruct(&resp.IncomeStatement, targetYear)
	populateStruct(&resp.CashFlowStatement, targetYear)
	populateStruct(&resp.SupplementalData, targetYear)
}

// populateStruct uses reflection to traverse FSAP structs and populate Value from Years
func populateStruct(v interface{}, year string) {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}

	// Check if this is an FSAPValue
	// We detect by checking for "Value" (ptr float64) and "Years" (map) fields
	valueField := val.FieldByName("Value")
	yearsField := val.FieldByName("Years")

	if valueField.IsValid() && yearsField.IsValid() && valueField.CanSet() {
		// This is likely an FSAPValue or similar struct
		// But skip AdditionalItem which has Years but Value is *FSAPValue, not *float64
		// Check that Value field is of type *float64
		if valueField.Type().String() != "*float64" {
			// Not an FSAPValue-style struct, skip direct population
			// But still need to recurse into nested structures
			goto recurse
		}
		if yearsField.Kind() == reflect.Map && !yearsField.IsNil() {
			// Find year value
			mapVal := yearsField.MapIndex(reflect.ValueOf(year))
			if mapVal.IsValid() {
				// Set Value field
				floatVal := mapVal.Float()
				valueField.Set(reflect.ValueOf(&floatVal))
			}
		}
		return // Stop recursing if we found a leaf node
	}

recurse:
	// Handle AdditionalItems arrays
	if val.Type().Name() == "AdditionalItem" {
		// Populate value for AdditionalItem wrapper
		if val.FieldByName("Value").IsValid() {
			// AdditionalItem.Value is of type *FSAPValue, so recurse into it
			populateStruct(val.FieldByName("Value").Interface(), year)
		}
		return
	}

	// Recurse into fields
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)

		// Handle slices (e.g. AdditionalItems)
		if field.Kind() == reflect.Slice {
			for j := 0; j < field.Len(); j++ {
				populateStruct(field.Index(j).Addr().Interface(), year)
			}
			continue
		}

		// Handle nested structs/pointers
		if field.Kind() == reflect.Ptr || field.Kind() == reflect.Struct {
			populateStruct(field.Interface(), year)
		}
	}
}
