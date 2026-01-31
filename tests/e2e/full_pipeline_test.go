package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentic_valuation/pkg/core/agent"
	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/debate"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/llm"
	"agentic_valuation/pkg/core/projection"
	"agentic_valuation/pkg/core/synthesis"
	"agentic_valuation/pkg/core/valuation"
)

// CompanyTestCase defines a company to test (Copied from edgar/integration_all_companies_test.go)
type CompanyTestCase struct {
	Name      string
	CIK       string
	CacheFile string
	Industry  string
}

// Companies to test - 20 companies across different industries
var testCompanies = []CompanyTestCase{
	// Technology (5)
	{Name: "Apple", CIK: "0000320193", CacheFile: "apple_10k_fy2024.html", Industry: "Technology"},
	{Name: "Microsoft", CIK: "0000789019", CacheFile: "", Industry: "Technology"},
	{Name: "Nvidia", CIK: "0001045810", CacheFile: "", Industry: "Technology"},
	{Name: "Intel", CIK: "0000050863", CacheFile: "intel_10k_2024.html", Industry: "Technology"},
	{Name: "Alphabet", CIK: "0001652044", CacheFile: "", Industry: "Technology"},

	// Financials (3)
	{Name: "JPMorgan", CIK: "0000019617", CacheFile: "", Industry: "Finance"},
	{Name: "Goldman Sachs", CIK: "0000886982", CacheFile: "", Industry: "Finance"},
	{Name: "Berkshire Hathaway", CIK: "0001067983", CacheFile: "", Industry: "Finance"},

	// Healthcare (3)
	{Name: "Johnson & Johnson", CIK: "0000200406", CacheFile: "", Industry: "Healthcare"},
	{Name: "Pfizer", CIK: "0000078003", CacheFile: "", Industry: "Healthcare"},
	{Name: "UnitedHealth", CIK: "0000731766", CacheFile: "", Industry: "Healthcare"},

	// Consumer (3)
	{Name: "Amazon", CIK: "0001018724", CacheFile: "", Industry: "Consumer"},
	{Name: "Walmart", CIK: "0000104169", CacheFile: "", Industry: "Consumer"},
	{Name: "Coca-Cola", CIK: "0000021344", CacheFile: "", Industry: "Consumer"},

	// Energy (2)
	{Name: "ExxonMobil", CIK: "0000034088", CacheFile: "", Industry: "Energy"},
	{Name: "Chevron", CIK: "0000093410", CacheFile: "", Industry: "Energy"},

	// Industrials (3)
	{Name: "Ford", CIK: "0000037996", CacheFile: "ford_10k_2024.html", Industry: "Industrial"},
	{Name: "Boeing", CIK: "0000012927", CacheFile: "", Industry: "Industrial"},
	{Name: "Caterpillar", CIK: "0000018230", CacheFile: "", Industry: "Industrial"},

	// Telecom (1)
	{Name: "Verizon", CIK: "0000732712", CacheFile: "", Industry: "Telecom"},
}

// DeepSeekAIProvider wrapper (copied for E2E)
type DeepSeekAIProvider struct {
	provider *llm.DeepSeekProvider
}

func (p *DeepSeekAIProvider) Generate(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return p.provider.GenerateResponse(ctx, userPrompt, systemPrompt, map[string]interface{}{})
}

// TestE2E_FullPipeline_ExtractionToSynthesis ensures that:
// 1. Data can be extracted from Raw Markdown (Stage 1).
// 2. Extracted data can be successfully zipped into a Golden Record (Stage 2).
func TestE2E_FullPipeline_ExtractionToSynthesis(t *testing.T) {
	// [MODIFIED] Injecting Real API Keys for Testing
	os.Setenv("DEEPSEEK_API_KEY", "sk-f3c4ca80d1924d7e82f12cbfaa23a7b0")
	os.Setenv("QWEN_API_KEY", "sk-8a84fa93d8934b46a825c1e0943ebe5b")
	os.Setenv("GEMINI_API_KEY", "AIzaSyCd5FkGoMV8GWVwCI0cKIs_ppiwZjCSJ5o")
	os.Setenv("ENABLE_REAL_SEC_TEST", "true")

	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	if os.Getenv("ENABLE_REAL_SEC_TEST") != "true" {
		t.Skip("Skipping real SEC test. Set ENABLE_REAL_SEC_TEST=true to run.")
	}

	provider := &DeepSeekAIProvider{provider: &llm.DeepSeekProvider{}}

	for _, company := range testCompanies {
		t.Run(company.Name, func(t *testing.T) {
			// A. Get Markdown
			markdown, err := fetchOrLoadMarkdown(t, company)
			if err != nil {
				t.Fatalf("Failed to load markdown for %s: %v", company.Name, err)
			}
			if len(markdown) < 1000 {
				t.Fatalf("Markdown content too short for %s", company.Name)
			}

			// B. Stage 1: Extraction
			t.Logf("🚀 [Stage 1] Extracting %s...", company.Name)
			meta := &edgar.FilingMetadata{
				CompanyName: company.Name,
				CIK:         company.CIK,
				FiscalYear:  2024, // Assuming most recent is 2024 for this test
				Form:        "10-K",
			}
			extractedData, err := edgar.ParallelExtract(context.Background(), markdown, provider, meta)
			if err != nil {
				t.Fatalf("Extraction failed: %v", err)
			}

			// NORMALIZE DATA (Legacy Flat -> Nested)
			normalizeReport(extractedData)

			// Validate Extraction Basic Integrity
			if extractedData.IncomeStatement.GrossProfitSection.Revenues.Value == nil {
				t.Error("❌ Revenue is nil after extraction")
			} else {
				t.Logf("✅ Revenue: $%.2f", *extractedData.IncomeStatement.GrossProfitSection.Revenues.Value)
			}

			// C. Stage 2: Synthesis (The Zipper)
			t.Logf("🤐 [Stage 2] Zipping into Golden Record...")

			// Wrap in Snapshot
			snapshot := synthesis.ExtractionSnapshot{
				FilingMetadata: synthesis.SourceMetadata{
					AccessionNumber: fmt.Sprintf("TEST-ACC-%s-2024", company.CIK),
					FilingDate:      time.Now().Format("2006-01-02"),
					Form:            "10-K",
				},
				FiscalYear: 2024,
				Data:       extractedData,
			}

			zipper := synthesis.NewZipperEngine()

			// We synthesize a single snapshot here to verify the pipeline contract.
			// In a real scenario, we'd pass multiple snapshots.
			goldenRecord, err := zipper.Stitch(company.Name, company.CIK, []synthesis.ExtractionSnapshot{snapshot})
			if err != nil {
				t.Fatalf("Zipper Stitch failed: %v", err)
			}

			// D. Verify Golden Record
			if len(goldenRecord.Timeline) == 0 {
				t.Error("❌ Golden Record Timeline is empty")
			}

			// Check if 2024 data exists in timeline
			if yearData, ok := goldenRecord.Timeline[2024]; !ok {
				t.Error("❌ Golden Record missing 2024 data")
			} else {
				if yearData.IncomeStatement.GrossProfitSection.Revenues.Value == nil {
					t.Error("❌ Golden Record has nil Revenue for 2024")
				}
				t.Logf("✅ Golden Record Validated for %s", company.Name)
			}

			// E. Stage 3: Cognitive Roundtable (Debate)
			t.Logf("🧠 [Stage 3] Orchestrating Debate (REAL MODE)...")

			// Initialize Real Manager with DeepSeek provider using the standard Config
			agentConfig := agent.Config{
				ActiveProvider: "deepseek", // Global default
				Agents: map[string]agent.AgentConfig{
					"macro":       {Provider: "deepseek"},
					"fundamental": {Provider: "deepseek"},
					"sentiment":   {Provider: "deepseek"},
					"skeptic":     {Provider: "deepseek"},
					"optimist":    {Provider: "deepseek"},
					"synthesizer": {Provider: "deepseek"},
				},
			}
			mgr := agent.NewManager(agentConfig)

			// We must manually inject our existing provider instance into the manager's map
			// because the test creates a specific one, or rely on NewManager creating one.
			// However NewManager creates internal providers. Let's just trust NewManager(config) works
			// if environment variables are set (DEEPSEEK_API_KEY).

			// Create Orchestrator (isSimulation = false)
			// We use "AAPL" as ticker for Apple to help Transcript loading match (though optional)
			ticker := "AAPL"
			if company.Name != "Apple" {
				ticker = company.CIK
			} // Fallback for others

			orch := debate.NewOrchestrator(
				fmt.Sprintf("TEST-DEBATE-%s", company.CIK),
				ticker,
				company.Name,
				"2024",
				false, // isSimulation: FALSE -> REAL AGENTS
				debate.ModeAutomatic,
				mgr,
				nil, // No repo persistence for E2E validation
			)

			// Inject Golden Record using Builder to auto-calc Common Size & Ratios
			poolBuilder := debate.NewMaterialPoolBuilder(extractedData, nil)

			// Inject Qualitative Segments if available
			if extractedData.Qualitative != nil {
				poolBuilder.WithQualitativeSegments(&extractedData.Qualitative.Segments)
			}

			// Capture error if any
			mp, err := poolBuilder.Build()
			if err != nil {
				t.Fatalf("❌ Failed to build Material Pool: %v", err)
			}
			orch.SharedContext.MaterialPool = mp

			// No pre-populated consensus drafts needed for Real Mode - Agents will generate them!

			// Subscribe to stream output to stdout
			msgChan, _ := orch.Subscribe()
			go func() {
				for msg := range msgChan {
					t.Logf("[%s] %s: %s", msg.AgentRole, msg.AgentName, extractSummary(msg.Content))
				}
			}()

			// Run Debate
			orch.Run(context.Background())

			// F. Validation & Persistence
			if orch.FinalReport == nil {
				t.Fatal("❌ Debate failed to produce Final Report")
			}
			t.Logf("✅ Final Report Executive Summary Length: %d chars", len(orch.FinalReport.ExecutiveSummary))

			// Verify Mandatory Assumptions
			requiredKeys := []string{"rev_growth", "cogs_pct", "sga_pct", "tax_rate", "wacc", "terminal_growth"}
			missingKeys := []string{}

			for _, key := range requiredKeys {
				if val, ok := orch.FinalReport.Assumptions[key]; ok {
					t.Logf("   - %s: %.4f %s (Conf: %.2f)", key, val.Value, val.Unit, val.Confidence)
				} else {
					missingKeys = append(missingKeys, key)
				}
			}

			if len(missingKeys) > 0 {
				t.Fatalf("❌ Critical Assumptions Missing from Debate: %v. Debate failed to align on drivers.", missingKeys)
			} else {
				t.Logf("✅ All Mandatory Valuation Drivers secured from Debate.")
			}

			// G. Stage 4: Projection (The Financial Engine)
			t.Logf("📈 [Stage 4] Generating Projections (5 Years)...")

			// Initialize Engine (Restored)
			projEngine := projection.NewProjectionEngine(&projection.StandardSkeleton{})

			// Extract Assumptions from Report
			getAssumption := func(key string, defaultVal float64) float64 {
				if val, ok := orch.FinalReport.Assumptions[key]; ok {
					return val.Value
				}
				return defaultVal
			}

			// Calculate Historical Common-Size Ratios for Defaults
			var defaults calc.CommonSizeDefaults
			// Try to get 2024 Actuals
			if hist2024, ok := extractedData.HistoricalData[2024]; ok {
				defaults = calc.CalculateCommonSizeDefaults(&hist2024)
			} else {
				defaults = calc.CalculateCommonSizeDefaults(nil)
			}

			// Map Defaults to Variables
			defaultREV := defaults.RevenueAction
			defaultCOGS := defaults.COGSPercent
			defaultSGA := defaults.SGAPercent
			defaultRD := defaults.RDPercent
			defaultTAX := defaults.TaxRate
			defaultSBC := defaults.StockBasedCompPercent
			defaultINT := defaults.DebtInterestRate

			t.Logf("📊 Calculated Historical Defaults (2024): COGS=%.1f%%, SG&A=%.1f%%, R&D=%.1f%%, Tax=%.1f%%, SBC=%.1f%%, IntCost=%.1f%%",
				defaultCOGS*100, defaultSGA*100, defaultRD*100, defaultTAX*100, defaultSBC*100, defaultINT*100)

			// Map Debate Output to Projection Drivers (Drivers only)
			baseAssumptions := projection.ProjectionAssumptions{
				RevenueGrowth:         getAssumption("rev_growth", defaultREV),
				COGSPercent:           getAssumption("cogs_pct", defaultCOGS),
				SGAPercent:            getAssumption("sga_pct", defaultSGA),
				RDPercent:             getAssumption("rd_pct", defaultRD),
				TaxRate:               getAssumption("tax_rate", defaultTAX),
				StockBasedCompPercent: getAssumption("stock_based_comp_percent", defaultSBC),
				DebtInterestRate:      getAssumption("debt_interest_rate", defaultINT),
				DSO:                   45, // OpEx items usually static in minimal debate
				DSI:                   60,
				DPO:                   60, // Refine DSO/DSI/DPO if Balance Sheet data available
				CapexPercent:          0.05,
				DepreciationPercent:   0.10,
				TerminalGrowth:        getAssumption("terminal_growth", 0.025),
				// New Working Capital Percentage Drivers
				ReceivablesPercent:     defaults.ReceivablesPercent,
				InventoryPercent:       defaults.InventoryPercent,
				AccountsPayablePercent: defaults.APPercent,
				DeferredRevenuePercent: defaults.DeferredRevPercent,
			}

			// Initialize SegmentGrowth map
			baseAssumptions.SegmentGrowth = make(map[string]float64)

			// Dynamic Mapping: Iterate over all assumptions to catch segment drivers
			for key, assumption := range orch.FinalReport.Assumptions {
				// Check for segment growth keys (e.g., "segment_growth_iphone")
				if strings.HasPrefix(key, "segment_growth_") {
					segmentName := strings.TrimPrefix(key, "segment_growth_")
					baseAssumptions.SegmentGrowth[segmentName] = assumption.Value
				}

				// Future: Map other dynamic common-size drivers if they follow a pattern
				// e.g. "driver_*"
			}

			// Valuation Explicit Parameters (Separate from Projection Drivers)
			valWACC := getAssumption("wacc", 0.09)
			valShares := 15000.0   // Static for now, usually from balance sheet or debate
			valNetDebt := -50000.0 // Static for now

			// Run Projection Loop (5 Years)
			var projections []*projection.ProjectedFinancials

			// Start from last actuals (2024 from Golden Record)
			lastActuals := goldenRecord.Timeline[2024]
			prevIS := &lastActuals.IncomeStatement
			prevBS := &lastActuals.BalanceSheet

			// Initial Segments (T=0)
			var prevSegments []edgar.StandardizedSegment
			if extractedData.Qualitative != nil {
				prevSegments = extractedData.Qualitative.Segments.Segments
			}

			for i := 1; i <= 5; i++ {
				targetYear := 2024 + i
				proj := projEngine.ProjectYear(prevIS, prevBS, prevSegments, baseAssumptions, targetYear)
				projections = append(projections, proj)

				// Roll forward for next iteration
				prevIS = proj.IncomeStatement
				prevBS = proj.BalanceSheet
				prevSegments = proj.Segments // Carry forward projected segments
			}
			t.Logf("✅ Generated %d years of projections", len(projections))

			// H. Stage 5: Valuation (The Pricing)
			t.Logf("💲 [Stage 5] Executing DCF Pricing Model...")

			dcfInput := valuation.DCFInput{
				Projections:       projections,
				WACC:              valWACC,
				TerminalGrowth:    baseAssumptions.TerminalGrowth,
				SharesOutstanding: valShares,
				NetDebt:           valNetDebt,
				TaxRate:           baseAssumptions.TaxRate,
			}

			result := valuation.CalculateDCF(dcfInput)

			if result.SharePrice == 0 {
				t.Error("❌ DCF resulted in $0.00 Share Price")
			} else {
				t.Logf("✅ FINAL PRICE TARGET: $%.2f", result.SharePrice)
				t.Logf("   - Enterprise Value: $%.2f M", result.EnterpriseValue)
				t.Logf("   - Equity Value:     $%.2f M", result.EquityValue)
				t.Logf("   - Implied Multiple: %.1fx EBITDA", result.ImpliedMultiple)
			}

			// I. Persistence (Save Artifact)
			// Define Artifact Structs
			type ValuationArtifact struct {
				Ticker           string                             `json:"ticker"`
				Date             string                             `json:"date"`
				SharePrice       float64                            `json:"share_price"`
				ImpliedMultiple  float64                            `json:"implied_exit_multiple"`
				EnterpriseValue  float64                            `json:"enterprise_value"`
				EquityValue      float64                            `json:"equity_value"`
				AgentAssumptions map[string]debate.AssumptionResult `json:"agent_assumptions"`
				ExecutiveSummary string                             `json:"executive_summary"`
				DCFDetails       valuation.DCFResult                `json:"dcf_details_snapshot"`
				// Projections      []*projection.ProjectedFinancials   `json:"projections_snapshot"` // Optional
			}

			artifact := ValuationArtifact{
				Ticker:           "AAPL",
				Date:             time.Now().Format("2006-01-02"),
				SharePrice:       result.SharePrice,
				ImpliedMultiple:  result.ImpliedMultiple,
				EnterpriseValue:  result.EnterpriseValue,
				EquityValue:      result.EquityValue,
				AgentAssumptions: orch.FinalReport.Assumptions,
				ExecutiveSummary: orch.FinalReport.ExecutiveSummary,
				DCFDetails:       result,
			}

			// Marshaling
			jsonData, err := json.MarshalIndent(artifact, "", "  ")
			if err != nil {
				t.Fatalf("Failed to marshal artifact: %v", err)
			}

			// Save to Artifacts Directory
			// We use a relative path for the test output
			outputPath := "valuation_report_apple.json"
			if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
				t.Fatalf("Failed to write artifact: %v", err)
			}

			absPath, _ := filepath.Abs(outputPath)
			t.Logf("💾 Saved Valuation Artifact to: %s", absPath)
			t.Logf("✅ Full Pipeline Test Complete")
		})
	}
}

// TestE2E_IntegratedDebate_Simulation verifies the Stage 3 Debate logic
// using synthesized data, bypassing the need for real API keys or SEC access.
func TestE2E_IntegratedDebate_Simulation(t *testing.T) {
	// 1. Setup Dummy Data (Stage 1 & 2 Output)
	// 1. Setup Dummy Data (Stage 1 & 2 Output)
	rev := 1000000.0
	cogs := 500000.0 // 50% Margin (Default is 60%)
	// In common_size.go, we access .CostOfGoodsSold.Value. If it's nil, we fallback.
	// Let's assume positive for expense in IS?
	// Actually, usually in this system, expenses are often negative, but let's check edgar logic.
	// If CalculateCommonSizeDefaults uses Value, it just divides.
	// Usually COGS is stored as positive or negative.
	// Let's check logic: defaults.COGSPercent = (cogs / revenue).
	// If cogs is negative, percent is negative.
	// But defaults usually positive 0.60.
	// Let's assume Cogs is positive magnitude.

	dummyData := &edgar.FSAPDataResponse{
		Company:    "TestCorp",
		CIK:        "0000000000",
		FiscalYear: 2024,
		IncomeStatement: edgar.IncomeStatement{
			GrossProfitSection: &edgar.GrossProfitSection{
				Revenues:        &edgar.FSAPValue{Value: &rev},
				CostOfGoodsSold: &edgar.FSAPValue{Value: &cogs},
			},
			OperatingCostSection: &edgar.OperatingCostSection{
				SGAExpenses: &edgar.FSAPValue{Value: float64Ptr(150000)},
				RDExpenses:  &edgar.FSAPValue{Value: float64Ptr(50000)},
			},
			TaxAdjustments: &edgar.TaxAdjustmentsSection{
				IncomeTaxExpense: &edgar.FSAPValue{Value: float64Ptr(42000)}, // 21% of 200k
			},
		},
		HistoricalData: map[int]edgar.YearData{
			2024: {
				IncomeStatement: edgar.IncomeStatement{
					GrossProfitSection: &edgar.GrossProfitSection{
						Revenues:        &edgar.FSAPValue{Value: &rev},
						CostOfGoodsSold: &edgar.FSAPValue{Value: &cogs},
					},
					OperatingCostSection: &edgar.OperatingCostSection{
						SGAExpenses: &edgar.FSAPValue{Value: float64Ptr(150000)},
						RDExpenses:  &edgar.FSAPValue{Value: float64Ptr(50000)},
					},
					TaxAdjustments: &edgar.TaxAdjustmentsSection{
						IncomeTaxExpense: &edgar.FSAPValue{Value: float64Ptr(42000)},
					},
				},
			},
		},
	}

	// 2. Setup Orchestrator (Stage 3)
	mgr := &agent.Manager{}
	orch := debate.NewOrchestrator(
		"TEST-DEBATE-SIM", "TEST", "TestCorp", "2024",
		true, // Simulation Mode -> Mock Agents
		debate.ModeAutomatic,
		mgr,
		nil,
	)

	// 3. Inject Data
	orch.SharedContext.MaterialPool = &debate.MaterialPool{
		FinancialHistory: []*edgar.FSAPDataResponse{dummyData},
	}
	// Inject Fallback Consensus (crucial for verifying JSON fallback path)
	orch.SharedContext.CurrentConsensus = map[string]debate.AssumptionDraft{
		"rev_growth": {
			ParameterName:      "Revenue Growth",
			ParentAssumptionID: "rev_growth",
			Value:              10.0,
			Unit:               "%",
			Confidence:         0.9,
			ProposedByAgent:    debate.RoleFundamental,
			Rationale:          "Simulated consensus",
		},
	}

	// 4. Run Debate
	t.Log("🚀 Starting Debate Simulation...")
	orch.Run(context.Background())

	// 5. Verify Output (Stage 4 Handoff)
	if orch.FinalReport == nil {
		t.Fatal("❌ No Final Report generated")
	}
	t.Logf("✅ Final Report Generated (Length: %d)", len(orch.FinalReport.ExecutiveSummary))

	// Verify JSON Assumptions (via Fallback or proper extraction)
	if val, ok := orch.FinalReport.Assumptions["rev_growth"]; ok {
		t.Logf("✅ Validated Assumption Handoff: %.2f%s (Source: %s)", val.Value, val.Unit, val.FinalizedByAgent)
		if val.Value != 10.0 {
			t.Errorf("Expected 10.0, got %.2f", val.Value)
		}
	} else {
		t.Error("❌ Critical Assumption 'rev_growth' lost during synthesis")
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}

// findProjectRoot attempts to find the project root by looking for go.mod
func findProjectRoot(startDir string) string {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return startDir // Hit root, just return start
		}
		dir = parent
	}
}

// Helper to get markdown locally or fetch (Simplified version of integration_all_companies_test)
func fetchOrLoadMarkdown(t *testing.T, company CompanyTestCase) (string, error) {
	// 1. Try Cache in pkg/core/edgar/testdata/cache (Relative path adjustment needed)
	// From tests/e2e, we go up to root, then down.
	// Assuming running from project root: pkg/core/edgar/testdata/cache

	// 2. Decode HTML -> Markdown

	// Assuming standard layout:
	// Relative path to cache directory
	// We can reconstruct it from the known structure.

	// Determine absolute path to project root based on current working directory
	// Assuming test is run from project root or a subdirectory
	wd, _ := os.Getwd()
	projectRoot := findProjectRoot(wd)
	baseDir := filepath.Join(projectRoot, "pkg", "core", "edgar", "testdata", "cache")

	if company.CacheFile == "" {
		// Use a simple safe name generator matching the other test
		safeName := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				return r
			}
			return '_'
		}, company.Name)
		company.CacheFile = fmt.Sprintf("%s_%s.html", safeName, company.CIK)
	}

	cachePath := filepath.Join(baseDir, company.CacheFile)

	// 2. Decode HTML -> Markdown
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Skipf("Cache file not found for %s at %s, skipping.", company.Name, cachePath)
		return "", nil
	}

	t.Logf("📁 Loaded from cache: %s", company.CacheFile)
	htmlContent := string(data)

	// Use PandocAdapter to convert
	converter := edgar.NewPandocAdapter()
	if !converter.IsAvailable() {
		t.Skipf("Pandoc not found in PATH, skipping %s", company.Name)
		return "", nil
	}

	markdown, err := converter.HTMLToMarkdown(htmlContent)
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to Markdown: %v", err)
	}

	return markdown, nil
}

// Helper to truncate long content for logs
func extractSummary(content string) string {
	if len(content) > 100 {
		return content[:100] + "..."
	}
	return content
}

// Helper: Normalize Flat JSON to Nested Sections
func normalizeReport(report *edgar.FSAPDataResponse) {
	is := &report.IncomeStatement
	// 1. IS Normalization
	if is.GrossProfitSection == nil {
		is.GrossProfitSection = &edgar.GrossProfitSection{
			Revenues:        is.Revenues,
			CostOfGoodsSold: is.CostOfGoodsSold,
			GrossProfit:     is.ReportedForValidation.GrossProfit,
		}
	}
	if is.OperatingCostSection == nil {
		is.OperatingCostSection = &edgar.OperatingCostSection{
			SGAExpenses:     is.SGAExpenses,
			RDExpenses:      is.RDExpenses,
			OperatingIncome: is.ReportedForValidation.OperatingIncome,
		}
	} else {
		if is.OperatingCostSection.RDExpenses == nil {
			is.OperatingCostSection.RDExpenses = is.RDExpenses
		}
		if is.OperatingCostSection.SGAExpenses == nil {
			is.OperatingCostSection.SGAExpenses = is.SGAExpenses
		}
	}
	if is.NonOperatingSection == nil {
		is.NonOperatingSection = &edgar.NonOperatingSection{
			InterestExpense: is.InterestExpense,
			IncomeBeforeTax: is.ReportedForValidation.IncomeBeforeTax,
		}
	}
	if is.TaxAdjustments == nil {
		is.TaxAdjustments = &edgar.TaxAdjustmentsSection{
			IncomeTaxExpense: is.IncomeTaxExpense,
		}
	}
	if is.NetIncomeSection == nil {
		is.NetIncomeSection = &edgar.NetIncomeSection{
			NetIncomeToCommon: is.ReportedForValidation.NetIncome,
		}
	}

	// 2. CF Normalization (Legacy -> Nested)
	cf := &report.CashFlowStatement
	if cf.OperatingActivities == nil {
		cf.OperatingActivities = &edgar.CFOperatingSection{
			StockBasedCompensation:   cf.StockBasedCompensation,
			DepreciationAmortization: cf.DepreciationAmortization,
		}
	} else {
		if cf.OperatingActivities.StockBasedCompensation == nil {
			cf.OperatingActivities.StockBasedCompensation = cf.StockBasedCompensation
		}
		if cf.OperatingActivities.DepreciationAmortization == nil {
			cf.OperatingActivities.DepreciationAmortization = cf.DepreciationAmortization
		}
	}
	// ... minimal implementation for test ...
}
