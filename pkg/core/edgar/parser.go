// Package edgar provides functionality for fetching and parsing SEC EDGAR filings.
//
// This package uses the following external libraries:
//   - github.com/JohannesKaufmann/html-to-markdown: Converts HTML to Markdown for LLM processing
//   - github.com/PuerkitoBio/goquery: jQuery-style HTML traversal and manipulation for cleaning
package edgar

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	userAgent         = "TIED Platform info@tied.com"
	submissionsAPIURL = "https://data.sec.gov/submissions/CIK%s.json"
	filingBaseURL     = "https://www.sec.gov/Archives/edgar/data/%s/%s/%s"
	companyTickersURL = "https://www.sec.gov/files/company_tickers.json"
)

// Parser handles SEC EDGAR 10-K parsing
type Parser struct {
	client      *http.Client
	tickerCache map[string]string // Ticker -> CIK (padded)
	tickerMutex sync.RWMutex
}

// NewParser creates a new EDGAR parser
func NewParser() *Parser {
	return &Parser{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// SubmissionsResponse from SEC API
type SubmissionsResponse struct {
	CIK     string   `json:"cik"`
	Name    string   `json:"name"`
	Tickers []string `json:"tickers"`
	Filings Filings  `json:"filings"`
}

// Filings contains filing information
type Filings struct {
	Recent RecentFilings `json:"recent"`
}

// RecentFilings contains recent filing arrays
type RecentFilings struct {
	AccessionNumber []string `json:"accessionNumber"`
	FilingDate      []string `json:"filingDate"`
	Form            []string `json:"form"`
	PrimaryDocument []string `json:"primaryDocument"`
}

// LookupCIK resolves a ticker symbol to a CIK using SEC's company_tickers.json
func (p *Parser) LookupCIK(ticker string) (string, error) {
	normalizedTicker := strings.ToUpper(strings.TrimSpace(ticker))

	p.tickerMutex.Lock()
	defer p.tickerMutex.Unlock()

	// Initialize cache if needed
	if p.tickerCache == nil {
		p.tickerCache = make(map[string]string)
	}

	// 1. Check Cache
	if cik, ok := p.tickerCache[normalizedTicker]; ok {
		return cik, nil
	}

	// 2. Fetch if cache empty (lazy load)
	if len(p.tickerCache) == 0 {
		if err := p.loadTickerCache(); err != nil {
			return "", err
		}
		// Retry check
		if cik, ok := p.tickerCache[normalizedTicker]; ok {
			return cik, nil
		}
	}

	return "", fmt.Errorf("ticker %s not found in SEC database", ticker)
}

// GetFilingMetadata fetches filing metadata for a company (returns most recent)
func (p *Parser) GetFilingMetadata(cik string, form string) (*FilingMetadata, error) {
	return p.GetFilingMetadataByYear(cik, form, 0) // 0 = most recent
}

// loadTickerCache fetches the full ticker list from SEC
// Format: {"0": {"cik_str": 123, "ticker": "AAPL", "title": "Apple"}, ...}
// or sometimes array. The official schema is object with numeric keys.
func (p *Parser) loadTickerCache() error {
	fmt.Println("Loading Ticker->CIK map from SEC...")
	body, err := p.fetchURL(companyTickersURL)
	if err != nil {
		return fmt.Errorf("failed to fetch company tickers: %w", err)
	}

	// Define struct for the specific JSON format
	type TickerEntry struct {
		CIK    int    `json:"cik_str"`
		Ticker string `json:"ticker"`
		Title  string `json:"title"`
	}

	// The JSON is a map of string keys to TickerEntry
	var resp map[string]TickerEntry
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse ticker JSON: %w", err)
	}

	for _, entry := range resp {
		// Pad CIK to 10 digits
		cikStr := fmt.Sprintf("%010d", entry.CIK)
		p.tickerCache[strings.ToUpper(entry.Ticker)] = cikStr
	}

	fmt.Printf("Loaded %d tickers from SEC.\n", len(p.tickerCache))
	return nil
}

// GetFilingMetadataByYear fetches filing metadata for a specific fiscal year
// fiscalYear=0 returns the most recent filing
// For 10-K: fiscalYear 2023 will find the 10-K filed in early 2024 covering FY2023
// GetFilingMetadataByYear fetches filing metadata for a specific fiscal year
// fiscalYear=0 returns the most recent filing.
// Supports both 10-K and 10-KA (amendment). Prioritizes the latest valid filing (by date) for that year.
func (p *Parser) GetFilingMetadataByYear(cik string, form string, fiscalYear int) (*FilingMetadata, error) {
	// Pad CIK to 10 digits
	cik = padCIK(cik)

	url := fmt.Sprintf(submissionsAPIURL, cik)
	body, err := p.fetchURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch submissions: %w", err)
	}

	var resp SubmissionsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse submissions JSON: %w", err)
	}

	// We want to find the *best* filing for the requested fiscal year.
	// Best = "10-KA" (latest) or "10-K" (latest) if no amendment.
	// We iterate through ALL filings and track the best match.
	var bestCandidate *FilingMetadata
	var bestDate string

	for i, f := range resp.Filings.Recent.Form {
		// Only check matches: form="10-K" matches "10-K" or "10-KA"
		isMatch := false
		if form == "10-K" {
			if f == "10-K" || f == "10-KA" || f == "10-K/A" {
				isMatch = true
			}
		} else {
			if f == form {
				isMatch = true
			}
		}

		if !isMatch {
			continue
		}

		// Parse metadata for this entry
		accession := resp.Filings.Recent.AccessionNumber[i]
		primaryDoc := resp.Filings.Recent.PrimaryDocument[i]
		filingDate := resp.Filings.Recent.FilingDate[i]

		// Calculate Fiscal Year from the filing info
		// Note: extractFiscalYear is a rough heuristic.
		// A better approach for 10-K is often Date - 1 year if filed Jan-Mar.
		fileFiscalYear := extractFiscalYear(primaryDoc, filingDate)

		// 1. If searching for specific year -> Match exactly
		if fiscalYear > 0 {
			if fileFiscalYear != fiscalYear {
				continue
			}
		} else {
			// 2. If fiscalYear=0 (Latest), we want the absolute latest filing date
			// But simpler logic: "Latest" usually means "Latest 10-K/A on record"
			// So we just rely on date comparison below.
		}

		// Found a candidate for this fiscal year.
		// Is it newer than our current best?
		if bestCandidate == nil || filingDate > bestDate {
			accessionNoDashes := strings.ReplaceAll(accession, "-", "")
			filingURL := fmt.Sprintf(filingBaseURL, cik, accessionNoDashes, primaryDoc)

			bestCandidate = &FilingMetadata{
				CIK:             cik,
				CompanyName:     resp.Name,
				Tickers:         resp.Tickers,
				AccessionNumber: accession,
				FilingDate:      filingDate,
				Form:            f, // Store actual form (10-K or 10-KA)
				FiscalYear:      fileFiscalYear,
				FiscalPeriod:    determineFiscalPeriod(form),
				PrimaryDocument: primaryDoc,
				FilingURL:       filingURL,
				ParsedAt:        time.Now(),
			}
			bestDate = filingDate
		}
	}

	if bestCandidate != nil {
		return bestCandidate, nil
	}

	if fiscalYear > 0 {
		return nil, fmt.Errorf("no %s (or amendment) filing found for CIK %s fiscal year %d", form, cik, fiscalYear)
	}
	return nil, fmt.Errorf("no %s filing found for CIK %s", form, cik)
}

// FetchFilingHTML fetches the HTML content of a filing
func (p *Parser) FetchFilingHTML(filingURL string) (string, error) {
	body, err := p.fetchURL(filingURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch filing HTML: %w", err)
	}
	return string(body), nil
}

// FetchSmartFilingHTML fetches the correct financial statement HTML using Smart Document Finder.
// STRATEGY: Handles two formats:
// 1. iXBRL format (R*.htm files): Merge Balance Sheet, Income Statement, Cash Flow files
// 2. Traditional format: Select best single document
func (p *Parser) FetchSmartFilingHTML(meta *FilingMetadata) (string, error) {
	// Step 1: Try to get Filing Index to discover all documents
	indexURL := buildFilingIndexURL(meta.CIK, meta.AccessionNumber)
	indexBody, err := p.fetchURL(indexURL)
	if err != nil {
		return p.FetchFilingHTML(meta.FilingURL)
	}

	// Step 2: Parse filing index
	documents, err := parseFilingIndex(indexBody, meta.CIK, meta.AccessionNumber)
	if err != nil || len(documents) == 0 {
		return p.FetchFilingHTML(meta.FilingURL)
	}

	// Step 3: Detect iXBRL format (R1.htm, R2.htm, etc.)
	var rFiles []FilingIndexDocument
	for _, doc := range documents {
		if matched, _ := regexp.MatchString(`^R\d+\.htm$`, doc.Name); matched {
			rFiles = append(rFiles, doc)
		}
	}

	// If iXBRL format detected (>5 R files), find the main document
	if len(rFiles) > 5 {
		// STRATEGY A: Find main document (largest .htm file that isn't R*.htm or exhibit)
		// Main documents are typically named like: aapl-20240928.htm, msft-20240630.htm
		var mainDoc *FilingIndexDocument
		var maxSize int

		for i := range documents {
			doc := &documents[i]
			nameLower := strings.ToLower(doc.Name)

			// Skip non-HTML files
			if !strings.HasSuffix(nameLower, ".htm") && !strings.HasSuffix(nameLower, ".html") {
				continue
			}

			// Skip R*.htm files (XBRL viewer files)
			if matched, _ := regexp.MatchString(`(?i)^r\d+\.htm$`, doc.Name); matched {
				continue
			}

			// Skip index files
			if strings.Contains(nameLower, "index") {
				continue
			}

			// Skip exhibit files (typically have "exhibit" in name or start with "ex")
			if strings.Contains(nameLower, "exhibit") || strings.HasPrefix(nameLower, "ex") {
				continue
			}

			// Skip small files (< 100KB) - main docs are usually > 500KB
			if doc.Size < 100000 {
				continue
			}

			// Find the largest qualifying file
			if doc.Size > maxSize {
				maxSize = doc.Size
				mainDoc = doc
			}
		}

		if mainDoc != nil && mainDoc.Size > 500000 { // Main doc should be > 500KB
			content, err := p.FetchFilingHTML(mainDoc.URL)
			if err == nil && len(content) > 10000 {
				return content, nil
			}
		}

		// STRATEGY B: Fallback to FilingSummary.xml
		summary, err := p.fetchAndParseFilingSummary(meta.CIK, meta.AccessionNumber)
		if err == nil && summary != nil && len(summary.MyReports.Reports) > 0 {
			mergedContent, mergeErr := p.fetchAndMergeUsingSummary(summary, meta.CIK, meta.AccessionNumber)
			if mergeErr == nil && len(mergedContent) > 500 {
				return mergedContent, nil
			}
		}

		// STRATEGY C: Fallback to primary document
		return p.FetchFilingHTML(meta.FilingURL)
	}

	// Traditional format: Download and score all HTML files
	htmlContents := make(map[string]string)
	for _, doc := range documents {
		nameLower := strings.ToLower(doc.Name)
		if !strings.HasSuffix(nameLower, ".htm") && !strings.HasSuffix(nameLower, ".html") {
			continue
		}
		if strings.Contains(nameLower, "index") || strings.Contains(nameLower, "cover") {
			continue
		}

		content, fetchErr := p.FetchFilingHTML(doc.URL)
		if fetchErr == nil && len(content) > 0 {
			htmlContents[doc.Name] = content
		}
	}

	if len(htmlContents) == 0 {
		return p.FetchFilingHTML(meta.FilingURL)
	}

	finder := NewSmartDocumentFinder()
	bestDoc := finder.FindBestDocument(documents, htmlContents)

	if bestDoc != nil {
		if content, exists := htmlContents[bestDoc.Name]; exists {
			return content, nil
		}
	}

	// Fallback to primary document
	return p.FetchFilingHTML(meta.FilingURL)
}

// fetchAndMergeIXBRLFiles fetches key R*.htm files and merges them for extraction.
// Targets: R3 (Income), R5 (Balance), R7/R8 (Cash Flow) based on common iXBRL patterns.
func (p *Parser) fetchAndMergeIXBRLFiles(rFiles []FilingIndexDocument) (string, error) {
	// Priority files to fetch (common iXBRL numbering for financial statements)
	priorityNumbers := []int{3, 4, 5, 6, 7, 8} // Income, Balance, Cash Flow typically in R3-R8

	var merged strings.Builder
	merged.WriteString("<html><body>\n")

	for _, priority := range priorityNumbers {
		targetName := fmt.Sprintf("R%d.htm", priority)
		for _, doc := range rFiles {
			if doc.Name == targetName {
				content, err := p.FetchFilingHTML(doc.URL)
				if err == nil && len(content) > 0 {
					// Strip HTML/HEAD/BODY wrappers for merging
					content = extractBodyContent(content)
					merged.WriteString(fmt.Sprintf("\n<!-- === %s === -->\n", targetName))
					merged.WriteString(content)
					merged.WriteString("\n")
				}
				break
			}
		}
	}

	merged.WriteString("</body></html>")
	return merged.String(), nil
}

// fetchAndMergeUsingSummary uses the parsed FilingSummary to find and merge the correct financial statements.
func (p *Parser) fetchAndMergeUsingSummary(summary *FilingSummary, cik, accession string) (string, error) {
	// Identify key files based on ShortName/LongName
	targetFiles := make(map[string]bool)
	var orderedFiles []string

	priorityPatterns := []struct {
		Pattern  string
		Category string
	}{
		{`(?i)balance\s*sheet|financial\s*position`, "BalanceSheet"},
		{`(?i)income\s*statement|statement\s*of\s*earnings|statement\s*of\s*operations`, "IncomeStatement"},
		{`(?i)cash\s*flow`, "CashFlow"},
		{`(?i)stockholder|shareholder|equity`, "StockholdersEquity"},
	}

	// Sort reports by Position to ensure logical order (or just use Pattern priority)
	// We'll iterate patterns and find matches in the summary
	for _, pat := range priorityPatterns {
		re := regexp.MustCompile(pat.Pattern)
		for _, report := range summary.MyReports.Reports {
			if report.HtmlFileName == "" {
				continue
			}
			// Skip parentheticals
			if strings.Contains(strings.ToLower(report.ShortName), "parenthetical") || strings.Contains(strings.ToLower(report.LongName), "parenthetical") {
				continue
			}

			if re.MatchString(report.ShortName) || re.MatchString(report.LongName) {
				if !targetFiles[report.HtmlFileName] {
					targetFiles[report.HtmlFileName] = true
					orderedFiles = append(orderedFiles, report.HtmlFileName)
				}
			}
		}
	}

	if len(orderedFiles) == 0 {
		return "", fmt.Errorf("no financial statements found in FilingSummary")
	}

	var merged strings.Builder
	merged.WriteString("<html><body>\n")

	// Base URL for fetching reports
	// URL format: https://www.sec.gov/Archives/edgar/data/{cik}/{accession_no_dashes}/{HtmlFileName}
	accessionNoDashes := strings.ReplaceAll(accession, "-", "")
	baseURL := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s", cik, accessionNoDashes)

	for _, fileName := range orderedFiles {
		fileURL := fmt.Sprintf("%s/%s", baseURL, fileName)
		content, err := p.FetchFilingHTML(fileURL)
		if err == nil && len(content) > 0 {
			content = extractBodyContent(content)
			merged.WriteString(fmt.Sprintf("\n<!-- === %s === -->\n", fileName))
			merged.WriteString(content)
			merged.WriteString("\n")
		}
	}

	merged.WriteString("</body></html>")
	return merged.String(), nil
}

// FilingSummary represents the FilingSummary.xml structure
type FilingSummary struct {
	MyReports MyReports `xml:"MyReports"`
}

type MyReports struct {
	Reports []Report `xml:"Report"`
}

type Report struct {
	ShortName    string `xml:"ShortName"`
	LongName     string `xml:"LongName"`
	HtmlFileName string `xml:"HtmlFileName"`
	MenuCategory string `xml:"MenuCategory"`
	Position     string `xml:"Position"`
}

// fetchAndParseFilingSummary fetches and parses the FilingSummary.xml
func (p *Parser) fetchAndParseFilingSummary(cik, accession string) (*FilingSummary, error) {
	accessionNoDashes := strings.ReplaceAll(accession, "-", "")
	url := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/FilingSummary.xml", cik, accessionNoDashes)

	content, err := p.fetchURL(url)
	if err != nil {
		return nil, err
	}

	var summary FilingSummary
	if err := xml.Unmarshal(content, &summary); err != nil {
		return nil, err
	}

	return &summary, nil
}

// extractBodyContent extracts content between <body> and </body> tags
func extractBodyContent(html string) string {
	bodyStart := strings.Index(strings.ToLower(html), "<body")
	if bodyStart == -1 {
		return html
	}
	// Find end of opening body tag
	bodyTagEnd := strings.Index(html[bodyStart:], ">")
	if bodyTagEnd == -1 {
		return html
	}
	contentStart := bodyStart + bodyTagEnd + 1

	bodyEnd := strings.LastIndex(strings.ToLower(html), "</body>")
	if bodyEnd == -1 || bodyEnd <= contentStart {
		return html[contentStart:]
	}

	return html[contentStart:bodyEnd]
}

// isItem8Redirect checks if Item 8 content is a redirect rather than actual financial statements
func isItem8Redirect(content string) bool {
	// Very short content is likely a redirect
	if len(content) < 2000 {
		return true
	}

	// Check for redirect patterns
	redirectPatterns := []string{
		`(?i)see\s+(the\s+)?consolidated\s+financial\s+statements`,
		`(?i)incorporated\s+(herein\s+)?by\s+reference`,
		`(?i)refer\s+to\s+exhibit`,
		`(?i)set\s+forth\s+on\s+pages`,
	}

	for _, pattern := range redirectPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(content[:min(1000, len(content))]) {
			return true
		}
	}

	// Check for financial table markers
	tableMarkers := []string{
		"[TABLE: BALANCE_SHEET]",
		"[TABLE: INCOME_STATEMENT]",
		"[TABLE: CASH_FLOW_STATEMENT]",
	}

	markerCount := 0
	for _, marker := range tableMarkers {
		if strings.Contains(content, marker) {
			markerCount++
		}
	}

	// Need at least 2 table markers
	return markerCount < 2
}

// buildFilingIndexURL constructs the URL for the filing index JSON
func buildFilingIndexURL(cik, accession string) string {
	accessionNoDashes := strings.ReplaceAll(accession, "-", "")
	return fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/index.json", cik, accessionNoDashes)
}

// FilingIndexDocument represents a document in the filing index
type FilingIndexDocument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Size        int    `json:"size"`
	URL         string
}

// parseFilingIndex parses the SEC filing index JSON
func parseFilingIndex(body []byte, cik, accession string) ([]FilingIndexDocument, error) {
	var index struct {
		Directory struct {
			Item []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Type        string `json:"type"`
				Size        string `json:"size"`
			} `json:"item"`
		} `json:"directory"`
	}

	if err := json.Unmarshal(body, &index); err != nil {
		return nil, fmt.Errorf("failed to parse filing index: %w", err)
	}

	accessionNoDashes := strings.ReplaceAll(accession, "-", "")
	baseURL := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/", cik, accessionNoDashes)

	documents := make([]FilingIndexDocument, 0)
	for _, item := range index.Directory.Item {
		size := 0
		fmt.Sscanf(item.Size, "%d", &size)

		documents = append(documents, FilingIndexDocument{
			Name:        item.Name,
			Description: item.Description,
			Type:        item.Type,
			Size:        size,
			URL:         baseURL + item.Name,
		})
	}

	return documents, nil
}

// SmartDocumentFinder helps locate financial statement documents
type SmartDocumentFinder struct {
	xbrlTags []string
	keywords []string
}

// NewSmartDocumentFinder creates a new document finder
func NewSmartDocumentFinder() *SmartDocumentFinder {
	return &SmartDocumentFinder{
		xbrlTags: []string{
			"us-gaap:Assets",
			"us-gaap:Liabilities",
			"us-gaap:StockholdersEquity",
			"us-gaap:Revenues",
			"us-gaap:NetIncomeLoss",
		},
		keywords: []string{
			"financial statements",
			"consolidated balance",
			"balance sheet",
		},
	}
}

// FindBestDocument finds the document most likely to contain financial statements
func (f *SmartDocumentFinder) FindBestDocument(docs []FilingIndexDocument, htmlContents map[string]string) *FilingIndexDocument {
	type scoredDoc struct {
		doc   FilingIndexDocument
		score float64
	}

	var scored []scoredDoc

	for _, doc := range docs {
		if !strings.HasSuffix(strings.ToLower(doc.Name), ".htm") &&
			!strings.HasSuffix(strings.ToLower(doc.Name), ".html") {
			continue
		}

		html, exists := htmlContents[doc.Name]
		if !exists {
			continue
		}

		score := 0.0

		// Score by XBRL tags
		for _, tag := range f.xbrlTags {
			if strings.Contains(html, tag) {
				score += 2.0
			}
		}

		// Score by keywords
		for _, keyword := range f.keywords {
			if strings.Contains(strings.ToLower(html), keyword) {
				score += 1.0
			}
		}

		// Score by file size (larger files more likely to be main document)
		score += float64(doc.Size) / 1000000.0 // Add 1 point per MB

		if score > 0 {
			scored = append(scored, scoredDoc{doc: doc, score: score})
		}
	}

	if len(scored) == 0 {
		return nil
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	return &scored[0].doc
}

// NOTE: ExtractXBRLFacts and FilterForCurrentPeriod removed as part of XBRL cleanup.
// Financial data extraction now uses LLM Navigator + Go Extractor pattern.

// ExtractNotesText extracts the "Notes to Consolidated Financial Statements" section
func (p *Parser) ExtractNotesText(html string) (text string) {
	defer func() {
		if r := recover(); r != nil {
			text = fmt.Sprintf("Error extracting notes: %v", r)
		}
	}()

	// 1. Find Item 8 start
	// Pattern covers standard Item 8 headers
	item8Pattern := regexp.MustCompile(`(?i)(Item\s*8\.?\s*[-–—]?\s*Financial\s+Statements)`)
	item8Match := item8Pattern.FindStringIndex(html)
	if item8Match == nil {
		// Try simpler fallback pattern
		item8Pattern = regexp.MustCompile(`(?i)>Item\s*8[.<]`) // e.g. <b>Item 8.</b>
		item8Match = item8Pattern.FindStringIndex(html)
	}

	if item8Match == nil {
		return ""
	}

	startPos := item8Match[0]

	// 2. Find Item 9 start (to determine end of Item 8/Notes)
	// Searching from startPos
	item9Pattern := regexp.MustCompile(`(?i)(Item\s*9\.?\s*[-–—]?\s*Changes)`)
	item9Match := item9Pattern.FindStringIndex(html[startPos:])

	var item8Html string
	if item9Match != nil {
		endPos := startPos + item9Match[0]
		item8Html = html[startPos:endPos]
	} else {
		// Fallback: take next 1MB characters if Item 9 not found clearly
		endPos := startPos + 1_000_000
		if endPos > len(html) {
			endPos = len(html)
		}
		item8Html = html[startPos:endPos]
	}

	// 3. Clean HTML to text
	return cleanHTML(item8Html)
}

// cleanHTML converts HTML to clean text for LLM using library-based approach
// Uses goquery for HTML traversal and html-to-markdown for conversion
func cleanHTML(html string) string {
	return htmlToMarkdown(html)
}

// htmlToMarkdown converts HTML to Markdown format using Pandoc.
//
// Strategy:
//  1. Clean HTML using goquery (remove scripts, XBRL tags)
//  2. Convert via Pandoc (handles all elements including complex tables)
//  3. Annotate table types for LLM identification
//
// Pandoc is the gold standard for document conversion and handles:
//   - Complex tables with colspan/rowspan
//   - Proper heading hierarchy
//   - Lists, blockquotes, code blocks
//   - Links and images
//
// REQUIREMENT: Pandoc must be installed on the system.
func htmlToMarkdown(htmlContent string) string {
	// Step 1: Pre-process with HTMLSanitizer (fixes fake headers, extracts tables, removes noise)
	sanitizer := NewHTMLSanitizer()
	cleanedHTML, err := sanitizer.Sanitize(htmlContent)
	if err != nil {
		// Fallback to legacy cleaning
		cleanedHTML = cleanHTMLWithGoquery(htmlContent)
	}

	// Step 2: Convert via Pandoc (text only - tables are placeholders)
	pandoc := NewPandocAdapter()
	if !pandoc.IsAvailable() {
		// Log warning but continue with cleaned HTML
		fmt.Println("WARNING: Pandoc not available. HTML tables may not be properly aligned.")
		// Restore tables even without Pandoc
		return sanitizer.RestoreTables(cleanedHTML)
	}

	markdown, err := pandoc.HTMLToMarkdown(cleanedHTML)
	if err != nil {
		fmt.Printf("WARNING: Pandoc conversion failed: %v\n", err)
		return sanitizer.RestoreTables(cleanedHTML)
	}

	if len(markdown) < 100 {
		// Pandoc returned insufficient content
		return sanitizer.RestoreTables(cleanedHTML)
	}

	// Step 3: Restore tables from {{TABLE_ID_N}} placeholders using Virtual Grid converter
	markdown = sanitizer.RestoreTables(markdown)

	// Step 4: Annotate table types for LLM
	markdown = annotateTableTypes(markdown)

	return strings.TrimSpace(markdown)
}

// annotateTableTypes adds [TABLE: Type] markers before financial tables
// This helps LLM identify which table is Balance Sheet, Income Statement, etc.
func annotateTableTypes(markdown string) string {
	annotations := []struct {
		patterns    []string
		tableType   string
		avoidMarker string
	}{
		{
			patterns:    []string{`(?i)Consolidated\s+Balance\s+Sheet`, `(?i)Balance\s+Sheets?`, `(?i)Statements?\s+of\s+Financial\s+Position`},
			tableType:   "[TABLE: BALANCE_SHEET]",
			avoidMarker: "Parent Company",
		},
		{
			patterns:    []string{`(?i)Consolidated\s+Statements?\s+of\s+Operations`, `(?i)Statements?\s+of\s+(Operations|Income|Earnings)`, `(?i)Income\s+Statement`},
			tableType:   "[TABLE: INCOME_STATEMENT]",
			avoidMarker: "Parent Company",
		},
		{
			patterns:    []string{`(?i)Consolidated\s+Statements?\s+of\s+Cash\s+Flows?`, `(?i)Statements?\s+of\s+Cash\s+Flows?`, `(?i)Cash\s+Flow\s+Statement`},
			tableType:   "[TABLE: CASH_FLOW_STATEMENT]",
			avoidMarker: "Parent Company",
		},
		{
			patterns:    []string{`(?i)Consolidated\s+Statements?\s+of\s+Comprehensive\s+Income`},
			tableType:   "[TABLE: COMPREHENSIVE_INCOME]",
			avoidMarker: "",
		},
		{
			patterns:    []string{`(?i)Consolidated\s+Statements?\s+of\s+Stockholders`, `(?i)Statements?\s+of\s+Changes\s+in\s+Equity`},
			tableType:   "[TABLE: STOCKHOLDERS_EQUITY]",
			avoidMarker: "",
		},
		{
			patterns:    []string{`(?i)Item\s*1\.?\s*Business`, `(?i)^Business$`},
			tableType:   "[TABLE: BUSINESS]",
			avoidMarker: "",
		},
		{
			patterns:    []string{`(?i)Item\s*1A\.?\s*Risk\s*Factors`, `(?i)^Risk\s*Factors$`},
			tableType:   "[TABLE: RISK_FACTORS]",
			avoidMarker: "",
		},
		{
			tableType:   "[TABLE: MDA]",
			avoidMarker: "",
		},
		{
			patterns:    []string{`(?i)Notes\s+to\s+(?:the\s+)?Consolidated\s+Financial\s+Statements`, `(?i)Notes\s+to\s+(?:the\s+)?Financial\s+Statements`},
			tableType:   "[TABLE: NOTES]",
			avoidMarker: "",
		},
	}

	result := markdown
	for _, ann := range annotations {
		for _, pattern := range ann.patterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindAllStringIndex(result, -1)

			// Process in reverse order to preserve indices
			for i := len(matches) - 1; i >= 0; i-- {
				matchStart := matches[i][0]

				// Check if this is a "Parent Company" section to avoid
				if ann.avoidMarker != "" {
					// Look at surrounding context (100 chars before)
					contextStart := matchStart - 100
					if contextStart < 0 {
						contextStart = 0
					}
					context := result[contextStart:matchStart]
					if strings.Contains(strings.ToLower(context), strings.ToLower(ann.avoidMarker)) {
						continue // Skip this match
					}
				}

				// Insert annotation before the table title
				result = result[:matchStart] + "\n" + ann.tableType + "\n" + result[matchStart:]
			}
		}
	}

	return result
}

// cleanHTMLWithGoquery uses goquery to clean and prepare HTML for Markdown conversion.
//
// Cleaning steps:
//   - Remove <script>, <style> tags and content
//   - Remove XBRL inline tags but preserve their text content
//   - Remove hidden elements
//   - Decode HTML entities
func cleanHTMLWithGoquery(htmlContent string) string {
	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		// Fallback to regex-based cleaning
		return fallbackClean(htmlContent)
	}

	// Remove script, style, and hidden elements
	doc.Find("script, style, [hidden], [style*='display:none'], [style*='display: none']").Remove()

	// Handle XBRL inline tags - replace with their text content
	doc.Find("ix\\:nonFraction, ix\\:nonNumeric, ix\\:fraction").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		s.ReplaceWithHtml(text)
	})

	// Get cleaned HTML
	cleanedHTML, err := doc.Find("body").Html()
	if err != nil {
		cleanedHTML, _ = doc.Html()
	}

	return cleanedHTML
}

// fallbackClean provides regex-based HTML cleaning when goquery fails
func fallbackClean(html string) string {
	// Remove XBRL tags but keep content
	reXBRL := regexp.MustCompile(`<ix:[^>]+>([^<]*)</ix:[^>]+>`)
	text := reXBRL.ReplaceAllString(html, "$1")

	// Remove script and style
	reScript := regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</\1>`)
	text = reScript.ReplaceAllString(text, "")

	return text
}

// convertTableToMarkdown converts an HTML table to Markdown using RobustTableConverter
func convertTableToMarkdown(tableHTML string) string {
	converter := &TableConverter{}
	return converter.ConvertTableToMarkdown(tableHTML)
}

// ExtractItem8Markdown extracts financial statements and converts to Markdown.
// v6: Robust Pipeline - Sanitizer -> Pandoc -> Table Restoration -> Anchor Extraction
// For companies with non-standard naming, use ExtractWithLLMAgent instead.
func (p *Parser) ExtractItem8Markdown(html string) string {
	defer func() {
		if r := recover(); r != nil {
			// Silently handle panic
		}
	}()

	var fullMarkdown string
	var err error

	// Step 1: Sanitize HTML (Critical for iXBRL noise removal and table preservation)
	sanitizer := NewHTMLSanitizer()
	cleanHTML, err := sanitizer.Sanitize(html)
	if err == nil {
		// Step 2: Convert cleaned HTML to Markdown using Pandoc
		adapter := &PandocAdapter{Timeout: 60 * time.Second}
		fullMarkdown, err = adapter.HTMLToMarkdown(cleanHTML)

		// Step 3: Restore tables using the robust Virtual Grid converter
		if err == nil && len(fullMarkdown) > 0 {
			fullMarkdown = sanitizer.RestoreTables(fullMarkdown)
			// Step 3.5: Clean Pandoc attributes
			fullMarkdown = removePandocAttributes(fullMarkdown)
		}
	}

	// Fallback: If improved pipeline fails, use legacy regex-based conversion
	if err != nil || len(fullMarkdown) < 1000 {
		fullMarkdown = htmlToMarkdownFull(html)
	}

	if len(fullMarkdown) < 1000 {
		return ""
	}

	// Step 4: Find TOC entries with anchors (regex-based fallback)
	tocEntries := p.parseTOCFromMarkdown(fullMarkdown)

	// Step 5: Extract sections by anchor
	var result []string
	for _, entry := range tocEntries {
		if entry.Anchor != "" {
			content := p.extractSectionByAnchor(fullMarkdown, entry.Anchor, entry.SectionType)
			if len(content) > 500 {
				result = append(result, "[TABLE: "+entry.SectionType+"]\n"+content)
			}
		}
	}

	// Fallback: Smart Scan for Financial Statements
	// If TOC parsing failed, we look for standard keywords to locate the financial section.
	terms := []string{"consolidated balance sheets", "consolidated statements of income", "item 8. financial statements"}
	for _, term := range terms {
		idx := strings.Index(strings.ToLower(fullMarkdown), term)
		if idx != -1 {
			// Found a likely start. Return a large window starting a bit before this match.
			// limit window to 200KB to stay within reasonable context limits while capturing enough data.
			start := idx - 5000
			if start < 0 {
				start = 0
			}
			end := start + 250000
			if end > len(fullMarkdown) {
				end = len(fullMarkdown)
			}
			return fullMarkdown[start:end]
		}
	}

	// Last resort: If no keywords found, return a larger chunk of the beginning,
	// but 50KB is too small. Increase to 250KB.
	return fullMarkdown[:min(250000, len(fullMarkdown))]
}

// ExtractWithLLMAgent uses LLM to identify TOC entries (handles any naming convention)
func (p *Parser) ExtractWithLLMAgent(ctx context.Context, html string, llmAnalyzer *LLMAnalyzer) (string, error) {
	var fullMarkdown string
	var err error

	// Step 1: Sanitize HTML (Critical for iXBRL noise removal and table preservation)
	sanitizer := NewHTMLSanitizer()
	cleanHTML, err := sanitizer.Sanitize(html)
	if err == nil {
		// Step 2: Convert cleaned HTML to Markdown using Pandoc
		adapter := &PandocAdapter{Timeout: 60 * time.Second}
		fullMarkdown, err = adapter.HTMLToMarkdown(cleanHTML)

		// Step 3: Restore tables using the robust Virtual Grid converter
		if err == nil && len(fullMarkdown) > 0 {
			fullMarkdown = sanitizer.RestoreTables(fullMarkdown)
			// Step 3.5: Clean Pandoc span attributes ([text]{style...} -> text)
			// This is CRITICAL for matching titles obscured by styling
			fullMarkdown = removePandocAttributes(fullMarkdown)
		}
	}

	// Fallback: If improved pipeline fails, use legacy regex-based conversion
	if err != nil || len(fullMarkdown) < 1000 {
		fullMarkdown = htmlToMarkdownFull(html)
	}

	if len(fullMarkdown) < 1000 {
		return "", fmt.Errorf("markdown too short after conversion")
	}

	// Step 2: Extract TOC region (first ~30KB for LLM)
	// DEBUG: Save full markdown to inspect structure
	os.WriteFile("debug_last_markdown.md", []byte(fullMarkdown), 0644)

	tocRegion := fullMarkdown
	if len(tocRegion) > 30000 {
		tocRegion = tocRegion[:30000]
	}

	fmt.Printf("[DEBUG] Full Markdown Length: %d, TOC Region Length: %d\n", len(fullMarkdown), len(tocRegion))

	// Step 3: Use LLM to analyze TOC and identify statements
	tocResult, err := llmAnalyzer.AnalyzeTOC(ctx, tocRegion)
	if err != nil {
		fmt.Printf("[ERROR] LLM TOC Analysis Failed: %v\n", err)
		return "", fmt.Errorf("LLM TOC analysis failed: %w", err)
	}

	// Log the TOC result for debugging
	fmt.Printf("[DEBUG] TOC Result: BS=%v, IS=%v, CF=%v\n",
		tocResult.BalanceSheet != nil,
		tocResult.IncomeStatement != nil,
		tocResult.CashFlow != nil)
	if tocResult.BalanceSheet != nil {
		fmt.Printf("   -> BS Anchor: '%s', Title: '%s'\n", tocResult.BalanceSheet.Anchor, tocResult.BalanceSheet.Title)
	}

	// Step 4: Extract each identified section
	var result []string

	extractSection := func(item *TOCItem, sectionType string) {
		if item == nil {
			fmt.Printf("[DEBUG] Missing TOC Item for %s\n", sectionType)
			return
		}

		var content string

		// 1. ANCHOR-FIRST STRATEGY (Higher Precision)
		if item.Anchor != "" {
			// Try to find by Anchor ID first
			content = p.extractSectionByAnchor(fullMarkdown, item.Anchor, sectionType)
			if content != "" {
				fmt.Printf("[DEBUG] Found %s via Anchor: %s (Len: %d)\n", sectionType, item.Anchor, len(content))
			} else {
				fmt.Printf("[DEBUG] Failed to find %s via Anchor: %s\n", sectionType, item.Anchor)
			}
		}

		// 2. FALLBACK TO TITLE (If anchor missing or failed)
		if content == "" && item.Title != "" {
			content = p.extractSectionByTitle(fullMarkdown, item.Title, sectionType)
			if content != "" {
				fmt.Printf("[DEBUG] Found %s via Title: %s (Len: %d)\n", sectionType, item.Title, len(content))
			} else {
				fmt.Printf("[DEBUG] Failed to find %s via Title: %s\n", sectionType, item.Title)
			}
		}

		if len(content) > 500 {
			result = append(result, "[TABLE: "+sectionType+"]\n"+content)
		} else {
			fmt.Printf("[WARNING] Content too short for %s (Len: %d)\n", sectionType, len(content))
		}
	}

	extractSection(tocResult.Business, "BUSINESS")
	extractSection(tocResult.RiskFactors, "RISK_FACTORS")
	extractSection(tocResult.MarketRisk, "MARKET_RISK")             // Item 7A
	extractSection(tocResult.LegalProceedings, "LEGAL_PROCEEDINGS") // Item 3
	extractSection(tocResult.Controls, "CONTROLS")                  // Item 9A
	extractSection(tocResult.MDA, "MDA")
	extractSection(tocResult.BalanceSheet, "BALANCE_SHEET")
	extractSection(tocResult.IncomeStatement, "INCOME_STATEMENT")
	extractSection(tocResult.CashFlow, "CASH_FLOW")

	combinedResult := strings.Join(result, "\n\n---\n\n")

	// Safety check: If extracted content is suspiciously short (e.g. LLM missed most sections),
	// fallback to regex extraction or full markdown.
	if len(combinedResult) < 5000 {
		fmt.Printf("[WARNING] LLM TOC extraction too short (%d chars). Falling back to regex extraction.\n", len(combinedResult))
		return "", fmt.Errorf("llm toc extraction too short")
	}

	return combinedResult, nil

	// Fallback to first 50KB
	return fullMarkdown[:min(50000, len(fullMarkdown))], nil
}

// extractSectionByAnchor finds a section by its HTML anchor ID
// Supports multiple anchor formats:
//   - Pandoc format: []{#anchor_id}
//   - Legacy format: [ANCHOR:anchor_id]
//   - Raw text fallback: anchor_id
func (p *Parser) extractSectionByAnchor(markdown string, anchor string, sectionType string) string {
	// Clean anchor (remove # if present)
	cleanAnchor := strings.TrimPrefix(anchor, "#")
	if cleanAnchor == "" {
		return ""
	}

	var idx int = -1

	// Strategy 1: Pandoc format []{#id}
	pandocMarker := "[]{#" + cleanAnchor + "}"
	idx = strings.Index(markdown, pandocMarker)

	// Strategy 2: Legacy format [ANCHOR:id]
	if idx == -1 {
		legacyMarker := "[ANCHOR:" + cleanAnchor + "]"
		idx = strings.Index(markdown, legacyMarker)
	}

	// Strategy 3: Case-insensitive raw text search
	if idx == -1 {
		lowerMarkdown := strings.ToLower(markdown)
		lowerAnchor := strings.ToLower(cleanAnchor)
		idx = strings.Index(lowerMarkdown, lowerAnchor)
	}

	if idx == -1 {
		return ""
	}

	// Find section end
	endPos := p.findNextSectionEnd(markdown, idx)
	if endPos > idx+500 {
		return markdown[idx:endPos]
	}
	return ""
}

// extractSectionByTitle finds a section by its exact title
// v3: Explicit Unicode whitespace support (\p{Z}) for NBSP handling
func (p *Parser) extractSectionByTitle(markdown string, title string, sectionType string) string {
	// Clean the target title first
	cleanTitle := strings.TrimSpace(title)

	// Strategy 1: Exact line match
	escapedTitle := regexp.QuoteMeta(cleanTitle)
	// Allow any kind of whitespace (including \xA0)
	flexibleSpaceTitle := strings.ReplaceAll(escapedTitle, "\\ ", "[\\s\\p{Z}]+")

	// Match line start, optional markup, title, optional markup, line end
	// Note: We use [\s\p{Z}] to ensure we catch NBSP which \s might miss in some Go versions
	pattern := fmt.Sprintf(`(?i)(?:^|\n)[ \t\p{Z}]*(?:[*#])*[ \t\p{Z}]*%s[ \t\p{Z}]*(?:[*#])*[ \t\p{Z}]*(?:\n|$)`, flexibleSpaceTitle)
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringIndex(markdown, -1)

	// Strategy 2: Relaxed Heading Match
	// If strict match fails, try matching core words ignoring "Item X."
	if len(matches) == 0 {
		// Remove "Item X." prefix (handling unicode spaces)
		coreTitle := regexp.MustCompile(`(?i)^Item[\s\p{Z}]+\w+\.?[\s\p{Z}]+`).ReplaceAllString(cleanTitle, "")
		words := strings.Fields(coreTitle)
		if len(words) > 0 {
			// Construct pattern: Item X . (optional) ... word1 ... word2
			// Use [\s\p{Z}] instead of \s to catch NBSP
			patternStr := `(?i)(?:^|\n)[\s\p{Z}]*(?:\[)?(?:Item[\s\p{Z}]+\w+\.?[\s\p{Z}]+)?`
			for i, word := range words {
				if i > 0 {
					// Allow non-word chars (markers, spaces, separators) between words
					// But ensure we don't cross newlines excessively
					patternStr += `[^\w\n]*`
				}
				patternStr += regexp.QuoteMeta(word)
			}
			patternStr += `.*(?:\n|$)`

			re2 := regexp.MustCompile(patternStr)
			matches = re2.FindAllStringIndex(markdown, -1)
		}
	}

	// Use match that's after TOC area (>10% of document)
	for _, match := range matches {
		if match[0] > len(markdown)/10 {
			endPos := p.findNextSectionEnd(markdown, match[0])
			// Ensure we got a decent chunk
			if endPos > match[0]+200 { // Reduced from 500 to catch smaller notes
				return markdown[match[0]:endPos]
			}
		}
	}

	return ""
}

// TOCEntry represents a parsed TOC item
type TOCEntry struct {
	SectionType string
	Title       string
	Page        int
	Anchor      string
}

// parseTOCFromMarkdown extracts financial statement locations from Markdown TOC
func (p *Parser) parseTOCFromMarkdown(markdown string) []TOCEntry {
	var entries []TOCEntry

	// Patterns for each statement type with anchor capture
	specs := []struct {
		sectionType string
		pattern     string
	}{
		{"BALANCE_SHEET", `(?i)(?:Consolidated\s+)?Balance\s+Sheets?[^|]*\|\s*\[(\d+)\]\(#([^)]+)\)`},
		{"INCOME_STATEMENT", `(?i)(?:Consolidated\s+)?(?:Income\s+Statements?|Statements?\s+of\s+(?:Operations|Income))[^|]*\|\s*\[(\d+)\]\(#([^)]+)\)`},
		{"CASH_FLOW", `(?i)(?:Consolidated\s+)?Statements?\s+of\s+Cash\s+Flows?[^|]*\|\s*\[(\d+)\]\(#([^)]+)\)`},
		{"NOTES", `(?i)Notes?\s+to\s+(?:Consolidated\s+)?Financial\s+Statements?[^|]*\|\s*\[(\d+)\]\(#([^)]+)\)`},
	}

	for _, spec := range specs {
		re := regexp.MustCompile(spec.pattern)
		if match := re.FindStringSubmatch(markdown); match != nil && len(match) >= 3 {
			page, _ := strconv.Atoi(match[1])
			entries = append(entries, TOCEntry{
				SectionType: spec.sectionType,
				Page:        page,
				Anchor:      match[2],
			})
		}
	}

	return entries
}

// extractSectionByAnchor finds anchor in markdown and extracts content

// findNextSectionEnd finds where current section ends
func (p *Parser) findNextSectionEnd(markdown string, startPos int) int {
	remaining := markdown[startPos:]
	maxLen := 50000 // 50KB max per section

	if len(remaining) < maxLen {
		maxLen = len(remaining)
	}

	// Find next major section
	endPatterns := []string{
		`(?i)\n\s*(?:Consolidated\s+)?Balance\s+Sheets?\s*\n`,
		`(?i)\n\s*(?:Consolidated\s+)?(?:Income\s+Statements?|Statements?\s+of)\s*\n`,
		`(?i)\n\s*(?:Consolidated\s+)?Statements?\s+of\s+Cash`,
		`(?i)\n\s*Notes?\s+to\s+`,
		`(?i)\n\s*Item\s+9`,
		`(?i)\n\s*SIGNATURES\s*\n`,
	}

	minEnd := maxLen
	for _, pattern := range endPatterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringIndex(remaining[:maxLen]); match != nil && match[0] > 500 {
			if match[0] < minEnd {
				minEnd = match[0]
			}
		}
	}

	return startPos + minEnd
}

// htmlToMarkdownFull converts entire HTML to Markdown efficiently
func htmlToMarkdownFull(html string) string {
	// Remove scripts and styles
	reScript := regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`)
	html = reScript.ReplaceAllString(html, "")
	reStyle := regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`)
	html = reStyle.ReplaceAllString(html, "")

	// Convert links - preserve anchors for navigation
	reLink := regexp.MustCompile(`<a[^>]*href="([^"]*)"[^>]*>([^<]*)</a>`)
	html = reLink.ReplaceAllString(html, "[$2]($1)")

	// PRESERVE ANCHOR DEFINITIONS - Convert <a name="id"> or <a id="id"> to markdown anchor marker
	// This allows Anchor-First extraction to locate sections in the converted markdown
	reAnchorName := regexp.MustCompile(`<a[^>]*(?:name|id)="([^"]*)"[^>]*>`)
	html = reAnchorName.ReplaceAllString(html, "\n[ANCHOR:$1]\n")

	// Convert tables using RobustTableConverter (Virtual Grid)
	// We need to find <table> tags and process them individually
	reTable := regexp.MustCompile(`(?is)<table[^>]*>.*?</table>`)
	html = reTable.ReplaceAllStringFunc(html, func(tableHTML string) string {
		return convertTableToMarkdown(tableHTML)
	})

	// Cleanup any remaining table tags if nested or missed (fallback)
	html = strings.ReplaceAll(html, "</tr>", " |\n")
	html = strings.ReplaceAll(html, "</td>", " | ")
	html = strings.ReplaceAll(html, "</th>", " | ")

	// Convert line breaks
	html = strings.ReplaceAll(html, "<br>", "\n")
	html = strings.ReplaceAll(html, "<br/>", "\n")
	html = strings.ReplaceAll(html, "</div>", "\n")
	html = strings.ReplaceAll(html, "</p>", "\n\n")

	// Remove remaining tags
	reTag := regexp.MustCompile(`<[^>]+>`)
	html = reTag.ReplaceAllString(html, "")

	// Decode entities
	html = strings.ReplaceAll(html, "&nbsp;", " ")
	html = strings.ReplaceAll(html, "&#160;", " ")
	html = strings.ReplaceAll(html, "&amp;", "&")

	// Clean whitespace
	html = regexp.MustCompile(`[ \t]+`).ReplaceAllString(html, " ")
	html = regexp.MustCompile(`\n\s*\n\s*\n+`).ReplaceAllString(html, "\n\n")

	return strings.TrimSpace(html)
}

// removePandocAttributes strips Pandoc style attributes like [Text]{style="..."}
// It converts them back to plain Text.
func removePandocAttributes(markdown string) string {
	// Pattern to match [Content]{...}
	// Note: We use a non-greedy match for content to handle multiple spans on one line
	// However, nested brackets might be tricky. For now, simple assumption works for headings.
	re := regexp.MustCompile(`\[([^\]]+)\]\{[^}]+\}`)
	return re.ReplaceAllString(markdown, "$1")
}

// findTableEndPosition finds where a financial table section ends
func findTableEndPosition(html string, startPos int) int {
	// Look for the next major section header
	endPatterns := []string{
		`(?i)<[^>]*>Item\s*9[.\s]`,                  // Item 9
		`(?i)<[^>]*>NOTES\s+TO\s+CONSOLIDATED`,      // Notes section
		`(?i)<[^>]*>Schedule\s+I`,                   // Parent Company Schedule
		`(?i)<[^>]*>Parent\s+Company\s+Only`,        // Parent Company Only
		`(?i)<[^>]*>SIGNATURES`,                     // Signatures
		`(?i)<[^>]*>Consolidated\s+Balance\s+Sheet`, // Next statement
		`(?i)<[^>]*>Consolidated\s+Statement`,       // Next statement
	}

	remaining := html[startPos:]
	minEnd := len(remaining)

	for _, pattern := range endPatterns {
		re := regexp.MustCompile(pattern)
		match := re.FindStringIndex(remaining)
		if match != nil && match[0] > 100 && match[0] < minEnd {
			// Must be at least 100 chars away to avoid matching the title itself
			minEnd = match[0]
		}
	}

	// Cap at 100KB per table section
	if minEnd > 100_000 {
		minEnd = 100_000
	}

	return startPos + minEnd
}

// extractItem8MarkdownLegacy is the old Item 8 based extraction for fallback
func (p *Parser) extractItem8MarkdownLegacy(html string) string {
	// Strategy: Find Item 8 and extract until Item 9
	item8Pattern := regexp.MustCompile(`(?i)(Item\s*8\.?\s*[-–—]?\s*Financial\s+Statements)`)
	item8Match := item8Pattern.FindStringIndex(html)

	if item8Match == nil {
		// Try simpler pattern
		item8Pattern = regexp.MustCompile(`(?i)>Item\s*8[.<]`)
		item8Match = item8Pattern.FindStringIndex(html)
	}

	if item8Match == nil {
		return ""
	}

	startPos := item8Match[0]

	// Find end boundary
	item9Pattern := regexp.MustCompile(`(?i)(Item\s*9\.?\s*[-–—]?\s*Changes)`)
	item9Match := item9Pattern.FindStringIndex(html[startPos:])

	var endPos int
	if item9Match != nil {
		endPos = startPos + item9Match[0]
	} else {
		sigPattern := regexp.MustCompile(`(?i)>SIGNATURES<`)
		if sigMatch := sigPattern.FindStringIndex(html[startPos:]); sigMatch != nil {
			endPos = startPos + sigMatch[0]
		} else {
			endPos = startPos + 600_000
			if endPos > len(html) {
				endPos = len(html)
			}
		}
	}

	return htmlToMarkdown(html[startPos:endPos])
}

// Helper functions

func (p *Parser) fetchURL(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/html")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func padCIK(cik string) string {
	// Remove leading zeros first, then pad to 10 digits
	cik = strings.TrimLeft(cik, "0")
	return fmt.Sprintf("%010s", cik)
}

func extractFiscalYear(doc string, filingDate string) int {
	// Try to extract from document name (e.g., "f-20241231.htm")
	re := regexp.MustCompile(`(\d{4})\d{4}\.htm`)
	if m := re.FindStringSubmatch(doc); len(m) > 1 {
		if year, err := strconv.Atoi(m[1]); err == nil {
			return year
		}
	}

	// Fallback to filing date year
	if len(filingDate) >= 4 {
		if year, err := strconv.Atoi(filingDate[:4]); err == nil {
			return year - 1 // 10-K filed in year N is for fiscal year N-1
		}
	}

	return 0
}

func determineFiscalPeriod(form string) string {
	switch form {
	case "10-K":
		return "FY"
	case "10-Q":
		return "Q" // Would need more logic to determine Q1/Q2/Q3
	default:
		return ""
	}
}

func parseNumericValue(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, " ", "")

	// Handle parentheses as negative
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = "-" + s[1:len(s)-1]
	}

	val, _ := strconv.ParseFloat(s, 64)
	return val
}

// extractByAnchors finds financial tables by searching for titles in HTML directly
func (p *Parser) extractByAnchors(html string) map[string]string {
	result := make(map[string]string)

	// Define what we're looking for - search directly in HTML
	tableSpecs := []struct {
		sectionType   string
		titlePatterns []string
	}{
		{
			sectionType: "BALANCE_SHEET",
			titlePatterns: []string{
				`(?i)>\s*Consolidated\s+Balance\s+Sheets?\s*<`,
				`(?i)>\s*CONSOLIDATED\s+BALANCE\s+SHEETS?\s*<`,
			},
		},
		{
			sectionType: "INCOME_STATEMENT",
			titlePatterns: []string{
				`(?i)>\s*Consolidated\s+Income\s+Statements?\s*<`,
				`(?i)>\s*Consolidated\s+Statements?\s+of\s+Operations\s*<`,
				`(?i)>\s*Consolidated\s+Statements?\s+of\s+Income\s*<`,
				`(?i)>\s*CONSOLIDATED\s+INCOME\s+STATEMENTS?\s*<`,
			},
		},
		{
			sectionType: "CASH_FLOW",
			titlePatterns: []string{
				`(?i)>\s*Consolidated\s+Statements?\s+of\s+Cash\s+Flows?\s*<`,
				`(?i)>\s*CONSOLIDATED\s+STATEMENTS?\s+OF\s+CASH\s+FLOWS?\s*<`,
			},
		},
	}

	for _, spec := range tableSpecs {
		for _, pattern := range spec.titlePatterns {
			re := regexp.MustCompile(pattern)
			// Find ALL matches, skip first few (likely in TOC)
			matches := re.FindAllStringIndex(html, -1)

			// We want matches that are NOT in the first 500KB (skip TOC area)
			for _, match := range matches {
				if match[0] > 500_000 { // Skip TOC, find actual table
					content := p.extractFromPosition(html, match[0], 80_000)
					if len(content) > 1000 {
						result[spec.sectionType] = content
						break
					}
				}
			}

			if result[spec.sectionType] != "" {
				break // Found for this section type
			}
		}
	}

	return result
}

// findAnchorPosition finds the position of an anchor ID in HTML
func (p *Parser) findAnchorPosition(html string, anchorID string) int {
	// Try various anchor formats
	patterns := []string{
		`<[^>]*\bid\s*=\s*["']?` + regexp.QuoteMeta(anchorID) + `["']?[^>]*>`,
		`<a[^>]*\bname\s*=\s*["']?` + regexp.QuoteMeta(anchorID) + `["']?[^>]*>`,
		`<[^>]*\bdata-anchor\s*=\s*["']?` + regexp.QuoteMeta(anchorID) + `["']?[^>]*>`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringIndex(html); match != nil {
			return match[0]
		}
	}

	return -1
}

// extractFromPosition extracts content from a position to the next major section
func (p *Parser) extractFromPosition(html string, startPos int, maxLen int) string {
	if startPos < 0 || startPos >= len(html) {
		return ""
	}

	endPos := startPos + maxLen
	if endPos > len(html) {
		endPos = len(html)
	}

	remaining := html[startPos:endPos]

	// Find next major section boundary
	sectionEndPatterns := []string{
		`(?i)<[^>]*>Item\s*9[.\s]`,
		`(?i)<[^>]*>SIGNATURES`,
		`(?i)<[^>]*>(?:Notes|NOTES)\s+(?:to|TO)\s+`,
		`(?i)<[^>]*>Consolidated\s+Balance`,
		`(?i)<[^>]*>Consolidated\s+Statement`,
	}

	minEnd := len(remaining)
	for _, pattern := range sectionEndPatterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringIndex(remaining); match != nil && match[0] > 500 {
			if match[0] < minEnd {
				minEnd = match[0]
			}
		}
	}

	// Cap at 50KB per section for LLM efficiency
	if minEnd > 50_000 {
		minEnd = 50_000
	}

	return html[startPos : startPos+minEnd]
}
