// Package fee - Table Parser for structured table extraction
package fee

import (
	"log"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// =============================================================================
// TABLE PARSER - Extract structured data from HTML tables
// =============================================================================

// TableParser extracts ParsedTable structures from HTML
type TableParser struct {
	matcher    *TableMatcher
	classifier *RowClassifier
	fsapMapper *FSAPMapper
}

// NewTableParser creates a new parser with all components
func NewTableParser() *TableParser {
	return &TableParser{
		matcher:    NewTableMatcher(),
		classifier: NewRowClassifier(),
		fsapMapper: NewFSAPMapper(),
	}
}

// ParseHTMLTables extracts all financial tables from HTML content
func (p *TableParser) ParseHTMLTables(html string) ([]ParsedTable, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var tables []ParsedTable
	position := 0
	totalTables := 0
	tablesWithTitles := 0
	identifiedTables := 0

	// Phase 1: Scan all tables
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		totalTables++
		// Try to find table title from surrounding context
		title := p.findTableTitle(table)

		// Identify table type
		tableType := p.matcher.IdentifyTableType(title, "")

		// Debug: Log tables (both with and without titles for first 10)
		if title != "" {
			tablesWithTitles++
			log.Printf("[TableParser] Table #%d title: %q -> type: %s", i, title, tableType)
		} else if i < 10 {
			// Log first 10 tables without titles for debugging
			firstRow := ""
			table.Find("tr").First().Find("td, th").Each(func(j int, cell *goquery.Selection) {
				if j < 3 {
					text := strings.TrimSpace(cell.Text())
					if len(text) > 50 {
						text = text[:50] + "..."
					}
					if firstRow != "" {
						firstRow += " | "
					}
					firstRow += text
				}
			})
			log.Printf("[TableParser] Table #%d no title, first row preview: %q", i, firstRow)
		}

		// Skip unknown tables for now (can be added later if useful)
		if tableType == TableTypeUnknown {
			return
		}

		identifiedTables++

		// Parse the table structure
		parsed := p.parseTable(table, title, tableType, position)
		if parsed != nil {
			tables = append(tables, *parsed)
			position++
		}
	})

	// Summary log
	log.Printf("[TableParser] SUMMARY: total=%d, with_titles=%d, identified=%d, parsed=%d",
		totalTables, tablesWithTitles, identifiedTables, len(tables))

	return tables, nil
}

// findTableTitle extracts the title from before the table or first row
func (p *TableParser) findTableTitle(table *goquery.Selection) string {
	// Check for preceding elements
	if prev := table.Prev(); prev.Length() > 0 {
		text := strings.TrimSpace(prev.Text())
		// Check if it looks like a title (contains key words)
		lower := strings.ToLower(text)
		if strings.Contains(lower, "balance") ||
			strings.Contains(lower, "statement") ||
			strings.Contains(lower, "income") ||
			strings.Contains(lower, "cash flow") {
			return text
		}
	}

	// Check first row for title
	firstRow := table.Find("tr").First()
	if firstRow.Length() > 0 {
		// If only one cell and it looks like a title
		cells := firstRow.Find("td, th")
		if cells.Length() == 1 {
			return strings.TrimSpace(cells.Text())
		}
	}

	return ""
}

// parseTable extracts structured data from a single table
func (p *TableParser) parseTable(table *goquery.Selection, title string, tableType TableType, position int) *ParsedTable {
	rows := table.Find("tr")
	if rows.Length() < 2 {
		return nil // Need at least header + 1 data row
	}

	// Detect scale from title or nearby text
	scale := DetectScale(title)

	// Parse header row(s)
	var columnHeaders []ColumnHeader
	var dataRowStartIndex int

	rows.EachWithBreak(func(i int, row *goquery.Selection) bool {
		cells := row.Find("td, th")
		if cells.Length() == 0 {
			return true // Continue
		}

		// Check if this looks like a header row (contains year patterns)
		var headers []string
		cells.Each(func(j int, cell *goquery.Selection) {
			headers = append(headers, strings.TrimSpace(cell.Text()))
		})

		// Check if any cell contains a year
		hasYear := false
		for _, h := range headers {
			if ParseColumnYear(h) > 0 {
				hasYear = true
				break
			}
		}

		if hasYear {
			columnHeaders = ParseColumnHeaders(headers)
			dataRowStartIndex = i + 1
			return false // Break
		}

		return true
	})

	// If no year headers found, use first row as header
	if len(columnHeaders) == 0 {
		dataRowStartIndex = 1
	}

	// Parse data rows
	var tableRows []TableRow
	rows.Slice(dataRowStartIndex, rows.Length()).Each(func(i int, row *goquery.Selection) {
		cells := row.Find("td, th")
		if cells.Length() == 0 {
			return
		}

		// First cell is typically the label
		label := ""
		var values []CellValue

		cells.Each(func(j int, cell *goquery.Selection) {
			text := strings.TrimSpace(cell.Text())
			if j == 0 {
				label = text
			} else {
				cv := ParseCellValue(text)
				cv.ColumnIndex = j - 1
				values = append(values, cv)
			}
		})

		// Skip empty labels
		if label == "" {
			return
		}

		isTotal, isHeader := p.classifier.ClassifyRow(label)
		indent := DetectIndentLevel(label, cells.First().Text())

		tableRows = append(tableRows, TableRow{
			Index:    i,
			Label:    label,
			Indent:   indent,
			IsTotal:  isTotal,
			IsHeader: isHeader,
			Values:   values,
		})
	})

	// Generate table ID
	tableID := GenerateTableID(title, len(tableRows), len(columnHeaders))

	return &ParsedTable{
		ID:             tableID,
		Type:           tableType,
		Title:          title,
		Position:       position,
		Columns:        columnHeaders,
		Rows:           tableRows,
		IsConsolidated: IsConsolidated(title),
		Scale:          scale,
		Currency:       "USD", // Default, could be enhanced
	}
}

// GetValueForYear extracts the value from a row for a specific year
func (p *TableParser) GetValueForYear(table *ParsedTable, rowIndex int, targetYear int) *CellValue {
	if rowIndex < 0 || rowIndex >= len(table.Rows) {
		return nil
	}

	row := table.Rows[rowIndex]

	// Find the column with the target year
	for _, col := range table.Columns {
		if col.Year == targetYear && col.Index-1 < len(row.Values) {
			return &row.Values[col.Index-1]
		}
	}

	// If no specific year column, try to use the first value
	if len(row.Values) > 0 {
		return &row.Values[0]
	}

	return nil
}

// GetLatestYearColumn returns the column with the most recent year
func GetLatestYearColumn(columns []ColumnHeader) *ColumnHeader {
	for i := range columns {
		if columns[i].IsLatest {
			return &columns[i]
		}
	}
	if len(columns) > 0 {
		return &columns[0]
	}
	return nil
}

// =============================================================================
// DOCUMENT PARSER - Full 10-K document parsing
// =============================================================================

// DocumentParser parses a complete 10-K into a DocumentIndex
type DocumentParser struct {
	tableParser *TableParser
}

// NewDocumentParser creates a new document parser
func NewDocumentParser() *DocumentParser {
	return &DocumentParser{
		tableParser: NewTableParser(),
	}
}

// ParseDocument creates a DocumentIndex from HTML content
func (dp *DocumentParser) ParseDocument(html string, metadata DocumentMetadata) (*DocumentIndex, error) {
	// Parse all tables
	tables, err := dp.tableParser.ParseHTMLTables(html)
	if err != nil {
		return nil, err
	}

	// Debug: Log table parsing results
	log.Printf("[DocumentParser] Found %d financial tables", len(tables))
	for i, t := range tables {
		log.Printf("[DocumentParser] Table %d: type=%s, rows=%d, cols=%d, title=%q",
			i, t.Type, len(t.Rows), len(t.Columns), t.Title)
	}

	// Collect available years from all tables
	yearsMap := make(map[int]bool)
	for _, table := range tables {
		for _, col := range table.Columns {
			if col.Year > 0 {
				yearsMap[col.Year] = true
			}
		}
	}

	var availableYears []int
	for year := range yearsMap {
		availableYears = append(availableYears, year)
	}
	// Sort descending
	for i := 0; i < len(availableYears)-1; i++ {
		for j := i + 1; j < len(availableYears); j++ {
			if availableYears[j] > availableYears[i] {
				availableYears[i], availableYears[j] = availableYears[j], availableYears[i]
			}
		}
	}

	return &DocumentIndex{
		Metadata:       metadata,
		Tables:         tables,
		AvailableYears: availableYears,
	}, nil
}
