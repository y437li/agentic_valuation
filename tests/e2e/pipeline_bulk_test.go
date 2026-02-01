package e2e_test

import (
	"agentic_valuation/pkg/core/analysis"
	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/ingest"
	"agentic_valuation/pkg/core/llm"
	"agentic_valuation/pkg/core/pipeline"
	"agentic_valuation/pkg/core/store"
	"agentic_valuation/pkg/core/synthesis"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

// Wrapper to satisfy edgar.AIProvider (redefined here since it's not exported)
type BulkTestAIProvider struct {
	provider *llm.DeepSeekProvider
}

func (p *BulkTestAIProvider) Generate(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return p.provider.GenerateResponse(ctx, userPrompt, systemPrompt, map[string]interface{}{})
}

// Mock Analysis Repo
type storeData struct {
	Record   *synthesis.GoldenRecord
	Analysis *analysis.CompanyAnalysis
}

type MockAnalysisRepo struct {
	data map[string]*storeData
}

func NewMockAnalysisRepo() *MockAnalysisRepo {
	return &MockAnalysisRepo{
		data: make(map[string]*storeData),
	}
}

func (m *MockAnalysisRepo) Save(ctx context.Context, record *synthesis.GoldenRecord, anal *analysis.CompanyAnalysis) error {
	m.data[record.Ticker] = &storeData{Record: record, Analysis: anal}
	return nil
}

func (m *MockAnalysisRepo) Load(ctx context.Context, ticker string) (*synthesis.GoldenRecord, *analysis.CompanyAnalysis, error) {
	if d, ok := m.data[ticker]; ok {
		return d.Record, d.Analysis, nil
	}
	return nil, nil, fmt.Errorf("mock: no data for %s", ticker)
}

// BulkRunResult captures the outcome for a single CIK
type BulkRunResult struct {
	Ticker             string   `json:"ticker"`
	CIK                string   `json:"cik"`
	Status             string   `json:"status"` // "Success", "Failed", "Skipped"
	FailureStage       string   `json:"failure_stage,omitempty"`
	ErrorMessage       string   `json:"error_message,omitempty"`
	ValidationWarnings []string `json:"validation_warnings,omitempty"`
}

// BulkTestReport is the final output structure
type BulkTestReport struct {
	Timestamp    time.Time       `json:"timestamp"`
	TotalTickers int             `json:"total_tickers"`
	SuccessCount int             `json:"success_count"`
	FailureCount int             `json:"failure_count"`
	Results      []BulkRunResult `json:"results"`
}

// TestPipelineBulkRunner executes the pipeline for multiple companies to identify edge cases.
// Usage: NUM_TICKERS=5 go test -v -run TestPipelineBulkRunner ./tests/e2e
func TestPipelineBulkRunner(t *testing.T) {
	// 0. Setup
	wd, _ := os.Getwd()
	projectRoot := findProjectRootForBulk(wd)
	if err := godotenv.Load(filepath.Join(projectRoot, ".env")); err != nil {
		t.Log("⚠️ Warning: .env file not found. relying on system env vars.")
	}

	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("⚠️ DEEPSEEK_API_KEY not set. Skipping bulk test.")
	}
	os.Setenv("ENABLE_REAL_SEC_TEST", "true")

	// Init Mock DB
	mockRepo := NewMockAnalysisRepo()

	// 1. Fetch Candidates
	t.Log("Fetching all tickers from SEC...")
	allTickers, err := ingest.FetchAllTickers()
	if err != nil {
		t.Fatalf("Failed to fetch tickers: %v", err)
	}
	t.Logf("Fetched %d tickers.", len(allTickers))

	// 2. Select Candidates
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(allTickers), func(i, j int) { allTickers[i], allTickers[j] = allTickers[j], allTickers[i] })

	// Determine sample size
	sampleSize := 100 // Default as requested
	if os.Getenv("NUM_TICKERS") != "" {
		fmt.Sscanf(os.Getenv("NUM_TICKERS"), "%d", &sampleSize)
	}
	if sampleSize > len(allTickers) {
		sampleSize = len(allTickers)
	}

	selectedTickers := allTickers[:sampleSize]
	t.Logf("Selected %d tickers for testing.", len(selectedTickers))

	// 3. Prepare Components
	cacheDir := filepath.Join(projectRoot, ".cache", "edgar")
	fetcher := ingest.NewSECContentFetcher(cacheDir)
	rawProvider := &llm.DeepSeekProvider{}
	provider := &BulkTestAIProvider{provider: rawProvider} // Reusing wrapper

	// Orchestrator
	orchestrator := pipeline.NewPipelineOrchestrator(fetcher, provider)
	orchestrator.SetRepository(mockRepo) // Inject mock

	orchestrator.SetValidationConfig(pipeline.ValidationConfig{
		EnableStrictValidation: false, // We don't want to abort, we want to record
		BalanceSheetTolerance:  0.1,
		CashFlowTolerance:      0.1,
	})

	// 4. Execution Loop
	var report BulkTestReport
	report.Timestamp = time.Now()
	report.TotalTickers = len(selectedTickers)

	// Ensure report is saved even if test times out or panics at top level
	defer func() {
		reportDir := filepath.Join(projectRoot, "test_output")
		os.MkdirAll(reportDir, 0755)
		reportPath := filepath.Join(reportDir, "bulk_test_report.json")

		file, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(reportPath, file, 0644)

		t.Logf("✅ Bulk Test Report saved to: %s", reportPath)
		t.Logf("Success: %d, Failures: %d", report.SuccessCount, report.FailureCount)
	}()

	for i, tickerInfo := range selectedTickers {
		t.Logf("[%d/%d] Processing %s (CIK: %s)...", i+1, len(selectedTickers), tickerInfo.Ticker, tickerInfo.CIK)

		res := runPipelineAndValidate(t, orchestrator, mockRepo, tickerInfo)
		report.Results = append(report.Results, res)

		if res.Status == "Success" {
			report.SuccessCount++
		} else {
			report.FailureCount++
		}

		// Save intermediate report
		reportDir := filepath.Join(projectRoot, "test_output")
		os.MkdirAll(reportDir, 0755)
		reportPath := filepath.Join(reportDir, "bulk_test_report_intermediate.json")
		file, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(reportPath, file, 0644)

		// Sleep briefly to be nice to APIs
		time.Sleep(2 * time.Second)
	}
}

func runPipelineAndValidate(t *testing.T, orch *pipeline.PipelineOrchestrator, repo store.AnalysisRepository, info ingest.TickerInfo) (res BulkRunResult) {
	res = BulkRunResult{
		Ticker: info.Ticker,
		CIK:    info.CIK,
		Status: "Success",
	}

	defer func() {
		if r := recover(); r != nil {
			res.Status = "Failed (Panic)"
			res.FailureStage = "Panic Recovery"
			res.ErrorMessage = fmt.Sprintf("Panic: %v", r)
		}
	}()

	ctx := context.Background()

	// A. Resolve Filings
	client := ingest.NewEDGARClient()
	companyInfo, err := client.FetchCompanyInfo(info.CIK)
	if err != nil {
		res.Status = "Failed"
		res.FailureStage = "Ingest (Company Info)"
		res.ErrorMessage = err.Error()
		return res
	}

	allFilings := client.GetFilings(companyInfo, []string{"10-K"}, 0)
	var filings []edgar.FilingMetadata
	for _, f := range allFilings {
		// Filter 2020+
		if f.FilingDate.Year() < 2020 {
			continue
		}
		meta := edgar.FilingMetadata{
			CompanyName:     companyInfo.Name,
			CIK:             info.CIK,
			AccessionNumber: f.AccessionNumber,
			Form:            f.FormType,
			FilingDate:      f.FilingDate.Format("2006-01-02"),
		}
		if f.FilingDate.Month() < 6 {
			meta.FiscalYear = f.FilingDate.Year() - 1
		} else {
			meta.FiscalYear = f.FilingDate.Year()
		}
		filings = append(filings, meta)
	}

	if len(filings) == 0 {
		res.Status = "Failed"
		res.FailureStage = "Ingest (No Filings)"
		res.ErrorMessage = "No 10-K filings found since 2020"
		return res
	}

	// B. Run Pipeline
	err = orch.RunForCompany(ctx, info.Ticker, info.CIK, filings)
	if err != nil {
		res.Status = "Failed"
		res.FailureStage = "Pipeline Execution"
		res.ErrorMessage = err.Error()
		return res
	}

	// C. Validate Results
	golden, _, err := repo.Load(ctx, info.Ticker)
	if err != nil {
		res.Status = "Failed"
		res.FailureStage = "Validation (Load)"
		res.ErrorMessage = fmt.Sprintf("Failed to load saved analysis: %v", err)
		return res
	}

	validateGoldenRecord(golden, &res)

	return res
}

func validateGoldenRecord(golden *synthesis.GoldenRecord, res *BulkRunResult) {
	// Iterate over timeline
	var years []int
	for year := range golden.Timeline {
		years = append(years, year)
	}
	sort.Ints(years)

	for _, year := range years {
		snapshot := golden.Timeline[year]
		yearStr := fmt.Sprintf("%d", year)

		// 1. Balance Sheet Check
		bsCheck := calc.CalculateBalanceSheetByYear(&snapshot.BalanceSheet, yearStr)
		if bsCheck != nil {
			checkTolerance(fmt.Sprintf("%d BS Balance", year), bsCheck.BalanceCheck, bsCheck.TotalAssets, 0.1, res)
		} else {
			res.ValidationWarnings = append(res.ValidationWarnings, fmt.Sprintf("%d: Balance Sheet Missing", year))
		}

		// 2. Income Statement Check (Net Income Calc vs Reported)
		isCheck := calc.CalculateIncomeStatementTotalsByYear(&snapshot.IncomeStatement, yearStr)
		if isCheck != nil {
			if isCheck.NetIncomeReported != 0 {
				diff := math.Abs(isCheck.NetIncomeCalc - isCheck.NetIncomeReported)
				checkTolerance(fmt.Sprintf("%d IS Net Income", year), diff, isCheck.NetIncomeReported, 1.0, res) // 1% tolerance
			}
		}

		// 3. Cash Flow Check (Net Change)
		cfCheck := calc.CalculateCashFlowTotalsByYear(&snapshot.CashFlowStatement, yearStr)
		if cfCheck != nil {
			if cfCheck.NetChangeReported != 0 {
				diff := math.Abs(cfCheck.NetChangeCalc - cfCheck.NetChangeReported)
				checkTolerance(fmt.Sprintf("%d CF Net Change", year), diff, cfCheck.NetChangeReported, 0.1, res)
			}
		}
	}

	if len(res.ValidationWarnings) > 0 {
		res.Status = "Failed (Validation)" // Or "Warning"
		res.FailureStage = "Validation Checks"
		res.ErrorMessage = fmt.Sprintf("%d validation issues found", len(res.ValidationWarnings))
	}
}

func checkTolerance(label string, diff float64, base float64, percentThreshold float64, res *BulkRunResult) {
	if base == 0 {
		return
	}
	diffPercent := (math.Abs(diff) / math.Abs(base)) * 100
	if diffPercent > percentThreshold {
		res.ValidationWarnings = append(res.ValidationWarnings,
			fmt.Sprintf("%s mismatch: Diff %.2f (%.2f%%) > %.2f%%", label, diff, diffPercent, percentThreshold))
	}
}

func findProjectRootForBulk(startDir string) string {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return startDir // fallback
		}
		dir = parent
	}
}
