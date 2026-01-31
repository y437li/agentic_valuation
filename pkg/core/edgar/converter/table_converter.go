package converter

import (
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// TableConverter handles complex HTML table to Markdown conversion
// using a "Virtual Grid" approach to handle colspan and rowspan correctly.
type TableConverter struct{}

type Cell struct {
	Text    string
	ColSpan int
	RowSpan int
	InGrid  bool // processed marker
}

// ConvertTableToMarkdown parses an HTML table and renders strictly aligned Markdown
func (tc *TableConverter) ConvertTableToMarkdown(tableHTML string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(tableHTML))
	if err != nil {
		return ""
	}

	// 1. Build Virtual Grid
	rows := doc.Find("tr")
	if rows.Length() == 0 {
		return ""
	}

	// Dynamic grid sizing
	maxCols := 0
	rowCount := rows.Length()

	// Pre-scan to estimate max cols (imperfect but helpful)
	rows.Each(func(i int, s *goquery.Selection) {
		cells := s.Find("td, th")
		localCols := 0
		cells.Each(func(_ int, cell *goquery.Selection) {
			colspan, _ := strconv.Atoi(cell.AttrOr("colspan", "1"))
			if colspan < 1 {
				colspan = 1
			}
			localCols += colspan
		})
		if localCols > maxCols {
			maxCols = localCols
		}
	})

	// Initialize Grid
	grid := make([][]string, rowCount)
	for i := range grid {
		grid[i] = make([]string, maxCols)
	}

	// 2. Populate Grid (handling spans)
	rowIdx := 0
	rows.Each(func(i int, tr *goquery.Selection) {
		colIdx := 0

		// Find next empty slot in this row (skipping spots taken by rowspans from above)
		for colIdx < maxCols && grid[rowIdx][colIdx] != "" {
			colIdx++
		}

		tr.Find("td, th").Each(func(_ int, cell *goquery.Selection) {
			// Parse attributes
			colspan, _ := strconv.Atoi(cell.AttrOr("colspan", "1"))
			rowspan, _ := strconv.Atoi(cell.AttrOr("rowspan", "1"))
			if colspan < 1 {
				colspan = 1
			}
			if rowspan < 1 {
				rowspan = 1
			}

			text := cleanCellText(cell.Text())

			// Fill the main cell and span placeholders
			for r := 0; r < rowspan; r++ {
				for c := 0; c < colspan; c++ {
					targetRow := rowIdx + r
					targetCol := colIdx + c

					if targetRow < rowCount && targetCol < maxCols {
						if r == 0 && c == 0 {
							grid[targetRow][targetCol] = text
						} else {
							// For spanned cells, we can either repeat the value
							// or leave distinct marker. For Markdown tables,
							// repeating header values is often clearer,
							// but for data, empty is safer.
							// Strategy:
							// - If it's a header (row 0 or 1), repeat text for colspan to make "Super Header" clear?
							// - Actually, standard Markdown doesn't support colspan.
							// - "Exploding" spans means: Value | Value | Value
							//   so that columns align.
							if r == 0 {
								// Horizontal span: Repeat value to maintain column context?
								// Or empty? If we leave empty, the column has no header.
								// Let's use a placeholder symbol "." or just empty space.
								grid[targetRow][targetCol] = " "
							} else {
								// Vertical span (rowspan)
								// Repeat value downwards? definitive for "merged" row headers.
								// e.g. "Assets" | "Current" -> "Assets" | "Non-Current"
								grid[targetRow][targetCol] = " " // quote: "md"
							}
						}
					}
				}
			}

			// Find next start point (jump over the colspan we just consumed)
			colIdx += colspan
			// Skip any slots already filled by previous rowspans
			for colIdx < maxCols && grid[rowIdx][colIdx] != "" {
				colIdx++
			}
		})
		rowIdx++
	})

	// 3. Render to Markdown
	var sb strings.Builder
	sb.WriteString("\n")

	// Render Rows
	for i, row := range grid {
		sb.WriteString("|")
		for _, cell := range row {
			if cell == "" {
				cell = " "
			} // default empty
			sb.WriteString(" " + cell + " |")
		}
		sb.WriteString("\n")

		// Add separator after first row (Header)
		if i == 0 {
			sb.WriteString("|")
			for range row {
				sb.WriteString(" --- |")
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

func cleanCellText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "|", "&#124;") // Escape pipes

	// CRITICAL: Convert accounting-style negative (parentheses) to actual negative numbers
	// Pattern: (1,234.56) -> -1234.56
	// Also handles: (1234), ($1,234), etc.
	normalized := normalizeNumber(text)
	if normalized != text {
		// fmt.Printf("[TableConverter] Normalized: '%s' -> '%s'\n", text, normalized)
	}
	text = normalized

	if text == "" {
		return " "
	}
	return text
}

// normalizeNumber converts accounting-format numbers to standard numbers.
// - Parentheses indicate negative: (1,234) -> -1234
// - Removes thousand separators (commas)
// - Removes currency symbols ($, €, etc.)
// - Preserves non-numeric strings as-is
func normalizeNumber(text string) string {
	original := text

	// Quick bail: if it doesn't look like a number, return as-is
	// Check for at least one digit
	hasDigit := false
	for _, r := range text {
		if r >= '0' && r <= '9' {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		return original
	}

	// Check if wrapped in parentheses (accounting negative format)
	isNegative := false
	if strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		isNegative = true
		text = text[1 : len(text)-1] // Remove parens
	}

	// Remove currency symbols
	text = strings.ReplaceAll(text, "$", "")
	text = strings.ReplaceAll(text, "€", "")
	text = strings.ReplaceAll(text, "£", "")
	text = strings.ReplaceAll(text, "¥", "")

	// Remove thousand separators (commas)
	text = strings.ReplaceAll(text, ",", "")

	// Remove common extraneous chars
	text = strings.TrimSpace(text)

	// After cleanup, check if it's a valid-looking number
	// If it has weird characters left, return original (e.g., "N/A", "—")
	validNumber := true
	for _, r := range text {
		if !((r >= '0' && r <= '9') || r == '.' || r == '-') {
			validNumber = false
			break
		}
	}
	if !validNumber {
		return original
	}

	// Apply negative sign if detected from parentheses
	if isNegative && !strings.HasPrefix(text, "-") {
		text = "-" + text
	}

	return text
}
