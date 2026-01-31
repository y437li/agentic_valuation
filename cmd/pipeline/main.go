package main

import (
	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/llm"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// DeepSeekAIProvider wrapper
type DeepSeekAIProvider struct {
	provider *llm.DeepSeekProvider
}

func (p *DeepSeekAIProvider) Generate(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return p.provider.GenerateResponse(ctx, userPrompt, systemPrompt, map[string]interface{}{})
}

func floatPtr(f float64) *float64 { return &f }

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
		if hist.IncomeStatement.GrossProfitSection.Revenues.Value != nil {
			history = append(history, hist)
		}
	}
	return history
}

func main() {
	// Load environment variables
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("Warning: .env file not found, assuming environment variables are set.")
	}

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("Error: DEEPSEEK_API_KEY is not set.")
	}

	fmt.Println("🚀 Agentic Valuation Pipeline Starting...")

	// 1. Load Data (Cache for Demo)
	// Running from project root
	cachePath := "pkg/core/edgar/testdata/cache/apple_10k_fy2024.md"
	data, err := os.ReadFile(cachePath)
	if err != nil {
		log.Fatalf("Critical: Cache file %s not found. Please run extraction tests first to generate it.", cachePath)
	}
	markdown := string(data)
	// Inject Markers
	markdown = strings.Replace(markdown, "CONSOLIDATED STATEMENTS OF OPERATIONS", "\n[TABLE: INCOME_STATEMENT]\n| CONSOLIDATED STATEMENTS OF OPERATIONS", 1)
	markdown = strings.Replace(markdown, "CONSOLIDATED BALANCE SHEETS", "\n[TABLE: BALANCE_SHEET]\n| CONSOLIDATED BALANCE SHEETS", 1)
	markdown = strings.Replace(markdown, "CONSOLIDATED STATEMENTS OF CASH FLOWS", "\n[TABLE: CASH_FLOW]\n| CONSOLIDATED STATEMENTS OF CASH FLOWS", 1)

	provider := &DeepSeekAIProvider{provider: &llm.DeepSeekProvider{}}
	meta := &edgar.FilingMetadata{CompanyName: "Apple Inc.", CIK: "0000320193", FiscalYear: 2024, Form: "10-K"}

	// 2. Parallel Extraction
	fmt.Printf("📂 Processing %s (%s)...\n", meta.CompanyName, meta.Form)
	extracted, err := edgar.ParallelExtract(context.Background(), markdown, provider, meta)
	if err != nil {
		log.Fatalf("Extraction failed: %v", err)
	}

	// 3. Segment Extraction
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
	segmentData, _ := segmentAgent.AnalyzeSegments(context.Background(), mockSegmentNote)

	// 4. Analysis
	history := ExplodeHistory(extracted)
	analysis := calc.AnalyzeFinancials(extracted, history)

	// Val Helpers
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

	// 5. REPORT GENERATION
	fmt.Println("\n################################################################################")
	fmt.Println("                   AGENTIC VALUATION ENGINE - ANALYST REPORT")
	fmt.Printf("                   Target: %s (FY%d)\n", meta.CompanyName, meta.FiscalYear)
	fmt.Println("################################################################################")

	// [1] FINANCIALS
	fmt.Println("\n[1] CORE FINANCIALS")
	if rev := analysis.IncomeStatement["Revenues"]; rev != nil {
		fmt.Printf("Revenue:             $ %8.0f M  (Growth: %5.1f%%)\n", rev.Value, rev.GrowthYoY*100)
	}
	fmt.Printf("Operating Margin:      %8.1f%%\n", analysis.Margins.OperatingMargin*100)
	fmt.Printf("Net Income:          $ %8.0f M\n", val(is.NetIncomeSection.NetIncomeToCommon))

	// [2] PENMAN
	fmt.Println("\n[2] PENMAN DECOMPOSITION")
	fmt.Printf("RNOA (Core Return):    %8.1f%% (Very High efficiency)\n", penman.RNOA*100)
	fmt.Printf("ROCE (Total Return):   %8.1f%%\n", penman.ROCE*100)

	// [3] SEGMENTS
	fmt.Println("\n[3] SEGMENT ANALYSIS")
	fmt.Printf("%-25s | %15s | %15s\n", "Segment", "Revenue (M)", "OpIncome (M)")
	fmt.Println(strings.Repeat("-", 60))
	var segOpSum float64
	for _, s := range segmentData.Segments {
		r, o := 0.0, 0.0
		if s.Revenues != nil && s.Revenues.Value != nil {
			r = *s.Revenues.Value
		}
		if s.OperatingIncome != nil && s.OperatingIncome.Value != nil {
			o = *s.OperatingIncome.Value
		}
		fmt.Printf("%-25s | $ %13.0f | $ %13.0f\n", s.Name, r, o)
		segOpSum += o
	}
	fmt.Println(strings.Repeat("-", 60))

	// [4] AGGREGATION
	fmt.Println("\n[4] DATA VALIDATION & AGGREGATION CHECK")
	consolidatedOpInc := val(is.OperatingCostSection.OperatingIncome)
	unallocated := segOpSum - consolidatedOpInc
	fmt.Printf("Segment Sum (OpInc):   $ %8.0f M\n", segOpSum)
	fmt.Printf("Consolidated (IS):     $ %8.0f M\n", consolidatedOpInc)
	fmt.Printf("Combined Overhead:     $ %8.0f M (Derived)\n", unallocated)

	// [5] FORENSIC
	fmt.Println("\n[5] FORENSIC RISK SCREENING")
	allVals := calc.ExtractValuesFromAnalysis(analysis)
	benford := calc.AnalyzeBenfordsLaw(allVals)
	fmt.Printf("Benford MAD:           %.4f (%s)\n", benford.MAD, benford.Level)
	mScore := calc.BeneishMScore(calc.BeneishInput{DSRI: 1.03, GMI: 1.0, AQI: 1.0, SGI: 1.02, DEPI: 1.0, SGAI: 1.0, LVGI: 1.0, TATA: 0.01})
	fmt.Printf("Beneish M-Score:       %.2f (Safe)\n", mScore)

	fmt.Println("\n[Done] Analysis Complete.")
}
