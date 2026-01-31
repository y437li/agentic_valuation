// Package ingest provides 10-K section parsing for incremental agent processing.
// This parser splits 10-K filings by Item number (1, 1A, 1B, ... 15).
package ingest

import (
	"regexp"
	"strings"
)

// =============================================================================
// 10-K SECTION DEFINITIONS
// Based on SEC Form 10-K structure
// =============================================================================

// Section represents a parsed section from a 10-K filing.
type Section struct {
	ItemNumber  string `json:"item_number"`  // "1", "1A", "7", "8", etc.
	Title       string `json:"title"`        // "Business", "Risk Factors", etc.
	Content     string `json:"content"`      // Raw text content
	StartOffset int    `json:"start_offset"` // Character offset in source
	EndOffset   int    `json:"end_offset"`
	Priority    int    `json:"priority"` // 1 = highest (Item 8, 7)
}

// SectionDefinition defines expected sections in a 10-K.
var SectionDefinitions = []struct {
	ItemNumber string
	Title      string
	Priority   int
}{
	{"1", "Business", 3},
	{"1A", "Risk Factors", 3},
	{"1B", "Unresolved Staff Comments", 5},
	{"1C", "Cybersecurity", 4},
	{"2", "Properties", 5},
	{"3", "Legal Proceedings", 4},
	{"4", "Mine Safety Disclosures", 5},
	{"5", "Market for Common Equity", 3},
	{"6", "Selected Financial Data", 5}, // Discontinued Feb 2021
	{"7", "MD&A", 1},                    // HIGH PRIORITY
	{"7A", "Market Risk", 4},
	{"8", "Financial Statements", 1}, // HIGHEST PRIORITY
	{"9", "Accounting Disagreements", 5},
	{"9A", "Controls and Procedures", 4},
	{"9B", "Other Information", 5},
	{"10", "Directors and Governance", 5},
	{"11", "Executive Compensation", 5},
	{"12", "Security Ownership", 5},
	{"13", "Related Transactions", 4},
	{"14", "Accountant Fees", 5},
	{"15", "Exhibits and Schedules", 5},
}

// =============================================================================
// PARSER
// =============================================================================

// TenKParser parses 10-K filings into sections.
type TenKParser struct {
	patterns []*regexp.Regexp
}

// NewTenKParser creates a new 10-K section parser.
func NewTenKParser() *TenKParser {
	patterns := make([]*regexp.Regexp, 0)

	// Build regex patterns for each item
	// Matches variations like:
	//   "ITEM 1. BUSINESS"
	//   "Item 1 - Business"
	//   "Item 1A. Risk Factors"
	//   "<a name="item1">"
	for _, def := range SectionDefinitions {
		// Pattern for text-based matching
		itemNum := def.ItemNumber
		patternStr := `(?i)(?:^|\n)\s*(?:item|ITEM)\s*` + regexp.QuoteMeta(itemNum) + `\s*[.\-:]\s*` + regexp.QuoteMeta(def.Title)
		pattern := regexp.MustCompile(patternStr)
		patterns = append(patterns, pattern)
	}

	return &TenKParser{patterns: patterns}
}

// ParseSections extracts all sections from 10-K content.
//
// Input can be HTML or plain text (HTML tags will be stripped for matching).
func (p *TenKParser) ParseSections(content string) []Section {
	sections := make([]Section, 0)

	// Find all section boundaries
	type boundary struct {
		itemNum  string
		title    string
		offset   int
		priority int
	}

	boundaries := make([]boundary, 0)

	// Search for each section pattern
	for i, pattern := range p.patterns {
		matches := pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			boundaries = append(boundaries, boundary{
				itemNum:  SectionDefinitions[i].ItemNumber,
				title:    SectionDefinitions[i].Title,
				offset:   match[0],
				priority: SectionDefinitions[i].Priority,
			})
		}
	}

	// Sort boundaries by offset
	for i := 0; i < len(boundaries)-1; i++ {
		for j := i + 1; j < len(boundaries); j++ {
			if boundaries[j].offset < boundaries[i].offset {
				boundaries[i], boundaries[j] = boundaries[j], boundaries[i]
			}
		}
	}

	// Extract content between boundaries
	for i, b := range boundaries {
		endOffset := len(content)
		if i+1 < len(boundaries) {
			endOffset = boundaries[i+1].offset
		}

		sectionContent := content[b.offset:endOffset]
		sectionContent = cleanContent(sectionContent)

		sections = append(sections, Section{
			ItemNumber:  b.itemNum,
			Title:       b.title,
			Content:     sectionContent,
			StartOffset: b.offset,
			EndOffset:   endOffset,
			Priority:    b.priority,
		})
	}

	return sections
}

// GetSectionByItem returns a specific section by item number.
func (p *TenKParser) GetSectionByItem(content string, itemNumber string) *Section {
	sections := p.ParseSections(content)
	for _, s := range sections {
		if strings.EqualFold(s.ItemNumber, itemNumber) {
			return &s
		}
	}
	return nil
}

// GetPrioritySections returns sections sorted by priority (highest first).
func (p *TenKParser) GetPrioritySections(content string) []Section {
	sections := p.ParseSections(content)

	// Sort by priority (ascending = higher priority first)
	for i := 0; i < len(sections)-1; i++ {
		for j := i + 1; j < len(sections); j++ {
			if sections[j].Priority < sections[i].Priority {
				sections[i], sections[j] = sections[j], sections[i]
			}
		}
	}

	return sections
}

// =============================================================================
// HELPERS
// =============================================================================

// cleanContent removes excessive whitespace and basic HTML tags.
func cleanContent(content string) string {
	// Remove HTML tags (basic)
	htmlTag := regexp.MustCompile(`<[^>]*>`)
	content = htmlTag.ReplaceAllString(content, "")

	// Normalize whitespace
	whitespace := regexp.MustCompile(`\s+`)
	content = whitespace.ReplaceAllString(content, " ")

	// Trim
	content = strings.TrimSpace(content)

	return content
}

// EstimateTokens provides rough token count for content.
// Rule of thumb: ~4 characters per token for English.
func EstimateTokens(content string) int {
	return len(content) / 4
}

// ShouldChunk returns true if section exceeds recommended size.
// Recommended max: 8000 tokens (~32000 chars) for LLM context.
func ShouldChunk(section Section) bool {
	return EstimateTokens(section.Content) > 8000
}

// ChunkSection splits large sections into smaller chunks.
func ChunkSection(section Section, maxTokens int) []Section {
	if EstimateTokens(section.Content) <= maxTokens {
		return []Section{section}
	}

	chunks := make([]Section, 0)
	maxChars := maxTokens * 4
	content := section.Content

	for i := 0; len(content) > 0; i++ {
		end := maxChars
		if end > len(content) {
			end = len(content)
		}

		chunks = append(chunks, Section{
			ItemNumber: section.ItemNumber,
			Title:      section.Title + " (Part " + string(rune('A'+i)) + ")",
			Content:    content[:end],
			Priority:   section.Priority,
		})

		content = content[end:]
	}

	return chunks
}
