// Package fee provides the Financial Extraction Engine for deterministic SEC filing parsing.
package fee

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DocumentFinder locates the correct financial statement document in a SEC filing.
// It handles cases where Item 8 redirects to exhibits or financial data is in separate files.
type DocumentFinder struct {
	// XBRL tags that indicate a financial statement document
	standardXBRLTags []string

	// Fallback keywords for document detection
	fallbackKeywords []string
}

// FilingDocument represents a document within a SEC filing
type FilingDocument struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Type        string  `json:"type"`
	Size        int     `json:"size"`
	URL         string  `json:"url"`
	Sequence    string  `json:"sequence"`
	XBRLScore   float64 `json:"-"` // Internal scoring for XBRL tag matches
}

// NewDocumentFinder creates a new Document Finder with standard configurations.
func NewDocumentFinder() *DocumentFinder {
	return &DocumentFinder{
		// Core XBRL tags that must be present in financial statements
		standardXBRLTags: []string{
			"us-gaap:Assets",
			"us-gaap:Liabilities",
			"us-gaap:StockholdersEquity",
			"us-gaap:LiabilitiesAndStockholdersEquity",
			"us-gaap:CashAndCashEquivalentsAtCarryingValue",
			"us-gaap:AccountsReceivableNetCurrent",
			"us-gaap:PropertyPlantAndEquipmentNet",
			"us-gaap:Revenues",
			"us-gaap:NetIncomeLoss",
		},

		// Fallback keywords for document description/name matching
		fallbackKeywords: []string{
			"financial statements",
			"consolidated balance",
			"balance sheet",
			"income statement",
			"statement of operations",
			"cash flow",
			"10-k",
			"form 10-k",
		},
	}
}

// FindFinancialDocument finds the document containing financial statements.
// It uses a multi-strategy approach:
// 1. Primary: XBRL tag detection
// 2. Fallback: Document description/name keywords
// 3. Last resort: Largest HTML file
func (df *DocumentFinder) FindFinancialDocument(documents []FilingDocument, htmlContents map[string]string) (*FilingDocument, error) {
	if len(documents) == 0 {
		return nil, fmt.Errorf("no documents provided")
	}

	// Strategy 1: XBRL tag scoring
	scoredDocs := df.scoreByXBRL(documents, htmlContents)
	if len(scoredDocs) > 0 && scoredDocs[0].XBRLScore >= 3 {
		// At least 3 XBRL tags found - high confidence
		return &scoredDocs[0], nil
	}

	// Strategy 2: Fallback to keywords
	keywordDoc := df.findByKeywords(documents)
	if keywordDoc != nil {
		return keywordDoc, nil
	}

	// Strategy 3: Largest HTML file (last resort)
	largestDoc := df.findLargestHTML(documents)
	if largestDoc != nil {
		return largestDoc, nil
	}

	// Return first document if all else fails
	return &documents[0], nil
}

// scoreByXBRL scores documents by the number of standard XBRL tags they contain.
func (df *DocumentFinder) scoreByXBRL(documents []FilingDocument, htmlContents map[string]string) []FilingDocument {
	scored := make([]FilingDocument, 0)

	for _, doc := range documents {
		// Only score HTML documents
		if !strings.HasSuffix(strings.ToLower(doc.Name), ".htm") &&
			!strings.HasSuffix(strings.ToLower(doc.Name), ".html") {
			continue
		}

		html, exists := htmlContents[doc.Name]
		if !exists || html == "" {
			continue
		}

		// Count XBRL tag matches
		score := 0.0
		for _, tag := range df.standardXBRLTags {
			// Check for both name="tag" and just the tag name in content
			if strings.Contains(html, tag) ||
				strings.Contains(html, strings.Replace(tag, ":", "_", 1)) {
				score += 1.0
			}
		}

		if score > 0 {
			scoredDoc := doc
			scoredDoc.XBRLScore = score
			scored = append(scored, scoredDoc)
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].XBRLScore > scored[j].XBRLScore
	})

	return scored
}

// findByKeywords finds documents matching fallback keywords.
func (df *DocumentFinder) findByKeywords(documents []FilingDocument) *FilingDocument {
	for _, doc := range documents {
		// Check document type first (most reliable)
		docType := strings.ToLower(doc.Type)
		if docType == "10-k" || docType == "form 10-k" {
			return &doc
		}

		// Check description
		desc := strings.ToLower(doc.Description)
		for _, keyword := range df.fallbackKeywords {
			if strings.Contains(desc, keyword) {
				return &doc
			}
		}

		// Check filename
		name := strings.ToLower(doc.Name)
		for _, keyword := range df.fallbackKeywords {
			// Simplified keyword for filename matching
			simplified := strings.ReplaceAll(keyword, " ", "")
			if strings.Contains(name, simplified) {
				return &doc
			}
		}
	}

	return nil
}

// findLargestHTML finds the largest HTML document (usually the main filing).
func (df *DocumentFinder) findLargestHTML(documents []FilingDocument) *FilingDocument {
	var largest *FilingDocument
	maxSize := 0

	for i := range documents {
		doc := &documents[i]

		// Only consider HTML files
		name := strings.ToLower(doc.Name)
		if !strings.HasSuffix(name, ".htm") && !strings.HasSuffix(name, ".html") {
			continue
		}

		if doc.Size > maxSize {
			maxSize = doc.Size
			largest = doc
		}
	}

	return largest
}

// DetectItem8Redirect checks if Item 8 content is a redirect to another section.
// Returns true if the content is just a pointer to exhibits or other files.
func (df *DocumentFinder) DetectItem8Redirect(item8Content string) bool {
	if len(item8Content) == 0 {
		return true
	}

	// If content is very short, likely a redirect
	if len(item8Content) < 2000 {
		return true
	}

	// Check for redirect patterns
	redirectPatterns := []string{
		`(?i)see\s+(the\s+)?consolidated\s+financial\s+statements`,
		`(?i)incorporated\s+(herein\s+)?by\s+reference`,
		`(?i)refer\s+to\s+exhibit`,
		`(?i)financial\s+statements\s+.*\s+are\s+(included|set\s+forth)`,
	}

	for _, pattern := range redirectPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(item8Content[:min(1000, len(item8Content))]) {
			return true
		}
	}

	// Check if actual financial tables exist
	tablePatterns := []string{
		`(?i)total\s+assets`,
		`(?i)total\s+liabilities`,
		`(?i)stockholders.*equity`,
	}

	tableCount := 0
	for _, pattern := range tablePatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(item8Content) {
			tableCount++
		}
	}

	// If we don't find at least 2 financial table markers, consider it a redirect
	return tableCount < 2
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
