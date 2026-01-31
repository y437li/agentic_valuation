//go:build integration

package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	edgar "agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/llm"
	"agentic_valuation/pkg/core/prompt"
)

// CompanyTestCase defines a company to test
type CompanyTestCase struct {
	Name      string
	CIK       string
	CacheFile string
	Industry  string // For reference only
}

// Companies to test - 20 companies across different industries
var testCompanies = []CompanyTestCase{
	// Technology (5)
	{Name: "Apple", CIK: "320193", CacheFile: "apple_10k_fy2024.html", Industry: "Technology"},
	{Name: "Microsoft", CIK: "789019", CacheFile: "", Industry: "Technology"},
	{Name: "Nvidia", CIK: "1045810", CacheFile: "", Industry: "Technology"},
	{Name: "Intel", CIK: "50863", CacheFile: "intel_10k_2024.html", Industry: "Technology"},
	{Name: "Alphabet", CIK: "1652044", CacheFile: "", Industry: "Technology"},

	// Financials (3)
	{Name: "JPMorgan", CIK: "19617", CacheFile: "", Industry: "Finance"},
	{Name: "Goldman Sachs", CIK: "886982", CacheFile: "", Industry: "Finance"},
	{Name: "Berkshire Hathaway", CIK: "1067983", CacheFile: "", Industry: "Finance"},

	// Healthcare (3)
	{Name: "Johnson & Johnson", CIK: "200406", CacheFile: "", Industry: "Healthcare"},
	{Name: "Pfizer", CIK: "78003", CacheFile: "", Industry: "Healthcare"},
	{Name: "UnitedHealth", CIK: "731766", CacheFile: "", Industry: "Healthcare"},

	// Consumer (3)
	{Name: "Amazon", CIK: "1018724", CacheFile: "", Industry: "Consumer"},
	{Name: "Walmart", CIK: "104169", CacheFile: "", Industry: "Consumer"},
	{Name: "Coca-Cola", CIK: "21344", CacheFile: "", Industry: "Consumer"},

	// Energy (2)
	{Name: "ExxonMobil", CIK: "34088", CacheFile: "", Industry: "Energy"},
	{Name: "Chevron", CIK: "93410", CacheFile: "", Industry: "Energy"},

	// Industrials (3)
	{Name: "Ford", CIK: "37996", CacheFile: "ford_10k_2024.html", Industry: "Industrial"},
	{Name: "Boeing", CIK: "12927", CacheFile: "", Industry: "Industrial"},
	{Name: "Caterpillar", CIK: "18230", CacheFile: "", Industry: "Industrial"},

	// Telecom (1)
	{Name: "Verizon", CIK: "732712", CacheFile: "", Industry: "Telecom"},
}

// TestIntegration_AllCompanies_AllStatements runs 3 statements test for all companies
func TestIntegration_AllCompanies_AllStatements(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	if os.Getenv("ENABLE_REAL_SEC_TEST") != "true" {
		t.Skip("Skipping real SEC test. Set ENABLE_REAL_SEC_TEST=true to run.")
	}

	if err := prompt.LoadFromDirectory("../../../resources"); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}

	for _, company := range testCompanies {
		t.Run(company.Name, func(t *testing.T) {
			runCompanyTest(t, company)
		})
	}
}

// runCompanyTest runs all 3 statements extraction for a company
func runCompanyTest(t *testing.T, company CompanyTestCase) {
	t.Logf("📊 Testing %s (CIK: %s)...", company.Name, company.CIK)

	// Get markdown
	markdown := getCompanyMarkdown(t, company)
	if markdown == "" {
		t.Skip("Failed to get markdown")
	}
	t.Logf("Markdown: %d chars", len(markdown))

	// Initialize agents
	provider := &DeepSeekAIProvider{provider: &llm.DeepSeekProvider{}}
	navigator := edgar.NewNavigatorAgent(provider)
	mapper := edgar.NewTableMapperAgent(provider)
	extractor := edgar.NewGoExtractor()
	ctx := context.Background()

	// NavigatorAgent
	toc, _ := navigator.ParseTOC(ctx, markdown)

	// Statement configs
	statements := []statementConfig{
		{
			name:      "Income_Statement",
			tableType: "income_statement",
			patterns:  []string{"CONSOLIDATED STATEMENTS OF INCOME", "CONSOLIDATED STATEMENTS OF OPERATIONS", "STATEMENTS OF INCOME"},
			expected:  []string{"revenue", "net income"},
		},
		{
			name:      "Balance_Sheet",
			tableType: "balance_sheet",
			patterns:  []string{"CONSOLIDATED BALANCE SHEETS", "CONSOLIDATED BALANCE SHEET"},
			expected:  []string{"cash", "total assets"},
		},
		{
			name:      "Cash_Flow",
			tableType: "cash_flow",
			patterns:  []string{"CONSOLIDATED STATEMENTS OF CASH FLOWS", "STATEMENTS OF CASH FLOWS"},
			expected:  []string{"net income", "operating"},
		},
	}

	// Add LLM titles
	if toc != nil {
		if toc.IncomeStatement != nil && toc.IncomeStatement.Title != "" {
			statements[0].patterns = append([]string{toc.IncomeStatement.Title}, statements[0].patterns...)
		}
		if toc.BalanceSheet != nil && toc.BalanceSheet.Title != "" {
			statements[1].patterns = append([]string{toc.BalanceSheet.Title}, statements[1].patterns...)
		}
		if toc.CashFlow != nil && toc.CashFlow.Title != "" {
			statements[2].patterns = append([]string{toc.CashFlow.Title}, statements[2].patterns...)
		}
	}

	// Test each statement
	for _, stmt := range statements {
		t.Run(stmt.name, func(t *testing.T) {
			testStatement(t, ctx, markdown, stmt, mapper, extractor)
		})
	}
}

type statementConfig struct {
	name      string
	tableType string
	patterns  []string
	expected  []string
}

func testStatement(t *testing.T, ctx context.Context, markdown string, stmt statementConfig, mapper *edgar.TableMapperAgent, extractor *edgar.GoExtractor) {
	startLine := findTableLine(markdown, stmt.patterns)
	if startLine == 0 {
		t.Skipf("⚠️ %s not found", stmt.name)
	}
	t.Logf("✅ Found at line %d", startLine)

	tableMarkdown := sliceLines(markdown, startLine, startLine+60)
	mapping, err := mapper.MapTable(ctx, stmt.tableType, tableMarkdown)
	if err != nil {
		t.Fatalf("MapTable failed: %v", err)
	}

	parsedTable := extractor.ParseMarkdownTableWithOffset(tableMarkdown, stmt.tableType, startLine)
	values := extractor.ExtractValues(parsedTable, mapping)

	foundItems := make(map[string]bool)
	for _, v := range values {
		t.Logf("   %s -> %v", v.Label, v.Years)
		for _, expected := range stmt.expected {
			if strings.Contains(strings.ToLower(v.Label), expected) {
				foundItems[expected] = true
			}
		}
	}

	for _, expected := range stmt.expected {
		if !foundItems[expected] {
			t.Logf("⚠️ Missing: %s", expected)
		}
	}

	// Run aggregation validation
	switch stmt.tableType {
	case "balance_sheet":
		result := edgar.ValidateBalanceSheet(values, "")
		t.Logf("📊 Balance Sheet: Assets=%.0f, L+E=%.0f, Diff=%.2f%%",
			result.TotalAssets, result.TotalLiabEquity, result.DiffPercent)
		if !result.IsValid {
			t.Errorf("❌ %s", result.Message)
		} else if result.TotalAssets > 0 {
			t.Logf("✅ %s", result.Message)
		}

	case "income_statement":
		result := edgar.ValidateIncomeStatement(values, "")
		t.Logf("📊 Income Statement: Revenue=%.0f, COGS=%.0f, GrossProfit=%.0f (calc=%.0f), NetIncome=%.0f",
			result.Revenues, result.COGS, result.GrossProfit, result.GrossProfitCalc, result.NetIncome)
		if !result.IsValid {
			t.Errorf("❌ %s", result.Message)
		} else {
			t.Logf("✅ %s", result.Message)
		}

	case "cash_flow":
		result := edgar.ValidateCashFlow(values, "")
		t.Logf("📊 Cash Flow: Op=%.0f, Inv=%.0f, Fin=%.0f, NetChange=%.0f (calc=%.0f)",
			result.OperatingCF, result.InvestingCF, result.FinancingCF, result.NetChangeCF, result.NetChangeCFCalc)
		if !result.IsValid {
			t.Errorf("❌ %s", result.Message)
		} else {
			t.Logf("✅ %s", result.Message)
		}
	}
}

func getCompanyMarkdown(t *testing.T, company CompanyTestCase) string {
	if company.CacheFile == "" {
		// Auto-generate cache filename if not provided
		safeName := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				return r
			}
			return '_'
		}, company.Name)
		company.CacheFile = fmt.Sprintf("%s_%s.html", safeName, company.CIK)
	}
	cacheFile := filepath.Join("testdata/cache", company.CacheFile)

	var htmlContent string
	if data, err := os.ReadFile(cacheFile); err == nil {
		t.Logf("📁 Loaded from cache")
		htmlContent = string(data)
	} else {
		t.Log("⬇️ Fetching from SEC...")
		parser := edgar.NewParser()
		meta, err := parser.GetFilingMetadata(company.CIK, "10-K")
		if err != nil {
			t.Fatalf("Failed to get metadata: %v", err)
			return ""
		}
		htmlContent, err = parser.FetchSmartFilingHTML(meta)
		if err != nil {
			t.Fatalf("Fetch failed: %v", err)
			return ""
		}
		os.MkdirAll("testdata/cache", 0755)
		os.WriteFile(cacheFile, []byte(htmlContent), 0644)
	}

	return edgar.HTMLToMarkdown(htmlContent)
}
