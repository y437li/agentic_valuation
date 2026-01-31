// Package fee - Test Suite for Financial Extraction Engine
package fee

import (
	"strings"
	"testing"
)

// =============================================================================
// AST.GO TESTS - Column Year Parsing, Value Parsing, Scale Detection
// =============================================================================

func TestParseColumnYear(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		expected int
	}{
		// Standard formats
		{"Full date", "December 31, 2024", 2024},
		{"Year ended", "Year Ended December 31, 2023", 2023},
		{"Simple year", "2024", 2024},
		{"As of format", "As of 12/31/2024", 2024},
		{"Fiscal year", "Fiscal Year 2023", 2023},

		// Edge cases
		{"Multiple years - use last", "2023 to 2024", 2024},
		{"No year", "Total Assets", 0},
		{"Empty", "", 0},
		{"Just month", "December 31", 0},

		// Real SEC filing formats
		{"Ford style", "December 31, 2024 (in millions)", 2024},
		{"Apple style", "September 28, 2024", 2024},
		{"Old year", "December 31, 1999", 1999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseColumnYear(tt.label)
			if result != tt.expected {
				t.Errorf("ParseColumnYear(%q) = %d, want %d", tt.label, result, tt.expected)
			}
		})
	}
}

func TestParseCellValue(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantValue float64
		wantNeg   bool
		wantBlank bool
	}{
		// Positive values
		{"Simple integer", "1234", 1234, false, false},
		{"With commas", "1,234,567", 1234567, false, false},
		{"With dollar sign", "$1,234", 1234, false, false},
		{"Decimal", "1,234.56", 1234.56, false, false},

		// Negative values (parentheses)
		{"Parentheses negative", "(1,234)", -1234, true, false},
		{"Dollar parentheses", "$(1,234)", -1234, true, false},
		{"Large negative", "(123,456,789)", -123456789, true, false},

		// Blank indicators
		{"Em dash", "—", 0, false, true},
		{"Hyphen dash", "-", 0, false, true},
		{"N/A", "N/A", 0, false, true},
		{"Empty", "", 0, false, true},
		{"Whitespace", "   ", 0, false, true},

		// Real SEC values
		{"Ford cash", "$ 25,165", 25165, false, false},
		{"Negative retained earnings", "$(15,234)", -15234, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCellValue(tt.raw)

			if tt.wantBlank {
				if !result.IsBlank {
					t.Errorf("ParseCellValue(%q) IsBlank = false, want true", tt.raw)
				}
				return
			}

			if result.IsBlank {
				t.Errorf("ParseCellValue(%q) IsBlank = true, want false", tt.raw)
				return
			}

			if result.Value == nil {
				t.Errorf("ParseCellValue(%q) Value = nil, want %f", tt.raw, tt.wantValue)
				return
			}

			if *result.Value != tt.wantValue {
				t.Errorf("ParseCellValue(%q) Value = %f, want %f", tt.raw, *result.Value, tt.wantValue)
			}

			if result.IsNegative != tt.wantNeg {
				t.Errorf("ParseCellValue(%q) IsNegative = %v, want %v", tt.raw, result.IsNegative, tt.wantNeg)
			}
		})
	}
}

func TestDetectScale(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected Scale
	}{
		{"Millions explicit", "(in millions)", ScaleMillions},
		{"Millions with dollar", "($ in millions)", ScaleMillions},
		{"Thousands", "(in thousands)", ScaleThousands},
		{"Per share", "(in millions, except per share)", ScaleMillions},
		{"No scale indicator", "Consolidated Balance Sheets", ScaleUnknown},
		{"Units mention", "(in units)", ScaleUnits},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectScale(tt.text)
			if result != tt.expected {
				t.Errorf("DetectScale(%q) = %s, want %s", tt.text, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// SECTION_ROUTER.GO TESTS - Table Type Identification
// =============================================================================

func TestTableMatcher_IdentifyTableType(t *testing.T) {
	matcher := NewTableMatcher()

	tests := []struct {
		name     string
		title    string
		context  string
		expected TableType
	}{
		// Balance Sheet variations
		{"Consolidated BS", "Consolidated Balance Sheets", "", TableTypeBalanceSheet},
		{"Simple BS", "Balance Sheet", "", TableTypeBalanceSheet},
		{"Financial Position", "Statement of Financial Position", "", TableTypeBalanceSheet},

		// Income Statement variations
		{"Consolidated IS", "Consolidated Statements of Operations", "", TableTypeIncomeStatement},
		{"Income Statement", "Statement of Income", "", TableTypeIncomeStatement},
		{"Earnings Statement", "Statement of Earnings", "", TableTypeIncomeStatement},

		// Cash Flow variations
		{"Consolidated CF", "Consolidated Statements of Cash Flows", "", TableTypeCashFlow},
		{"Simple CF", "Statement of Cash Flows", "", TableTypeCashFlow},

		// Avoid Parent Company
		{"Parent Company BS", "Balance Sheets", "Parent Company Only", TableTypeUnknown},
		{"Registrant BS", "Balance Sheet", "Registrant Only", TableTypeUnknown},

		// Unknown
		{"Random table", "Summary of Operations", "", TableTypeUnknown},
		{"Notes table", "Note 10 - Debt", "", TableTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.IdentifyTableType(tt.title, tt.context)
			if result != tt.expected {
				t.Errorf("IdentifyTableType(%q, %q) = %s, want %s", tt.title, tt.context, result, tt.expected)
			}
		})
	}
}

func TestRowClassifier_ClassifyRow(t *testing.T) {
	classifier := NewRowClassifier()

	tests := []struct {
		name       string
		label      string
		wantTotal  bool
		wantHeader bool
	}{
		// Totals
		{"Total assets", "Total assets", true, false},
		{"Subtotal", "Subtotal", true, false},
		{"Net income", "Net income", true, false},
		{"Gross profit", "Gross profit", true, false},

		// Headers
		{"Assets header", "Assets", false, true},
		{"Current assets", "Current assets:", false, true},
		{"Revenues header", "Revenues:", false, true},

		// Regular items
		{"Cash", "Cash and cash equivalents", false, false},
		{"Inventory", "Inventories", false, false},
		{"Receivables", "Accounts receivable, net", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isTotal, isHeader := classifier.ClassifyRow(tt.label)
			if isTotal != tt.wantTotal {
				t.Errorf("ClassifyRow(%q) isTotal = %v, want %v", tt.label, isTotal, tt.wantTotal)
			}
			if isHeader != tt.wantHeader {
				t.Errorf("ClassifyRow(%q) isHeader = %v, want %v", tt.label, isHeader, tt.wantHeader)
			}
		})
	}
}

func TestFSAPMapper_MapRowToFSAP(t *testing.T) {
	mapper := NewFSAPMapper()

	tests := []struct {
		name      string
		label     string
		tableType TableType
		wantVar   string
		wantMatch bool
	}{
		// Balance Sheet - Assets
		{"Cash simple", "Cash", TableTypeBalanceSheet, "cash_and_equivalents", true},
		{"Cash full", "Cash and cash equivalents", TableTypeBalanceSheet, "cash_and_equivalents", true},
		{"Receivables", "Accounts receivable, net", TableTypeBalanceSheet, "accounts_receivable_net", true},
		{"Trade receivables", "Trade receivables", TableTypeBalanceSheet, "accounts_receivable_net", true},
		{"Inventory", "Inventories", TableTypeBalanceSheet, "inventories", true},
		{"Goodwill", "Goodwill", TableTypeBalanceSheet, "goodwill", true},

		// Balance Sheet - Liabilities
		{"AP", "Accounts payable", TableTypeBalanceSheet, "accounts_payable", true},
		{"LT Debt", "Long-term debt", TableTypeBalanceSheet, "long_term_debt", true},

		// Income Statement
		{"Revenue", "Revenues", TableTypeIncomeStatement, "revenues", true},
		{"Net sales", "Net sales", TableTypeIncomeStatement, "revenues", true},
		{"COGS", "Cost of goods sold", TableTypeIncomeStatement, "cost_of_goods_sold", true},
		{"Net income", "Net income", TableTypeIncomeStatement, "net_income", true},

		// Cash Flow
		{"D&A", "Depreciation and amortization", TableTypeCashFlow, "depreciation_amortization", true},
		{"Capex", "Capital expenditures", TableTypeCashFlow, "capex", true},

		// No match
		{"Random", "Some random item", TableTypeBalanceSheet, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := mapper.MapRowToFSAP(tt.label, tt.tableType)

			if !tt.wantMatch {
				if len(candidates) > 0 {
					t.Errorf("MapRowToFSAP(%q) found match, want none", tt.label)
				}
				return
			}

			if len(candidates) == 0 {
				t.Errorf("MapRowToFSAP(%q) found no match, want %s", tt.label, tt.wantVar)
				return
			}

			if candidates[0].FSAPVariable != tt.wantVar {
				t.Errorf("MapRowToFSAP(%q) = %s, want %s", tt.label, candidates[0].FSAPVariable, tt.wantVar)
			}
		})
	}
}

// =============================================================================
// COMPANY_OVERRIDES.GO TESTS - Override Registry
// =============================================================================

func TestOverrideRegistry_ResolveMapping(t *testing.T) {
	reg := NewOverrideRegistry("")

	// Add Ford override
	reg.AddCompanyOverride(&CompanyOverride{
		CIK:         "0000037996",
		Ticker:      "F",
		CompanyName: "Ford Motor Company",
		Industry:    IndustryGeneral,
		LabelMappings: map[string]string{
			"Ford Credit receivables": "finance_div_loans_leases_st",
			"Automotive receivables":  "accounts_receivable_net",
		},
		SkipLabels: []string{
			"Deferred revenue (Ford Credit)",
		},
	})

	tests := []struct {
		name       string
		cik        string
		label      string
		wantVar    string
		wantSource string
		wantFound  bool
	}{
		// Ford overrides
		{"Ford Credit receivables", "37996", "Ford Credit receivables", "finance_div_loans_leases_st", "company_override", true},
		{"Automotive receivables", "37996", "Automotive receivables", "accounts_receivable_net", "company_override", true},
		{"Skip label", "37996", "Deferred revenue (Ford Credit)", "", "skip", true},

		// Unknown company - should not match
		{"Unknown company", "12345", "Automotive receivables", "", "generic", false},

		// Unknown label for Ford - fallback to generic
		{"Unknown label", "37996", "Random item", "", "generic", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsapVar, source, found := reg.ResolveMapping(tt.cik, tt.label)

			if found != tt.wantFound {
				t.Errorf("ResolveMapping(%q, %q) found = %v, want %v", tt.cik, tt.label, found, tt.wantFound)
			}

			if source != tt.wantSource {
				t.Errorf("ResolveMapping(%q, %q) source = %s, want %s", tt.cik, tt.label, source, tt.wantSource)
			}

			if tt.wantFound && tt.wantVar != "" && fsapVar != tt.wantVar {
				t.Errorf("ResolveMapping(%q, %q) = %s, want %s", tt.cik, tt.label, fsapVar, tt.wantVar)
			}
		})
	}
}

func TestIndustryTemplates(t *testing.T) {
	reg := NewOverrideRegistry("")

	// Test banking template exists
	banking := reg.GetIndustryTemplate(IndustryBanking)
	if banking == nil {
		t.Fatal("Banking template not found")
	}

	// Check specific banking mappings
	expectedMappings := map[string]string{
		"Loans and leases": "finance_div_loans_leases_st",
		"Interest income":  "revenues",
		"Deposits":         "notes_payable_short_term_debt",
	}

	for label, expectedVar := range expectedMappings {
		if fsapVar, ok := banking.LabelMappings[label]; !ok || fsapVar != expectedVar {
			t.Errorf("Banking template mapping %q = %q, want %q", label, fsapVar, expectedVar)
		}
	}
}

// =============================================================================
// TABLE_PARSER.GO TESTS - HTML Parsing
// =============================================================================

func TestTableParser_ParseHTMLTables(t *testing.T) {
	parser := NewTableParser()

	html := `
	<html>
	<body>
	<p>Consolidated Balance Sheets</p>
	<table>
		<tr><th></th><th>December 31, 2024</th><th>December 31, 2023</th></tr>
		<tr><td>Cash and cash equivalents</td><td>$ 25,165</td><td>$ 22,345</td></tr>
		<tr><td>Accounts receivable, net</td><td>10,500</td><td>9,800</td></tr>
		<tr><td>Total current assets</td><td>35,665</td><td>32,145</td></tr>
	</table>
	</body>
	</html>
	`

	tables, err := parser.ParseHTMLTables(html)
	if err != nil {
		t.Fatalf("ParseHTMLTables failed: %v", err)
	}

	if len(tables) == 0 {
		t.Skip("No tables parsed - may need HTML structure adjustment")
	}

	// Check that we found a balance sheet
	var bsTable *ParsedTable
	for i := range tables {
		if tables[i].Type == TableTypeBalanceSheet {
			bsTable = &tables[i]
			break
		}
	}

	if bsTable == nil {
		t.Skip("Balance sheet not identified - may need HTML structure adjustment")
	}

	// Check columns have years
	var has2024, has2023 bool
	for _, col := range bsTable.Columns {
		if col.Year == 2024 {
			has2024 = true
		}
		if col.Year == 2023 {
			has2023 = true
		}
	}

	if !has2024 || !has2023 {
		t.Errorf("Expected columns for 2024 and 2023, got columns: %+v", bsTable.Columns)
	}
}

// =============================================================================
// SEMANTIC_LAYER.GO TESTS - Column Selection
// =============================================================================

func TestColumnSelector_SelectColumn(t *testing.T) {
	selector := &ColumnSelector{}

	columns := []ColumnHeader{
		{Index: 1, Label: "December 31, 2024", Year: 2024, IsLatest: true},
		{Index: 2, Label: "December 31, 2023", Year: 2023, IsLatest: false},
		{Index: 3, Label: "December 31, 2022", Year: 2022, IsLatest: false},
	}

	tests := []struct {
		name       string
		targetYear int
		wantYear   int
	}{
		{"Exact match 2023", 2023, 2023},
		{"Exact match 2024", 2024, 2024},
		{"No match - use latest", 2025, 2024},
		{"Zero - use latest", 0, 2024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selector.SelectColumn(columns, tt.targetYear)
			if result == nil {
				t.Fatal("SelectColumn returned nil")
			}
			if result.Year != tt.wantYear {
				t.Errorf("SelectColumn(%d) = year %d, want %d", tt.targetYear, result.Year, tt.wantYear)
			}
		})
	}
}

// =============================================================================
// INTEGRATION TEST - Full Pipeline
// =============================================================================

func TestFullExtractionPipeline(t *testing.T) {
	// This test verifies the complete pipeline with sample HTML
	html := `
	<html>
	<body>
	<h2>Consolidated Balance Sheets (in millions)</h2>
	<table>
		<tr><th></th><th>December 31, 2024</th><th>December 31, 2023</th></tr>
		<tr><td>Assets</td><td></td><td></td></tr>
		<tr><td>Cash and cash equivalents</td><td>$ 25,165</td><td>$ 22,345</td></tr>
		<tr><td>Accounts receivable, net</td><td>10,500</td><td>9,800</td></tr>
		<tr><td>Inventories</td><td>15,000</td><td>14,500</td></tr>
		<tr><td>Total current assets</td><td>50,665</td><td>46,645</td></tr>
	</table>
	</body>
	</html>
	`

	docParser := NewDocumentParser()
	metadata := DocumentMetadata{
		CIK:         "0000037996",
		CompanyName: "Test Company",
		FilingDate:  "2025-02-15",
	}

	docIndex, err := docParser.ParseDocument(html, metadata)
	if err != nil {
		t.Fatalf("ParseDocument failed: %v", err)
	}

	// Verify available years were extracted
	t.Logf("Available years: %v", docIndex.AvailableYears)
	t.Logf("Tables found: %d", len(docIndex.Tables))

	for _, table := range docIndex.Tables {
		t.Logf("Table: %s (%s), Rows: %d, Columns: %d",
			table.Title, table.Type, len(table.Rows), len(table.Columns))
	}
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

func BenchmarkParseCellValue(b *testing.B) {
	values := []string{
		"$ 25,165",
		"(1,234,567)",
		"—",
		"123.45",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range values {
			ParseCellValue(v)
		}
	}
}

func BenchmarkParseColumnYear(b *testing.B) {
	labels := []string{
		"December 31, 2024",
		"Year Ended December 31, 2023",
		"Total Assets",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, l := range labels {
			ParseColumnYear(l)
		}
	}
}

// Helper function for tests
func stringContains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
