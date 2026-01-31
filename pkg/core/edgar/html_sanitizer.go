package edgar

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// HTMLSanitizer performs pre-Pandoc HTML cleaning specific to SEC EDGAR filings.
// It addresses three main issues:
// 1. Fake headers (styled <p> instead of semantic <h2>/<h3>)
// 2. Complex tables that need specialized conversion
// 3. Noise (footers, transparent images, pagination)
type HTMLSanitizer struct {
	// tableStore maps placeholder IDs to original HTML table content
	tableStore map[string]string
	tableCount int
}

// NewHTMLSanitizer creates a new sanitizer instance
func NewHTMLSanitizer() *HTMLSanitizer {
	return &HTMLSanitizer{
		tableStore: make(map[string]string),
		tableCount: 0,
	}
}

// Sanitize performs all pre-processing steps on raw HTML
// Returns cleaned HTML ready for Pandoc conversion
func (s *HTMLSanitizer) Sanitize(htmlContent string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Step 1: Remove noise elements
	s.RemoveNoise(doc)

	// Step 1.5: Preserve Anchors (Critical for navigation and extraction)
	// Converts <a name="x"> to [ANCHOR:x] so Pandoc preserves it
	s.PreserveAnchors(doc)

	// Step 2: Fix fake headers (before table extraction so table headers are also fixed)
	s.FixFakeHeaders(doc)

	// Step 3: Extract tables and replace with placeholders
	s.ExtractTablesWithPlaceholders(doc)

	// Get cleaned HTML
	cleanedHTML, err := doc.Find("body").Html()
	if err != nil {
		cleanedHTML, _ = doc.Html()
	}

	return cleanedHTML, nil
}

// FixFakeHeaders converts styled <p> and <span> elements to semantic headers.
// SEC EDGAR filings often use inline styles instead of proper <h2>/<h3> tags.
func (s *HTMLSanitizer) FixFakeHeaders(doc *goquery.Document) {
	// Pattern 1: Bold + large font-size paragraphs → h2
	doc.Find("p").Each(func(i int, sel *goquery.Selection) {
		style, exists := sel.Attr("style")
		if !exists {
			return
		}
		styleLower := strings.ToLower(style)

		// Check for bold (font-weight:bold or font-weight:700)
		isBold := strings.Contains(styleLower, "font-weight:bold") ||
			strings.Contains(styleLower, "font-weight: bold") ||
			strings.Contains(styleLower, "font-weight:700") ||
			strings.Contains(styleLower, "font-weight: 700")

		if !isBold {
			return
		}

		// Check font-size to determine h2 vs h3
		// 14pt+ → h2, 12pt+ → h3
		if s.hasFontSize(styleLower, 14) {
			s.convertToHeader(sel, "h2")
		} else if s.hasFontSize(styleLower, 12) {
			s.convertToHeader(sel, "h3")
		}
	})

	// Pattern 2: Bold spans with large font → convert parent or wrap
	doc.Find("span").Each(func(i int, sel *goquery.Selection) {
		style, exists := sel.Attr("style")
		if !exists {
			return
		}
		styleLower := strings.ToLower(style)

		isBold := strings.Contains(styleLower, "font-weight:bold") ||
			strings.Contains(styleLower, "font-weight: bold") ||
			strings.Contains(styleLower, "font-weight:700")

		if isBold && s.hasFontSize(styleLower, 14) {
			// If parent is a <p>, convert the parent
			parent := sel.Parent()
			if goquery.NodeName(parent) == "p" {
				s.convertToHeader(parent, "h2")
			}
		}
	})

	// Pattern 3: <b> or <strong> tags that look like section headers
	doc.Find("b, strong").Each(func(i int, sel *goquery.Selection) {
		text := strings.TrimSpace(sel.Text())
		// Check if it looks like a section header (Item X, PART X, etc.)
		if s.looksLikeSectionHeader(text) {
			parent := sel.Parent()
			if goquery.NodeName(parent) == "p" || goquery.NodeName(parent) == "div" {
				s.convertToHeader(parent, "h2")
			}
		}
	})
}

// PreserveAnchors converts HTML anchors to text markers consistent with parser expectations.
func (s *HTMLSanitizer) PreserveAnchors(doc *goquery.Document) {
	// Look for any element that might act as an anchor (a, div, span, p) with an ID or Name
	doc.Find("a[name], a[id], div[id], span[id], p[id]").Each(func(i int, sel *goquery.Selection) {
		id, exists := sel.Attr("name")
		if !exists {
			id, exists = sel.Attr("id")
		}

		if exists && id != "" {
			// Append the anchor marker as text.
			// We prepend a newline to ensure it sits on its own line if possible.
			// Format must match parser.extractSectionByAnchor expectations: [ANCHOR:id]
			marker := fmt.Sprintf("\n[ANCHOR:%s]\n", id)

			// Prepend to the selection so it acts as a landmark before the content
			sel.BeforeHtml(marker)
		}
	})
}

// hasFontSize checks if style contains font-size >= minPt
func (s *HTMLSanitizer) hasFontSize(style string, minPt int) bool {
	// Match font-size: Xpt or font-size:Xpt
	re := regexp.MustCompile(`font-size:\s*(\d+)(?:\.?\d*)pt`)
	matches := re.FindStringSubmatch(style)
	if len(matches) >= 2 {
		var size int
		fmt.Sscanf(matches[1], "%d", &size)
		return size >= minPt
	}
	return false
}

// looksLikeSectionHeader checks if text matches SEC section naming patterns
func (s *HTMLSanitizer) looksLikeSectionHeader(text string) bool {
	patterns := []string{
		`(?i)^Item\s+\d`,
		`(?i)^PART\s+[IVX]+`,
		`(?i)^Note\s+\d`,
		`(?i)^CONSOLIDATED\s+`,
		`(?i)^FINANCIAL\s+STATEMENTS`,
		`(?i)^BALANCE\s+SHEET`,
		`(?i)^STATEMENTS?\s+OF`,
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, text); matched {
			return true
		}
	}
	return false
}

// convertToHeader changes a selection's tag name to the specified header tag
func (s *HTMLSanitizer) convertToHeader(sel *goquery.Selection, tag string) {
	// Get inner HTML
	html, _ := sel.Html()
	// Create new element
	sel.ReplaceWithHtml(fmt.Sprintf("<%s>%s</%s>", tag, html, tag))
}

// ExtractTablesWithPlaceholders replaces <table> elements with {{TABLE_ID_N}} placeholders.
// This allows Pandoc to handle text-only content while tables are processed separately.
func (s *HTMLSanitizer) ExtractTablesWithPlaceholders(doc *goquery.Document) {
	doc.Find("table").Each(func(i int, sel *goquery.Selection) {
		// Get the full table HTML
		tableHTML, err := goquery.OuterHtml(sel)
		if err != nil {
			return
		}

		// Generate placeholder ID
		s.tableCount++
		placeholderID := fmt.Sprintf("{{TABLE_ID_%d}}", s.tableCount)

		// Store the table HTML
		s.tableStore[placeholderID] = tableHTML

		// Replace table with placeholder
		sel.ReplaceWithHtml(fmt.Sprintf("\n%s\n", placeholderID))
	})
}

// RestoreTables replaces {{TABLE_ID_N}} placeholders with converted Markdown tables.
// Uses the existing TableConverter (Virtual Grid) for colspan/rowspan handling.
func (s *HTMLSanitizer) RestoreTables(markdown string) string {
	converter := &TableConverter{}

	for placeholderID, tableHTML := range s.tableStore {
		// Convert HTML table to Markdown using Virtual Grid
		markdownTable := converter.ConvertTableToMarkdown(tableHTML)

		// Replace placeholder with converted table
		markdown = strings.Replace(markdown, placeholderID, markdownTable, 1)
	}

	return markdown
}

// RemoveNoise strips elements that add no value for financial extraction.
func (s *HTMLSanitizer) RemoveNoise(doc *goquery.Document) {
	// Remove script and style tags
	doc.Find("script, style").Remove()

	// Remove hidden elements
	doc.Find("[hidden], [style*='display:none'], [style*='display: none']").Remove()

	// Remove transparent/spacer images (often 1x1 pixels)
	doc.Find("img").Each(func(i int, sel *goquery.Selection) {
		src, _ := sel.Attr("src")
		alt, _ := sel.Attr("alt")
		width, _ := sel.Attr("width")
		height, _ := sel.Attr("height")

		// Remove if it looks like a spacer image
		if src == "" || strings.Contains(src, "spacer") || strings.Contains(src, "blank") {
			sel.Remove()
			return
		}
		// Remove tiny images (likely spacers)
		if width == "1" || height == "1" {
			sel.Remove()
			return
		}
		// Remove images with no alt text and no meaningful src
		if alt == "" && !strings.Contains(src, "logo") {
			sel.Remove()
		}
	})

	// Remove page number footers (common pattern in SEC filings)
	doc.Find("p, div, span").Each(func(i int, sel *goquery.Selection) {
		text := strings.TrimSpace(sel.Text())
		// Match patterns like "Page 1", "- 1 -", "F-1", etc.
		if matched, _ := regexp.MatchString(`^(?:Page\s*)?\d+$|^-\s*\d+\s*-$|^[A-Z]?-\d+$`, text); matched {
			// Only remove if it's short (likely just a page number)
			if len(text) < 20 {
				sel.Remove()
			}
		}
	})

	// Remove XBRL inline tags but preserve their text content
	doc.Find("ix\\:nonFraction, ix\\:nonNumeric, ix\\:fraction").Each(func(i int, sel *goquery.Selection) {
		text := sel.Text()
		sel.ReplaceWithHtml(text)
	})

	// Remove empty elements that create noise in markdown
	// BUT preserve elements with ID/Name as they might be anchors
	doc.Find("p, div, span").Each(func(i int, sel *goquery.Selection) {
		if strings.TrimSpace(sel.Text()) == "" && sel.Children().Length() == 0 {
			// Check for ID or Name before removing
			if _, hasID := sel.Attr("id"); hasID {
				return
			}
			if _, hasName := sel.Attr("name"); hasName {
				return
			}
			sel.Remove()
		}
	})
}

// GetTableCount returns the number of tables extracted
func (s *HTMLSanitizer) GetTableCount() int {
	return s.tableCount
}
