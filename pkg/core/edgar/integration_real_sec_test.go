package edgar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentic_valuation/pkg/core/llm"
	"agentic_valuation/pkg/core/prompt"
)

// Cache directory for downloaded HTML files
const testCacheDir = "testdata/cache"

// TestIntegration_RealData_Apple_AllStatements tests all 3 financial statements
func TestIntegration_RealData_Apple_AllStatements(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	if os.Getenv("ENABLE_REAL_SEC_TEST") != "true" {
		t.Skip("Skipping real SEC test. Set ENABLE_REAL_SEC_TEST=true to run.")
	}

	if err := prompt.LoadFromDirectory("../../../resources"); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}

	// Load cached markdown
	mdCacheFile := filepath.Join(testCacheDir, "apple_10k_fy2024.md")
	data, err := os.ReadFile(mdCacheFile)
	if err != nil {
		t.Fatalf("Cache not found. Run TestIntegration_RealData_Apple_EndToEnd first: %v", err)
	}
	markdown := string(data)
	t.Logf("📁 Loaded markdown from cache: %d chars", len(markdown))

	// Initialize agents
	provider := &DeepSeekAIProvider{provider: &llm.DeepSeekProvider{}}
	navigator := NewNavigatorAgent(provider)
	mapper := NewTableMapperAgent(provider)
	extractor := NewGoExtractor()
	ctx := context.Background()

	// Use NavigatorAgent to find all sections
	t.Log("🧭 Using NavigatorAgent to locate all financial statements...")
	toc, err := navigator.ParseTOC(ctx, markdown)
	if err != nil {
		t.Logf("⚠️ ParseTOC warning: %v", err)
	}

	// Test cases for each statement type
	testCases := []struct {
		name           string
		tableType      string
		searchPatterns []string
		llmTitle       string
		expectedItems  []string
	}{
		{
			name:      "Income Statement",
			tableType: "income_statement",
			searchPatterns: []string{
				"CONSOLIDATED STATEMENTS OF OPERATIONS",
				"STATEMENTS OF OPERATIONS",
				"INCOME STATEMENT",
			},
			expectedItems: []string{"Net sales", "Net income"},
		},
		{
			name:      "Balance Sheet",
			tableType: "balance_sheet",
			searchPatterns: []string{
				"CONSOLIDATED BALANCE SHEETS",
				"BALANCE SHEETS",
			},
			expectedItems: []string{"Cash", "Total assets"},
		},
		{
			name:      "Cash Flow",
			tableType: "cash_flow",
			searchPatterns: []string{
				"CONSOLIDATED STATEMENTS OF CASH FLOWS",
				"STATEMENTS OF CASH FLOWS",
			},
			expectedItems: []string{"Net income", "Cash generated"},
		},
	}

	// Add LLM-suggested titles if available
	if toc != nil {
		if toc.IncomeStatement != nil && toc.IncomeStatement.Title != "" {
			testCases[0].llmTitle = toc.IncomeStatement.Title
		}
		if toc.BalanceSheet != nil && toc.BalanceSheet.Title != "" {
			testCases[1].llmTitle = toc.BalanceSheet.Title
		}
		if toc.CashFlow != nil && toc.CashFlow.Title != "" {
			testCases[2].llmTitle = toc.CashFlow.Title
		}
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build search patterns
			patterns := tc.searchPatterns
			if tc.llmTitle != "" {
				patterns = append([]string{tc.llmTitle}, patterns...)
				t.Logf("🤖 LLM suggested: '%s'", tc.llmTitle)
			}

			// Find table
			startLine := findTableLine(markdown, patterns)
			if startLine == 0 {
				t.Skipf("⚠️ %s not found in markdown", tc.name)
			}
			t.Logf("✅ Found at line %d", startLine)

			endLine := startLine + 50
			tableMarkdown := sliceLines(markdown, startLine, endLine)

			// Map table
			t.Log("🗺️  Mapping...")
			mapping, err := mapper.MapTable(ctx, tc.tableType, tableMarkdown)
			if err != nil {
				t.Fatalf("MapTable failed: %v", err)
			}

			// Extract values
			t.Log("⛏️  Extracting...")
			parsedTable := extractor.ParseMarkdownTableWithOffset(tableMarkdown, tc.tableType, startLine)
			values := extractor.ExtractValues(parsedTable, mapping)

			// Verify expected items
			foundItems := make(map[string]bool)
			for _, v := range values {
				t.Logf("   %s -> %v", v.Label, v.Years)
				for _, expected := range tc.expectedItems {
					if strings.Contains(strings.ToLower(v.Label), strings.ToLower(expected)) {
						foundItems[expected] = true
					}
				}
			}

			for _, expected := range tc.expectedItems {
				if !foundItems[expected] {
					t.Errorf("Missing expected item: %s", expected)
				}
			}
		})
	}
}

// findTableLine searches for table header using multiple patterns
func findTableLine(content string, patterns []string) int {
	lines := strings.Split(content, "\n")
	limit := len(lines)
	if limit > 500 {
		limit = 500
	}

	for i := 0; i < limit; i++ {
		lineLower := strings.ToLower(lines[i])
		for _, pattern := range patterns {
			if strings.Contains(lineLower, strings.ToLower(pattern)) {
				return i + 1
			}
		}
	}
	return 0
}

// TestIntegration_RealData_Apple_EndToEnd - moved from previous file
func TestIntegration_RealData_Apple_EndToEnd(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	if os.Getenv("ENABLE_REAL_SEC_TEST") != "true" {
		t.Skip("Skipping real SEC test. Set ENABLE_REAL_SEC_TEST=true to run.")
	}

	if err := prompt.LoadFromDirectory("../../../resources"); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}

	// 1. Hardcoded Apple 10-K metadata
	t.Log("🔍 Using hardcoded Apple (AAPL) 10-K metadata...")
	cikPadded := "0000320193"
	accessionNumber := "0000320193-24-000123"
	filingURL := "https://www.sec.gov/Archives/edgar/data/320193/000032019324000123/aapl-20240928.htm"
	t.Logf("Using 10-K: %s", accessionNumber)

	// 2. Check cache first
	cacheFile := filepath.Join(testCacheDir, "apple_10k_fy2024.html")
	var htmlContent string

	if data, err := os.ReadFile(cacheFile); err == nil {
		t.Logf("📁 Loaded from cache: %s (%d bytes)", cacheFile, len(data))
		htmlContent = string(data)
	} else {
		// Download and cache
		t.Log("⬇️ Cache miss. Smart-fetching filing from SEC...")
		parser := NewParser()
		meta := &FilingMetadata{
			CIK:             cikPadded,
			AccessionNumber: accessionNumber,
			FilingDate:      "2024-10-31",
			Form:            "10-K",
			CompanyName:     "Apple Inc.",
			FilingURL:       filingURL,
		}

		var err error
		htmlContent, err = parser.FetchSmartFilingHTML(meta)
		if err != nil {
			t.Fatalf("Smart fetch failed: %v", err)
		}
		t.Logf("Downloaded %d bytes", len(htmlContent))

		// Save to cache
		os.MkdirAll(testCacheDir, 0755)
		if err := os.WriteFile(cacheFile, []byte(htmlContent), 0644); err != nil {
			t.Logf("Warning: Failed to save cache: %v", err)
		} else {
			t.Logf("💾 Saved to cache: %s", cacheFile)
		}
	}

	// 3. Convert to Markdown
	t.Log("📝 Converting to Markdown...")
	markdown := htmlToMarkdown(htmlContent)
	t.Logf("Converted to Markdown: %d chars", len(markdown))

	// Save markdown for other tests
	mdCacheFile := filepath.Join(testCacheDir, "apple_10k_fy2024.md")
	os.WriteFile(mdCacheFile, []byte(markdown), 0644)
	t.Logf("💾 Saved markdown: %s", mdCacheFile)

	// 4. Initialize Agents
	provider := &DeepSeekAIProvider{provider: &llm.DeepSeekProvider{}}
	navigator := NewNavigatorAgent(provider)
	mapper := NewTableMapperAgent(provider)
	extractor := NewGoExtractor()
	ctx := context.Background()

	// 5. Use NavigatorAgent to find Balance Sheet title
	t.Log("🧭 Using NavigatorAgent to locate Balance Sheet...")

	var targetTitle string
	var startLine int

	toc, err := navigator.ParseTOC(ctx, markdown)
	if err != nil {
		t.Logf("⚠️ ParseTOC error (expected for iXBRL): %v", err)
	} else if toc.BalanceSheet != nil && toc.BalanceSheet.Title != "" {
		targetTitle = toc.BalanceSheet.Title
		t.Logf("🤖 LLM suggested title: '%s'", targetTitle)
	}

	// Pattern search (primary for iXBRL, fallback for traditional)
	searchPatterns := []string{
		"CONSOLIDATED BALANCE SHEETS",
		"BALANCE SHEETS",
		"STATEMENTS OF FINANCIAL POSITION",
		"CONSOLIDATED STATEMENTS OF FINANCIAL POSITION",
	}
	// Add LLM-suggested title to patterns if available
	if targetTitle != "" {
		searchPatterns = append([]string{targetTitle}, searchPatterns...)
	}

	startLine = findTableLine(markdown, searchPatterns)
	if startLine == 0 {
		t.Log("No Balance Sheet found in markdown")
		t.Logf("First 2000 chars:\n%s", markdown[:min(2000, len(markdown))])
		t.Skip("⚠️ Balance Sheet not found")
	}
	t.Logf("✅ Found at line %d", startLine)

	endLine := startLine + 50
	t.Logf("✅ Located Section: Lines %d-%d", startLine, endLine)

	bsMarkdown := sliceLines(markdown, startLine, endLine)

	// 6. Map & Extract
	t.Log("🗺️  Mapping Table...")
	mappingResult, err := mapper.MapTable(ctx, "balance_sheet", bsMarkdown)
	if err != nil {
		t.Fatalf("MapTable failed: %v", err)
	}

	t.Log("⛏️  Extracting Values...")
	parsedTable := extractor.ParseMarkdownTableWithOffset(bsMarkdown, "balance_sheet", startLine)
	values := extractor.ExtractValues(parsedTable, mappingResult)

	// 7. Verify
	foundCash := false
	for _, v := range values {
		scale := "none"
		if v.Provenance != nil {
			scale = v.Provenance.Scale
		}
		t.Logf("   Extracted: %-30s -> %v [Unit: %s]", v.Label, v.Years, scale)

		if strings.Contains(strings.ToLower(v.Label), "cash") {
			foundCash = true
		}
	}

	if !foundCash {
		t.Error("Did not extract 'Cash'")
	}
}

func sliceLines(s string, start, end int) string {
	lines := strings.Split(s, "\n")
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) || end < start {
		return ""
	}
	return strings.Join(lines[start-1:end], "\n")
}
