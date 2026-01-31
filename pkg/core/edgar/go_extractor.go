// Package edgar - Go Extractor for precise value extraction
package edgar

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// GO EXTRACTOR - Extracts values from tables using LLM-provided mapping
// =============================================================================

// ParsedTableRow represents a row in a parsed table
type ParsedTableRow struct {
	Index        int
	Label        string
	Values       []string // Value for each column
	MarkdownLine int      // Absolute line number in source document (for jump-to-source)
}

// ParsedTable represents a financial table parsed from markdown
type ParsedTable struct {
	Title      string
	Type       string // "balance_sheet", "income_statement", "cash_flow"
	Headers    []string
	Rows       []ParsedTableRow
	StartLine  int    // Starting line number in source document
	RawContent string // Original markdown content (for unit detection)
}

// GoExtractor extracts values from parsed tables using LineItemMapping
type GoExtractor struct{}

// NewGoExtractor creates a new Go extractor
func NewGoExtractor() *GoExtractor {
	return &GoExtractor{}
}

// ExtractValues extracts FSAPValues from a parsed table using the mapping.
// Standard items → mapped to FSAP variable.
// Unique items (fsap_variable == "UNIQUE") → stored with original label.
func (e *GoExtractor) ExtractValues(table *ParsedTable, mapping *LineItemMapping) []*FSAPValue {
	if table == nil || mapping == nil {
		return nil
	}

	// Detect scale factor from markdown content (e.g. "in millions")
	// Per user request: We DO NOT apply scaling to the values.
	// We store the unit name in metadata for frontend display.
	_, unitName := e.DetectScaleFactor(table.RawContent)

	var values []*FSAPValue

	// Build year column index map
	yearCols := make(map[int]int) // column_index -> year
	for _, yc := range mapping.YearColumns {
		yearCols[yc.ColumnIndex] = yc.Year
	}

	for _, rm := range mapping.RowMappings {
		// Find row by MarkdownLine (most reliable) or by label fallback
		row := e.findRowByMapping(table, rm)
		if row == nil {
			continue // Skip if row not found
		}

		// Build multi-year values (Years map is the primary data source)
		years := make(map[string]float64)
		var latestYear int

		for colIdx, valStr := range row.Values {
			// colIdx is 0-based index of *values* (excluding label).
			// So colIdx 0 corresponds to Table Column 1.
			// LLM usually provides 1-based Table Column index.
			year, ok := yearCols[colIdx+1]
			if !ok {
				continue
			}

			numVal := parseNumericValueFromString(valStr)
			if numVal != nil {
				years[strconv.Itoa(year)] = *numVal
				if year > latestYear {
					latestYear = year
				}
			}
		}

		// Skip if no values extracted
		if len(years) == 0 {
			continue
		}

		// Determine mapping type based on ItemType
		mappingType := "LLM_MAPPED"
		if rm.FSAPVariable == "UNIQUE" || rm.FSAPVariable == "" {
			mappingType = "UNIQUE_ITEM"
		} else if rm.ItemType == ItemTypeSubtotal {
			mappingType = "SUBTOTAL"
		} else if rm.ItemType == ItemTypeTotal {
			mappingType = "TOTAL"
		}

		// Use Go-calculated line number from ParsedTable (more reliable than LLM)
		markdownLine := row.MarkdownLine
		if markdownLine == 0 && rm.MarkdownLine > 0 {
			markdownLine = rm.MarkdownLine // Fallback to LLM-provided
		}

		// Parse numeric values for all year columns
		// ... (logic handled above in aggregation via years map)

		fsapValue := &FSAPValue{
			// Value field intentionally nil - use Years map via aggregation.go
			Years:        years,
			Label:        rm.RowLabel, // Original label preserved
			FSAPVariable: rm.FSAPVariable,
			SourcePath:   table.Title,
			MappingType:  mappingType,
			Confidence:   rm.Confidence,
			SourceType:   SourceInternalDB,
			Provenance: &SourceTrace{
				SectionTitle:  table.Title,
				ParentSection: rm.ParentSection,
				RowLabel:      rm.RowLabel,
				RowIndex:      rm.RowIndex,
				ColumnLabel:   strconv.Itoa(latestYear),
				MarkdownLine:  markdownLine, // Go-calculated for jump-to-source
				Scale:         unitName,     // Store unit metadata
				ExtractedBy:   "GO_EXTRACTOR",
				ExtractedAt:   time.Now().Format(time.RFC3339),
			},
		}

		values = append(values, fsapValue)
	}

	return values
}

// findRowByMapping finds a table row matching the LLM mapping.
// Priority: 1. MarkdownLine match, 2. Label match, 3. RowIndex fallback
func (e *GoExtractor) findRowByMapping(table *ParsedTable, rm RowMapping) *ParsedTableRow {
	// 1. Try MarkdownLine match (most reliable)
	if rm.MarkdownLine > 0 {
		for i := range table.Rows {
			if table.Rows[i].MarkdownLine == rm.MarkdownLine {
				return &table.Rows[i]
			}
		}
	}

	// 2. Try label matching (prefer exact match)
	if rm.RowLabel != "" {
		targetLabel := strings.ToLower(strings.TrimSpace(rm.RowLabel))

		// First pass: exact match
		for i := range table.Rows {
			rowLabel := strings.ToLower(strings.TrimSpace(table.Rows[i].Label))
			if rowLabel == targetLabel {
				return &table.Rows[i]
			}
		}

		// Second pass: contains match, but only if target is more specific
		// This prevents "Total liabilities and equity" matching "Total liabilities"
		for i := range table.Rows {
			rowLabel := strings.ToLower(strings.TrimSpace(table.Rows[i].Label))
			// Only match if target contains row label AND they share significant overlap
			if len(targetLabel) >= len(rowLabel) && strings.Contains(targetLabel, rowLabel) {
				// Skip if this is a partial match that might cause confusion
				// e.g., don't match "Total liabilities" to "Total liabilities and equity"
				continue
			}
			// Match if row label contains target (target is more specific)
			if strings.Contains(rowLabel, targetLabel) {
				return &table.Rows[i]
			}
		}
	}

	// 3. Fallback to RowIndex (legacy, less reliable)
	if rm.RowIndex >= 0 && rm.RowIndex < len(table.Rows) {
		return &table.Rows[rm.RowIndex]
	}

	return nil
}

// ParseMarkdownTable parses a markdown table into ParsedTable.
// lineOffset is the starting line number in the source document (0 if parsing from start).
func (e *GoExtractor) ParseMarkdownTable(markdown string, tableType string) *ParsedTable {
	return e.ParseMarkdownTableWithOffset(markdown, tableType, 0)
}

// ParseMarkdownTableWithOffset parses a markdown table with a line number offset.
// Use this when parsing a section extracted from a larger document.
func (e *GoExtractor) ParseMarkdownTableWithOffset(markdown string, tableType string, lineOffset int) *ParsedTable {
	lines := strings.Split(markdown, "\n")

	table := &ParsedTable{
		Type:       tableType,
		StartLine:  lineOffset,
		RawContent: markdown,
	}

	inTable := false
	headerParsed := false

	for lineIdx, line := range lines {
		absoluteLine := lineOffset + lineIdx + 1 // 1-indexed
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Detect table start
		if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
			inTable = true
			if table.StartLine == 0 {
				table.StartLine = absoluteLine
			}
		} else if inTable {
			// Table ended
			break
		}

		if !inTable {
			// Check for title
			if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "**") {
				table.Title = strings.Trim(line, "#* ")
			}
			continue
		}

		// Skip separator line (|---|---|)
		if strings.Contains(line, "---") {
			continue
		}

		// Parse table row
		cells := parseTableRow(line)

		if !headerParsed {
			table.Headers = cells
			headerParsed = true
			continue
		}

		// Data row with line number tracking
		if len(cells) > 0 {
			// Skip sub-header rows (first column empty or whitespace-only)
			// These are typically date rows like "| | Sep. 28, 2024 | Sep. 30, 2023 |"
			firstCol := strings.TrimSpace(cells[0])
			if firstCol == "" {
				continue // Skip sub-header row
			}

			table.Rows = append(table.Rows, ParsedTableRow{
				Index:        len(table.Rows),
				Label:        cells[0],
				Values:       cells[1:],
				MarkdownLine: absoluteLine, // Track source line number
			})
		}
	}

	return table
}

// parseTableRow splits a markdown table row into cells
func parseTableRow(line string) []string {
	// Remove leading/trailing pipes
	line = strings.Trim(line, "|")

	// Split by pipe
	parts := strings.Split(line, "|")

	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}

	return cells
}

// parseNumericValueFromString extracts numeric value from a cell string
func parseNumericValueFromString(s string) *float64 {
	if s == "" || s == "-" || s == "—" || s == "N/A" {
		return nil
	}

	// Skip date-like values (e.g., "Sep. 28, 2024", "12/31/2023")
	lowerS := strings.ToLower(s)
	datePatterns := []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}
	for _, month := range datePatterns {
		if strings.Contains(lowerS, month) {
			return nil
		}
	}
	// Skip if looks like date format MM/DD/YYYY or similar
	if matched, _ := regexp.MatchString(`^\d{1,2}/\d{1,2}/\d{2,4}$`, strings.TrimSpace(s)); matched {
		return nil
	}

	// Remove common formatting
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, " ", "")

	// Handle parentheses for negative
	isNegative := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		isNegative = true
		s = strings.Trim(s, "()")
	} else if strings.HasPrefix(s, "-") {
		isNegative = true
		s = strings.TrimPrefix(s, "-")
	}

	// Extract numeric value using regex
	re := regexp.MustCompile(`[\d.]+`)
	match := re.FindString(s)
	if match == "" {
		return nil
	}

	val, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return nil
	}

	if isNegative {
		val = -val
	}

	return &val
}

// =============================================================================
// SECTION SLICING - Extract table sections from markdown
// =============================================================================

// SlicedSection represents a section extracted from a larger document
type SlicedSection struct {
	Content   string // Markdown content of the section
	StartLine int    // Starting line number in source document
	EndLine   int    // Ending line number in source document
}

// SliceSection extracts a section from markdown based on SectionLocation.
// Returns the section content and its starting line number.
func SliceSection(markdown string, section *SectionLocation) *SlicedSection {
	if section == nil {
		return nil
	}

	lines := strings.Split(markdown, "\n")
	var result strings.Builder
	var startLine, endLine int
	inSection := false

	for i, line := range lines {
		lineNum := i + 1 // 1-indexed

		// Check if this is the start of our target section
		if !inSection && containsSectionTitle(line, section.Title) {
			inSection = true
			startLine = lineNum
		}

		if inSection {
			result.WriteString(line)
			result.WriteString("\n")
			endLine = lineNum

			// Stop at next major section (next Item or next table header)
			if lineNum > startLine && isNewSectionStart(line) {
				break
			}
		}
	}

	if startLine == 0 {
		return nil
	}

	return &SlicedSection{
		Content:   result.String(),
		StartLine: startLine,
		EndLine:   endLine,
	}
}

// containsSectionTitle checks if line contains the section title
func containsSectionTitle(line, title string) bool {
	lineLower := strings.ToLower(line)
	titleLower := strings.ToLower(title)
	return strings.Contains(lineLower, titleLower)
}

// isNewSectionStart detects if this line starts a new major section
func isNewSectionStart(line string) bool {
	line = strings.TrimSpace(line)
	// Markdown headers
	if strings.HasPrefix(line, "##") || strings.HasPrefix(line, "# ") {
		return true
	}
	// SEC Item headers
	if strings.HasPrefix(strings.ToLower(line), "item ") {
		return true
	}
	return false
}

// =============================================================================
// CROSS VALIDATION - Compare calculated vs reported values
// =============================================================================

// FieldValidationResult represents the result of cross-validating calculated vs reported values
type FieldValidationResult struct {
	FieldName       string  `json:"field_name"`
	CalculatedValue float64 `json:"calculated_value"`
	ReportedValue   float64 `json:"reported_value"`
	Difference      float64 `json:"difference"`
	PercentDiff     float64 `json:"percent_diff"` // Percentage difference
	Match           bool    `json:"match"`        // True if within tolerance
}

// ValidationReport contains all validation results for a statement
type ValidationReport struct {
	StatementType string                   `json:"statement_type"`
	FiscalYear    string                   `json:"fiscal_year"`
	Results       []*FieldValidationResult `json:"results"`
	AllPassed     bool                     `json:"all_passed"`
	ErrorCount    int                      `json:"error_count"`
}

// Tolerance for matching (0.01 = 1% tolerance)
const ValidationTolerance = 0.01

// ValidateAgainstReported compares calculated totals against reported subtotals/totals.
// Items: regular line items (used for calculation)
// Subtotals/Totals: reported values from 10-K (used for validation)
func ValidateAgainstReported(
	calculatedTotals map[string]float64, // e.g., {"total_current_assets": 19700}
	reportedValues []*FSAPValue, // Subtotals/Totals from extraction
	fiscalYear string,
) *ValidationReport {
	report := &ValidationReport{
		FiscalYear: fiscalYear,
		AllPassed:  true,
	}

	for _, rv := range reportedValues {
		if rv == nil {
			continue
		}

		// Get reported value for the fiscal year
		reported, ok := rv.Years[fiscalYear]
		if !ok {
			continue
		}

		// Look up the calculated value using the FSAP variable name
		// We need to match Label to calculated key
		fsapVar := labelToFSAPKey(rv.Label)
		calculated, exists := calculatedTotals[fsapVar]
		if !exists {
			continue
		}

		// Calculate difference
		diff := calculated - reported
		var percentDiff float64
		if reported != 0 {
			percentDiff = diff / reported
		}

		match := abs(percentDiff) <= ValidationTolerance

		result := &FieldValidationResult{
			FieldName:       rv.Label,
			CalculatedValue: calculated,
			ReportedValue:   reported,
			Difference:      diff,
			PercentDiff:     percentDiff * 100, // Convert to percentage
			Match:           match,
		}

		report.Results = append(report.Results, result)

		if !match {
			report.AllPassed = false
			report.ErrorCount++
		}
	}

	return report
}

// labelToFSAPKey converts a label like "Total current assets" to "total_current_assets"
func labelToFSAPKey(label string) string {
	label = strings.ToLower(label)
	label = strings.ReplaceAll(label, " ", "_")
	label = strings.ReplaceAll(label, ",", "")
	label = strings.ReplaceAll(label, "(", "")
	label = strings.ReplaceAll(label, ")", "")
	return label
}

// abs returns absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
