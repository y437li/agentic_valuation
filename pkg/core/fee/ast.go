// Package fee implements the Financial Extraction Engine (FEE).
// It provides structured parsing and semantic extraction of SEC 10-K filings.
package fee

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// DOCUMENT INDEX - Core data structure for parsed 10-K
// =============================================================================

// DocumentIndex represents a fully parsed 10-K document
type DocumentIndex struct {
	Metadata       DocumentMetadata `json:"metadata"`
	Sections       []Section        `json:"sections"`
	Tables         []ParsedTable    `json:"tables"`
	Notes          []Note           `json:"notes"`
	AvailableYears []int            `json:"available_years"`
	ParsedAt       time.Time        `json:"parsed_at"`
}

// DocumentMetadata contains filing-level information
type DocumentMetadata struct {
	CIK             string `json:"cik"`
	CompanyName     string `json:"company_name"`
	FilingDate      string `json:"filing_date"`
	FiscalYearEnd   string `json:"fiscal_year_end"` // e.g., "December 31"
	Form            string `json:"form"`            // "10-K", "10-Q"
	AccessionNumber string `json:"accession_number"`
}

// Section represents a major section of the 10-K
type Section struct {
	ID       string `json:"id"`    // e.g., "item8"
	Title    string `json:"title"` // e.g., "Financial Statements and Supplementary Data"
	StartPos int    `json:"start_pos"`
	EndPos   int    `json:"end_pos"`
	Content  string `json:"content,omitempty"` // Raw text/markdown
}

// =============================================================================
// PARSED TABLE - Structured financial table
// =============================================================================

// ParsedTable represents a financial table extracted from the document
type ParsedTable struct {
	ID             string         `json:"id"`       // SHA-256 fingerprint
	Type           TableType      `json:"type"`     // BALANCE_SHEET, INCOME_STATEMENT, etc.
	Title          string         `json:"title"`    // Original table title
	PageRef        string         `json:"page_ref"` // e.g., "F-3"
	Position       int            `json:"position"` // Position in document
	Columns        []ColumnHeader `json:"columns"`  // Column definitions with years
	Rows           []TableRow     `json:"rows"`     // Parsed rows
	IsConsolidated bool           `json:"is_consolidated"`
	Scale          Scale          `json:"scale"`    // millions, thousands, units
	Currency       string         `json:"currency"` // USD, EUR, etc.
}

// TableType identifies the type of financial statement
type TableType string

const (
	TableTypeBalanceSheet        TableType = "BALANCE_SHEET"
	TableTypeIncomeStatement     TableType = "INCOME_STATEMENT"
	TableTypeCashFlow            TableType = "CASH_FLOW"
	TableTypeComprehensiveIncome TableType = "COMPREHENSIVE_INCOME"
	TableTypeEquity              TableType = "STOCKHOLDERS_EQUITY"
	TableTypeUnknown             TableType = "UNKNOWN"
)

// Scale represents the unit scale of values
type Scale string

const (
	ScaleMillions  Scale = "millions"
	ScaleThousands Scale = "thousands"
	ScaleUnits     Scale = "units"
	ScaleUnknown   Scale = "unknown"
)

// ColumnHeader represents a table column with year information
type ColumnHeader struct {
	Index      int    `json:"index"`
	Label      string `json:"label"`       // e.g., "December 31, 2024"
	Year       int    `json:"year"`        // Parsed year: 2024
	PeriodType string `json:"period_type"` // "instant" (BS) or "duration" (IS/CF)
	IsLatest   bool   `json:"is_latest"`   // True for most recent year
}

// TableRow represents a single row in a financial table
type TableRow struct {
	Index    int         `json:"index"`
	Label    string      `json:"label"`              // Original row text
	Indent   int         `json:"indent"`             // Nesting level (0, 1, 2...)
	IsTotal  bool        `json:"is_total"`           // Is this a subtotal/total row?
	IsHeader bool        `json:"is_header"`          // Is this a section header?
	Values   []CellValue `json:"values"`             // Values for each column
	XBRLTag  string      `json:"xbrl_tag,omitempty"` // If inline XBRL present
}

// CellValue represents a single cell value
type CellValue struct {
	ColumnIndex int      `json:"column_index"`
	RawText     string   `json:"raw_text"`    // Original text: "$ (1,234)"
	Value       *float64 `json:"value"`       // Parsed value: -1234
	IsNegative  bool     `json:"is_negative"` // Was in parentheses?
	IsBlank     bool     `json:"is_blank"`    // Empty cell?
}

// Note represents a note to financial statements
type Note struct {
	Number   int    `json:"number"`  // Note 1, 2, etc.
	Title    string `json:"title"`   // "Summary of Significant Accounting Policies"
	Content  string `json:"content"` // Full text
	Position int    `json:"position"`
}

// =============================================================================
// TABLE ID GENERATION - Fingerprinting for deduplication
// =============================================================================

// GenerateTableID creates a unique fingerprint for a table
func GenerateTableID(title string, rowCount int, colCount int) string {
	data := title + strconv.Itoa(rowCount) + strconv.Itoa(colCount)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // First 16 chars
}

// =============================================================================
// COLUMN YEAR PARSING - Extract year from column headers
// =============================================================================

// ParseColumnYear extracts the year from a column label
// Examples:
//
//	"December 31, 2024" → 2024
//	"Year Ended December 31, 2023" → 2023
//	"2024" → 2024
//	"As of 12/31/2024" → 2024
func ParseColumnYear(label string) int {
	// Pattern 1: Full year in text (e.g., "2024")
	yearPattern := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	matches := yearPattern.FindAllString(label, -1)
	if len(matches) > 0 {
		// Return the last year found (usually the one we want)
		year, _ := strconv.Atoi(matches[len(matches)-1])
		return year
	}
	return 0
}

// ParseColumnHeaders extracts structured columns from raw header text
func ParseColumnHeaders(headerRow []string) []ColumnHeader {
	columns := make([]ColumnHeader, 0, len(headerRow))
	maxYear := 0

	for i, label := range headerRow {
		year := ParseColumnYear(label)
		if year > maxYear {
			maxYear = year
		}
		columns = append(columns, ColumnHeader{
			Index: i,
			Label: strings.TrimSpace(label),
			Year:  year,
		})
	}

	// Mark the latest year column
	for i := range columns {
		if columns[i].Year == maxYear {
			columns[i].IsLatest = true
		}
	}

	return columns
}

// =============================================================================
// VALUE PARSING - Clean and parse financial values
// =============================================================================

// ParseCellValue parses a raw cell text into a structured value
// Handles:
//
//	"(1,234)" → -1234 (parentheses = negative)
//	"$1,234.56" → 1234.56
//	"—" or "-" → nil (blank)
//	"1,234" → 1234
func ParseCellValue(raw string) CellValue {
	raw = strings.TrimSpace(raw)

	// Check for blank indicators
	if raw == "" || raw == "—" || raw == "-" || raw == "–" || raw == "N/A" {
		return CellValue{RawText: raw, IsBlank: true}
	}

	// Check for parentheses (negative)
	isNegative := strings.Contains(raw, "(") && strings.Contains(raw, ")")

	// Remove all non-numeric characters except decimal point and minus
	cleanPattern := regexp.MustCompile(`[^0-9.\-]`)
	cleaned := cleanPattern.ReplaceAllString(raw, "")

	// Handle empty after cleaning
	if cleaned == "" || cleaned == "." || cleaned == "-" {
		return CellValue{RawText: raw, IsBlank: true}
	}

	// Parse the number
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return CellValue{RawText: raw, IsBlank: true}
	}

	// Apply negative sign if parentheses were present
	if isNegative && value > 0 {
		value = -value
	}

	return CellValue{
		RawText:    raw,
		Value:      &value,
		IsNegative: isNegative,
		IsBlank:    false,
	}
}

// =============================================================================
// SCALE DETECTION - Determine if values are in millions, thousands, etc.
// =============================================================================

// DetectScale analyzes table title/header to determine value scale
// Examples:
//
//	"(in millions)" → ScaleMillions
//	"($ in thousands)" → ScaleThousands
//	"(in millions, except per share)" → ScaleMillions
func DetectScale(text string) Scale {
	lower := strings.ToLower(text)

	if strings.Contains(lower, "million") {
		return ScaleMillions
	}
	if strings.Contains(lower, "thousand") {
		return ScaleThousands
	}
	if strings.Contains(lower, "in units") || strings.Contains(lower, "per share") {
		return ScaleUnits
	}

	return ScaleUnknown
}
