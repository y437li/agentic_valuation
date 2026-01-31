package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	edgar "agentic_valuation/pkg/core/edgar"
)

// TestFetchVerification_AllCompanies validates that the Smart Filing Fetcher
// correctly retrieves the Balance Sheet (and other statements) for all 20 test companies.
func TestFetchVerification_AllCompanies(t *testing.T) {
	// Re-use the list from integration_all_companies_test.go by duplicating it here
	// (or making it public, but duplication is safer for this isolated test).
	testCompanies := []struct {
		Name string
		CIK  string
	}{
		// Technology
		{"Apple", "320193"},
		{"Microsoft", "789019"},
		{"Nvidia", "1045810"},
		{"Intel", "50863"},
		{"Alphabet", "1652044"},

		// Financials
		{"JPMorgan", "19617"},
		{"Goldman Sachs", "886982"},
		{"Berkshire Hathaway", "1067983"},

		// Healthcare
		{"Johnson & Johnson", "200406"},
		{"Pfizer", "78003"},
		{"UnitedHealth", "731766"},

		// Consumer
		{"Amazon", "1018724"},
		{"Walmart", "104169"},
		{"Coca-Cola", "21344"},

		// Energy
		{"ExxonMobil", "34088"},
		{"Chevron", "93410"},

		// Industrial
		{"Ford", "37996"},
		{"Boeing", "12927"},
		{"Caterpillar", "18230"},

		// Telecom
		{"Verizon", "732712"},
	}

	parser := edgar.NewParser()

	fmt.Println("🔎 Starting Fetch Verification for 20 Companies...")
	fmt.Printf("%-20s %-10s %-10s %-20s\n", "Company", "CIK", "Status", "Details")
	fmt.Println(strings.Repeat("-", 60))

	for _, company := range testCompanies {
		// Rate limit protection for SEC API
		time.Sleep(1000 * time.Millisecond)

		t.Run(company.Name, func(t *testing.T) {
			meta, err := parser.GetFilingMetadata(company.CIK, "10-K")
			if err != nil {
				t.Fatalf("Failed to get metadata: %v", err)
			}

			htmlContent, err := parser.FetchSmartFilingHTML(meta)
			if err != nil {
				t.Fatalf("FetchSmartFilingHTML failed: %v", err)
			}

			if len(htmlContent) < 1000 {
				t.Errorf("Fetched content too small: %d bytes", len(htmlContent))
			}

			// Validation: Check for "Total Assets" (common to all BS)
			// Note: Some might use "Total assets" or "TOTAL ASSETS"
			lowerContent := strings.ToLower(htmlContent)
			if !strings.Contains(lowerContent, "total assets") && !strings.Contains(lowerContent, "total current assets") {
				// Fallback for weird formats
				t.Errorf("❌ Missing 'Total Assets' in fetched HTML (Size: %d)", len(htmlContent))
				fmt.Printf("%-20s %-10s ❌ FAIL      Missing 'Total Assets'\n", company.Name, company.CIK)
			} else {
				fmt.Printf("%-20s %-10s ✅ PASS      Size: %d KB\n", company.Name, company.CIK, len(htmlContent)/1024)
			}
		})
	}
}
