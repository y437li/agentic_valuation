package synthesis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/validate"
)

// =============================================================================
// END-TO-END TEST: Apple 2020+ 10-K → GoldenRecord
// =============================================================================
// This test demonstrates the full workflow:
// 1. Load cached 10-K extraction(s)
// 2. Run Zipper to synthesize all years
// 3. Generate final output data
// 4. Validate financial integrity

// CachedExtraction represents the JSON structure from .cache/edgar/fsap_extractions/
type CachedExtraction struct {
	ID              string                  `json:"id"`
	CIK             string                  `json:"cik"`
	Ticker          string                  `json:"ticker"`
	CompanyName     string                  `json:"company_name"`
	FiscalYear      int                     `json:"fiscal_year"`
	FiscalPeriod    string                  `json:"fiscal_period"`
	FormType        string                  `json:"form_type"`
	AccessionNumber string                  `json:"accession_number"`
	FilingDate      string                  `json:"filing_date"`  // Date filed with SEC
	IsAmendment     bool                    `json:"is_amendment"` // True if 10-K/A
	Data            *edgar.FSAPDataResponse `json:"data"`
}

// FinalOutput represents the structured output for downstream consumption
type FinalOutput struct {
	Ticker       string                 `json:"ticker"`
	CompanyName  string                 `json:"company_name"`
	CIK          string                 `json:"cik"`
	Years        map[int]*YearlyMetrics `json:"years"`
	Restatements []RestatementLog       `json:"restatements"`
	Metadata     OutputMetadata         `json:"metadata"`
}

type YearlyMetrics struct {
	FiscalYear      int    `json:"fiscal_year"`
	SourceAccession string `json:"source_accession"`

	// Income Statement
	Revenue         *float64 `json:"revenue,omitempty"`
	GrossProfit     *float64 `json:"gross_profit,omitempty"`
	OperatingIncome *float64 `json:"operating_income,omitempty"`
	NetIncome       *float64 `json:"net_income,omitempty"`

	// Balance Sheet
	TotalAssets      *float64 `json:"total_assets,omitempty"`
	TotalLiabilities *float64 `json:"total_liabilities,omitempty"`
	TotalEquity      *float64 `json:"total_equity,omitempty"`
	Cash             *float64 `json:"cash,omitempty"`

	// Cash Flow
	CFO          *float64 `json:"cfo,omitempty"`
	CFI          *float64 `json:"cfi,omitempty"`
	CFF          *float64 `json:"cff,omitempty"`
	CapEx        *float64 `json:"capex,omitempty"`
	FreeCashFlow *float64 `json:"free_cash_flow,omitempty"`

	// Derived Metrics
	GrossMargin     *float64 `json:"gross_margin,omitempty"`
	OperatingMargin *float64 `json:"operating_margin,omitempty"`
	NetMargin       *float64 `json:"net_margin,omitempty"`
	RevenueYoY      *float64 `json:"revenue_yoy,omitempty"`
	NetIncomeYoY    *float64 `json:"net_income_yoy,omitempty"`
}

type OutputMetadata struct {
	FilingsProcessed int    `json:"filings_processed"`
	YearsAvailable   []int  `json:"years_available"`
	DataQuality      string `json:"data_quality"`
}

// loadCachedExtractions loads extractions from the cache directory, optionally filtering by ticker
func loadCachedExtractions(t *testing.T, cacheDir string, filterTicker string) []*CachedExtraction {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("Failed to read cache directory: %v", err)
	}

	var extractions []*CachedExtraction
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(cacheDir, entry.Name()))
		if err != nil {
			t.Logf("Warning: Failed to read %s: %v", entry.Name(), err)
			continue
		}

		var cached CachedExtraction
		if err := json.Unmarshal(data, &cached); err != nil {
			t.Logf("Warning: Failed to parse %s: %v", entry.Name(), err)
			continue
		}

		// FILTER BY TICKER if provided
		if filterTicker != "" && cached.Ticker != filterTicker {
			continue
		}

		extractions = append(extractions, &cached)
		t.Logf("Loaded: %s (FY%d, %s)", entry.Name(), cached.FiscalYear, cached.FormType)
	}

	return extractions
}

// TestE2E_Apple2020Plus is the main end-to-end test
func TestE2E_Apple2020Plus(t *testing.T) {
	// 1. Determine Company from Env
	targetTicker := os.Getenv("TEST_TICKER")
	if targetTicker == "" {
		targetTicker = "AAPL" // Default
	}
	t.Logf("Testing Target: %s", targetTicker)

	// Try multiple cache paths
	cachePaths := []string{
		"../../../.cache/edgar/fsap_extractions",
		"../../../../.cache/edgar/fsap_extractions",
	}

	var cacheDir string
	for _, p := range cachePaths {
		absPath, _ := filepath.Abs(p)
		if _, err := os.Stat(absPath); err == nil {
			cacheDir = absPath
			break
		}
	}

	if cacheDir == "" {
		t.Skip("Cache directory not found, skipping E2E test")
	}

	t.Logf("Using cache: %s", cacheDir)

	// Step 1: Load all cached extractions
	t.Log("\n=== STEP 1: Load Cached Extractions ===")
	extractions := loadCachedExtractions(t, cacheDir, targetTicker)
	if len(extractions) == 0 {
		t.Skipf("No cached extractions found for %s. Run E2E Pipeline first to populate cache.", targetTicker)
	}
	t.Logf("Found %d cached extraction(s) for %s", len(extractions), targetTicker)

	// Step 2: Convert to ExtractionSnapshots
	t.Log("\n=== STEP 2: Prepare Snapshots ===")
	var snapshots []ExtractionSnapshot
	var ticker, companyName, cik string

	for _, cached := range extractions {
		if cached.Data == nil {
			t.Logf("Skipping %s: no data", cached.AccessionNumber)
			continue
		}

		ticker = cached.Ticker
		companyName = cached.CompanyName
		cik = cached.CIK

		snapshot := ExtractionSnapshot{
			FilingMetadata: SourceMetadata{
				AccessionNumber: cached.AccessionNumber,
				FilingDate:      cached.FilingDate,
				Form:            cached.FormType,
				IsAmended:       cached.IsAmendment,
			},
			FiscalYear: cached.FiscalYear,
			Data:       cached.Data,
		}
		snapshots = append(snapshots, snapshot)
		t.Logf("Prepared snapshot: FY%d from %s (Filed: %s, Amendment: %v)",
			cached.FiscalYear, cached.AccessionNumber, cached.FilingDate, cached.IsAmendment)
	}

	// Step 3: Run Zipper
	t.Log("\n=== STEP 3: Run Zipper Synthesis ===")
	zipper := NewZipperEngine()
	record, err := zipper.Stitch(ticker, cik, snapshots)
	if err != nil {
		t.Fatalf("Zipper failed: %v", err)
	}
	t.Logf("Zipper synthesized %d years", len(record.Timeline))

	// Step 4: Generate Final Output
	t.Log("\n=== STEP 4: Generate Final Output ===")
	output := generateFinalOutput(t, record, ticker, companyName, cik)

	// Step 5: Print Results
	t.Log("\n=== FINAL OUTPUT ===")
	printFinalOutput(t, output)

	// Step 6: Validate Results
	t.Log("\n=== VALIDATION ===")
	validateOutput(t, output)

	// Save output to file
	outputJSON, _ := json.MarshalIndent(output, "", "  ")
	outputPath := filepath.Join(cacheDir, fmt.Sprintf("%s_golden_record.json", ticker))
	os.WriteFile(outputPath, outputJSON, 0644)
	t.Logf("Saved output to: %s", outputPath)
}

func generateFinalOutput(t *testing.T, record *GoldenRecord, ticker, companyName, cik string) *FinalOutput {
	output := &FinalOutput{
		Ticker:       ticker,
		CompanyName:  companyName,
		CIK:          cik,
		Years:        make(map[int]*YearlyMetrics),
		Restatements: record.Restatements,
		Metadata: OutputMetadata{
			FilingsProcessed: 1, // Would be actual count
			DataQuality:      "GOOD",
		},
	}

	var years []int
	for year, snapshot := range record.Timeline {
		years = append(years, year)

		metrics := extractMetrics(snapshot, year)
		output.Years[year] = metrics
	}
	output.Metadata.YearsAvailable = years

	// Calculate YoY for each year
	for year := range output.Years {
		priorYear := year - 1
		if prior, ok := output.Years[priorYear]; ok {
			current := output.Years[year]

			if current.Revenue != nil && prior.Revenue != nil && *prior.Revenue != 0 {
				yoy := validate.CalculateYoY(*current.Revenue, *prior.Revenue)
				current.RevenueYoY = &yoy
			}
			if current.NetIncome != nil && prior.NetIncome != nil && *prior.NetIncome != 0 {
				yoy := validate.CalculateYoY(*current.NetIncome, *prior.NetIncome)
				current.NetIncomeYoY = &yoy
			}
		}
	}

	return output
}

func extractMetrics(snapshot *YearlySnapshot, year int) *YearlyMetrics {
	metrics := &YearlyMetrics{
		FiscalYear:      year,
		SourceAccession: snapshot.SourceFiling.AccessionNumber,
	}

	// Extract from Income Statement
	if snapshot.IncomeStatement.GrossProfitSection != nil {
		if snapshot.IncomeStatement.GrossProfitSection.Revenues != nil {
			metrics.Revenue = snapshot.IncomeStatement.GrossProfitSection.Revenues.Value
		}
		if snapshot.IncomeStatement.GrossProfitSection.GrossProfit != nil {
			metrics.GrossProfit = snapshot.IncomeStatement.GrossProfitSection.GrossProfit.Value
		}
	}
	if snapshot.IncomeStatement.NetIncomeSection != nil && snapshot.IncomeStatement.NetIncomeSection.NetIncomeToCommon != nil {
		metrics.NetIncome = snapshot.IncomeStatement.NetIncomeSection.NetIncomeToCommon.Value
	}

	// Extract from Balance Sheet
	if snapshot.BalanceSheet.ReportedForValidation.TotalAssets != nil {
		metrics.TotalAssets = snapshot.BalanceSheet.ReportedForValidation.TotalAssets.Value
	}
	if snapshot.BalanceSheet.ReportedForValidation.TotalLiabilities != nil {
		metrics.TotalLiabilities = snapshot.BalanceSheet.ReportedForValidation.TotalLiabilities.Value
	}
	if snapshot.BalanceSheet.ReportedForValidation.TotalEquity != nil {
		// Start with reported stockholders' equity
		totalEquity := *snapshot.BalanceSheet.ReportedForValidation.TotalEquity.Value
		// Add NCI if present (some companies report NCI separately from stockholders' equity)
		if snapshot.BalanceSheet.Equity.NoncontrollingInterests != nil && snapshot.BalanceSheet.Equity.NoncontrollingInterests.Value != nil {
			totalEquity += *snapshot.BalanceSheet.Equity.NoncontrollingInterests.Value
		}
		metrics.TotalEquity = &totalEquity
	}
	if snapshot.BalanceSheet.CurrentAssets.CashAndEquivalents != nil {
		metrics.Cash = snapshot.BalanceSheet.CurrentAssets.CashAndEquivalents.Value
	}

	// Extract from Cash Flow
	if snapshot.CashFlowStatement.CashSummary != nil {
		if snapshot.CashFlowStatement.CashSummary.NetCashOperating != nil {
			metrics.CFO = snapshot.CashFlowStatement.CashSummary.NetCashOperating.Value
		}
		if snapshot.CashFlowStatement.CashSummary.NetCashInvesting != nil {
			metrics.CFI = snapshot.CashFlowStatement.CashSummary.NetCashInvesting.Value
		}
		if snapshot.CashFlowStatement.CashSummary.NetCashFinancing != nil {
			metrics.CFF = snapshot.CashFlowStatement.CashSummary.NetCashFinancing.Value
		}
	}
	if snapshot.CashFlowStatement.InvestingActivities != nil && snapshot.CashFlowStatement.InvestingActivities.Capex != nil {
		metrics.CapEx = snapshot.CashFlowStatement.InvestingActivities.Capex.Value
	}

	// Calculate FCF
	if metrics.CFO != nil && metrics.CapEx != nil {
		fcf := validate.CalculateFCF(*metrics.CFO, *metrics.CapEx)
		metrics.FreeCashFlow = &fcf
	}

	// Calculate Margins
	if metrics.Revenue != nil && *metrics.Revenue != 0 {
		if metrics.GrossProfit != nil {
			gm := *metrics.GrossProfit / *metrics.Revenue * 100
			metrics.GrossMargin = &gm
		}
		if metrics.NetIncome != nil {
			nm := *metrics.NetIncome / *metrics.Revenue * 100
			metrics.NetMargin = &nm
		}
	}

	return metrics
}

func printFinalOutput(t *testing.T, output *FinalOutput) {
	t.Logf("\n%s (%s)", output.CompanyName, output.Ticker)
	t.Logf("CIK: %s", output.CIK)
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Sort years
	years := output.Metadata.YearsAvailable
	for i := len(years) - 1; i >= 0; i-- {
		year := years[i]
		m := output.Years[year]

		t.Logf("\nFY%d (Source: %s)", year, m.SourceAccession)
		t.Log("────────────────────────────────────────")

		if m.Revenue != nil {
			yoyStr := ""
			if m.RevenueYoY != nil {
				yoyStr = fmt.Sprintf(" (YoY: %+.1f%%)", *m.RevenueYoY)
			}
			t.Logf("  Revenue:         $%.0fM%s", *m.Revenue, yoyStr)
		}
		if m.GrossProfit != nil {
			gmStr := ""
			if m.GrossMargin != nil {
				gmStr = fmt.Sprintf(" (%.1f%% margin)", *m.GrossMargin)
			}
			t.Logf("  Gross Profit:    $%.0fM%s", *m.GrossProfit, gmStr)
		}
		if m.NetIncome != nil {
			yoyStr := ""
			if m.NetIncomeYoY != nil {
				yoyStr = fmt.Sprintf(" (YoY: %+.1f%%)", *m.NetIncomeYoY)
			}
			t.Logf("  Net Income:      $%.0fM%s", *m.NetIncome, yoyStr)
		}
		if m.TotalAssets != nil {
			t.Logf("  Total Assets:    $%.0fM", *m.TotalAssets)
		}
		if m.CFO != nil {
			t.Logf("  Operating CF:    $%.0fM", *m.CFO)
		}
		if m.FreeCashFlow != nil {
			t.Logf("  Free Cash Flow:  $%.0fM", *m.FreeCashFlow)
		}
	}

	if len(output.Restatements) > 0 {
		t.Log("\n⚠️ RESTATEMENTS DETECTED:")
		for _, r := range output.Restatements {
			t.Logf("  Year %d: %.1f%% change (Item: %s)", r.Year, r.DeltaPercent, r.Item)
		}
	}
}

func validateOutput(t *testing.T, output *FinalOutput) {
	passCount := 0
	failCount := 0

	for year, m := range output.Years {
		// Check Balance Sheet equation
		if m.TotalAssets != nil && m.TotalLiabilities != nil && m.TotalEquity != nil {
			check := validate.CheckBalanceEquation(*m.TotalAssets, *m.TotalLiabilities, *m.TotalEquity, 1.0)
			if check.IsBalanced {
				passCount++
				t.Logf("  ✓ FY%d: Balance Sheet equation valid", year)
			} else {
				failCount++
				t.Logf("  ✗ FY%d: Balance Sheet mismatch (diff: $%.0fM)", year, check.Difference)
			}
		}

		// Check for suspicious values
		if m.Revenue != nil {
			check := validate.CheckForOutlier("Revenue", *m.Revenue, 0, 1000)
			if *m.Revenue == 0 {
				failCount++
				t.Logf("  ✗ FY%d: Revenue is zero (likely extraction error)", year)
			} else if !check.IsOutlier {
				passCount++
			}
		}
	}

	t.Logf("\nValidation Summary: %d passed, %d failed", passCount, failCount)
	if failCount > 0 {
		t.Log("⚠️ Some validation checks failed - review data quality")
	} else {
		t.Log("✓ All validation checks passed")
	}
}
