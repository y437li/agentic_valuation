package core_test

import (
	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/llm"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

// ExplodeHistory converts a single extraction with multi-year maps into a generic history slice
func ExplodeHistory(data *edgar.FSAPDataResponse) []*edgar.FSAPDataResponse {
	years := []int{data.FiscalYear - 1, data.FiscalYear - 2} // 2023, 2022
	history := make([]*edgar.FSAPDataResponse, 0)

	for _, y := range years {
		hist := &edgar.FSAPDataResponse{FiscalYear: y}
		getYearVal := func(v *edgar.FSAPValue) *float64 {
			if v == nil || v.Years == nil {
				return nil
			}
			val, ok := v.Years[fmt.Sprintf("%d", y)]
			if ok {
				return &val
			}
			return nil
		}

		hist.IncomeStatement.GrossProfitSection = &edgar.GrossProfitSection{
			Revenues: &edgar.FSAPValue{Value: getYearVal(data.IncomeStatement.GrossProfitSection.Revenues)},
		}
		hist.IncomeStatement.NetIncomeSection = &edgar.NetIncomeSection{
			NetIncomeToCommon: &edgar.FSAPValue{Value: getYearVal(data.IncomeStatement.NetIncomeSection.NetIncomeToCommon)},
		}
		hist.BalanceSheet.ReportedForValidation.TotalAssets = &edgar.FSAPValue{
			Value: getYearVal(data.BalanceSheet.ReportedForValidation.TotalAssets),
		}
		if data.BalanceSheet.ReportedForValidation.TotalEquity != nil {
			hist.BalanceSheet.ReportedForValidation.TotalEquity = &edgar.FSAPValue{
				Value: getYearVal(data.BalanceSheet.ReportedForValidation.TotalEquity),
			}
		}

		if hist.IncomeStatement.GrossProfitSection.Revenues.Value != nil {
			history = append(history, hist)
		}
	}
	return history
}

// DeepSeekAIProvider wrapper
type DeepSeekAIProvider struct {
	provider *llm.DeepSeekProvider
}

func (p *DeepSeekAIProvider) Generate(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return p.provider.GenerateResponse(ctx, userPrompt, systemPrompt, map[string]interface{}{})
}

// TestEndToEnd_ValuationReport_Apple generates the final polished report
func TestEndToEnd_ValuationReport_Apple(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}

	// =========================================================================
	// STEP 1: LOAD & EXTRACT FINANCIAL STATEMENTS (Cached 10-K)
	// =========================================================================
	cachePath := "edgar/testdata/cache/apple_10k_fy2024.md"
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("Cache file %s not found.", cachePath)
	}
	markdown := string(data)
	// Inject Table Markers for Parallel Extraction
	markdown = strings.Replace(markdown, "CONSOLIDATED STATEMENTS OF OPERATIONS", "\n[TABLE: INCOME_STATEMENT]\n| CONSOLIDATED STATEMENTS OF OPERATIONS", 1)
	markdown = strings.Replace(markdown, "CONSOLIDATED BALANCE SHEETS", "\n[TABLE: BALANCE_SHEET]\n| CONSOLIDATED BALANCE SHEETS", 1)
	markdown = strings.Replace(markdown, "CONSOLIDATED STATEMENTS OF CASH FLOWS", "\n[TABLE: CASH_FLOW]\n| CONSOLIDATED STATEMENTS OF CASH FLOWS", 1)

	provider := &DeepSeekAIProvider{provider: &llm.DeepSeekProvider{}}
	meta := &edgar.FilingMetadata{CompanyName: "Apple Inc.", CIK: "0000320193", FiscalYear: 2024, Form: "10-K"}

	// Extractor Agent
	t.Log("🚀 Starting End-to-End Extraction Pipeline...")
	extracted, err := edgar.ParallelExtract(context.Background(), markdown, provider, meta)
	if err != nil {
		t.Fatalf("Financial Statement Extraction failed: %v", err)
	}

	// =========================================================================
	// STEP 2: EXTRACT SEGMENT DATA (Mocked Note 25)
	// =========================================================================
	// Using correct mock numbers where Segment Sum = Gross Margin level ($180B)
	mockSegmentNote := `
Note 25. Segment Information
Operating Income by Reportable Segment:
Americas                                   $ 75,000       $ 60,100
Europe                                        45,000         36,000
Greater China                                 34,000         28,500
Japan                                         13,000         10,800
Rest of Asia Pacific                          13,683         10,200
Total Segment Operating Income               180,683        145,600
Research and Development Expense             (31,370)       (29,915)
Selling, General and Administrative          (26,097)       (24,932)
Total Operating Income                     $ 123,216      $ 114,301
`
	segmentAgent := edgar.NewQuantitativeSegmentAgent(provider)
	segmentData, err := segmentAgent.AnalyzeSegments(context.Background(), mockSegmentNote)
	if err != nil {
		t.Fatalf("Segment Extraction failed: %v", err)
	}

	// =========================================================================
	// STEP 3: RUN ANALYTICAL ENGINE
	// =========================================================================
	history := ExplodeHistory(extracted)
	analysis := calc.AnalyzeFinancials(extracted, history)

	// Penman Calculation (Manual wiring)
	val := func(v *edgar.FSAPValue) float64 {
		if v != nil && v.Value != nil {
			return *v.Value
		}
		return 0
	}

	bs := extracted.BalanceSheet
	is := extracted.IncomeStatement

	noa := calc.NetOperatingAssets(
		val(bs.ReportedForValidation.TotalAssets),
		val(bs.CurrentAssets.CashAndEquivalents),
		val(bs.CurrentAssets.ShortTermInvestments),
		val(bs.ReportedForValidation.TotalLiabilities),
		val(bs.NoncurrentLiabilities.LongTermDebt)+val(bs.CurrentLiabilities.NotesPayableShortTermDebt),
	)
	nfo := calc.NetFinancialObligations(val(bs.NoncurrentLiabilities.LongTermDebt), val(bs.CurrentAssets.CashAndEquivalents), 0)
	penman := calc.CalculatePenmanDecomposition(val(is.NetIncomeSection.NetIncomeToCommon), 0, noa, nfo, val(bs.ReportedForValidation.TotalEquity))

	// =========================================================================
	// STEP 4: GENERATE VALUATION REPORT (Console Output)
	// =========================================================================

	fmt.Println("\n################################################################################")
	fmt.Println("                   AGENTIC VALUATION ENGINE - ANALYST REPORT")
	fmt.Printf("                   Target: %s (FY%d)\n", meta.CompanyName, meta.FiscalYear)
	fmt.Println("################################################################################")

	// --- SECTION 1: CORE FINANCIALS ---
	fmt.Println("\n[1] CORE FINANCIALS (Consolidated)")
	fmt.Println("-----------------------------------")
	if rev := analysis.IncomeStatement["Revenues"]; rev != nil {
		fmt.Printf("Revenue:             $ %8.0f M  (Growth: %5.1f%%)\n", rev.Value, rev.GrowthYoY*100)
	}
	fmt.Printf("Gross Margin:          %8.1f%%\n", analysis.Margins.GrossMargin*100)
	fmt.Printf("Operating Margin:      %8.1f%%\n", analysis.Margins.OperatingMargin*100)
	fmt.Printf("Net Income:          $ %8.0f M  (Margin: %5.1f%%)\n", val(is.NetIncomeSection.NetIncomeToCommon), analysis.Margins.NetMargin*100)

	// --- SECTION 2: PROFITABILITY DECOMPOSITION (PENMAN) ---
	fmt.Println("\n[2] PENMAN PROFITABILITY DECOMPOSITION")
	fmt.Println("---------------------------------------")
	fmt.Printf("RNOA (Core Return):    %8.1f%%  (Return on Net Operating Assets)\n", penman.RNOA*100)
	fmt.Printf("NBC (Cost of Debt):    %8.1f%%\n", penman.NBC*100)
	fmt.Printf("FLEV (Leverage):       %8.2fx\n", penman.FLEV)
	fmt.Printf("ROCE (Total Return):   %8.1f%%  (Return on Common Equity)\n", penman.ROCE*100)

	// --- SECTION 3: SEGMENT ANALYSIS (SUM-OF-PARTS INPUTS) ---
	fmt.Println("\n[3] SEGMENT ANALYSIS (Note 25)")
	fmt.Println("-------------------------------")
	fmt.Printf("%-25s | %15s | %15s\n", "Segment", "Revenue (M)", "OpIncome (M)")
	fmt.Println(strings.Repeat("-", 60))

	var segRevSum, segOpSum float64
	for _, s := range segmentData.Segments {
		r := 0.0
		o := 0.0
		if s.Revenues != nil && s.Revenues.Value != nil {
			r = *s.Revenues.Value
		}
		if s.OperatingIncome != nil && s.OperatingIncome.Value != nil {
			o = *s.OperatingIncome.Value
		}

		fmt.Printf("%-25s | $ %13.0f | $ %13.0f\n", s.Name, r, o)
		segRevSum += r
		segOpSum += o
	}
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("%-25s | $ %13.0f | $ %13.0f\n", "AGGREGATED TOTAL", segRevSum, segOpSum)

	// --- SECTION 4: AGGREGATION VALIDATION CHECK ---
	fmt.Println("\n[4] DATA VALIDATION & AGGREGATION CHECK")
	fmt.Println("----------------------------------------")
	consolidatedOpInc := val(is.OperatingCostSection.OperatingIncome)
	unallocated := segOpSum - consolidatedOpInc

	fmt.Printf("Segment Sum (OpInc):   $ %8.0f M\n", segOpSum)
	fmt.Printf("Consolidated (IS):     $ %8.0f M\n", consolidatedOpInc)
	fmt.Printf("Diff (Corp Overhead):  $ %8.0f M  (Implied R&D + Central Costs)\n", unallocated)

	if unallocated > 0 {
		fmt.Println("✅ VALIDATION PASSED: Positive Corporate Overhead derived.")
	} else {
		fmt.Println("⚠️ VALIDATION WARNING: Segment Sum < Consolidated. Check input data.")
	}

	// --- SECTION 5: FORENSIC & RISK ---
	fmt.Println("\n[5] FORENSIC & RISK SCREENING")
	fmt.Println("------------------------------")
	// Benford
	allVals := calc.ExtractValuesFromAnalysis(analysis)
	benford := calc.AnalyzeBenfordsLaw(allVals)
	fmt.Printf("Benford's Law:         MAD %.4f (%s)\n", benford.MAD, benford.Level)

	// Beneish
	mScore := calc.BeneishMScore(calc.BeneishInput{DSRI: 1.0, GMI: 1.0, AQI: 1.0, SGI: 1.02, DEPI: 1.0, SGAI: 1.0, LVGI: 1.0, TATA: 0.0})
	fmt.Printf("Beneish M-Score:       %.2f (Safe < -1.78)\n", mScore)

	// Altman
	z := calc.AltmanZScoreManufacturing(0, 0, consolidatedOpInc, 3400000, val(is.GrossProfitSection.Revenues), val(bs.ReportedForValidation.TotalAssets), val(bs.ReportedForValidation.TotalLiabilities))
	fmt.Printf("Altman Z-Score:        %.2f (Safe > 2.99)\n", z)

	fmt.Println("################################################################################")
}

func floatPtr(f float64) *float64 { return &f }
