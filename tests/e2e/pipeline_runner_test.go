package e2e_test

import (
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/ingest"
	"agentic_valuation/pkg/core/llm"
	"agentic_valuation/pkg/core/pipeline"
	"agentic_valuation/pkg/core/store"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

// Wrapper to satisfy edgar.AIProvider
type TestAIProvider struct {
	provider *llm.DeepSeekProvider
}

func (p *TestAIProvider) Generate(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return p.provider.GenerateResponse(ctx, userPrompt, systemPrompt, map[string]interface{}{})
}

// TestPipelineRunner executes the full pipeline for a target company defined by env var.
// Default: TSLA
// Usage: TARGET_TICKER=AAPL go test -v -run TestPipelineRunner ./tests/e2e
func TestPipelineRunner(t *testing.T) {
	// 0. Load Environment Variables (.env)
	wd, _ := os.Getwd()
	projectRoot := findProjectRoot(wd)
	if err := godotenv.Load(filepath.Join(projectRoot, ".env")); err != nil {
		t.Log("⚠️ Warning: .env file not found. relying on system env vars.")
	}

	// Setup API Keys
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Log("⚠️ DEEPSEEK_API_KEY not set. Test will fail during LLM calls.")
	}
	os.Setenv("ENABLE_REAL_SEC_TEST", "true")

	// Init DB
	if err := store.InitDB(context.Background()); err != nil {
		t.Fatalf("❌ Failed to initialize DB: %v. Check DATABASE_URL in .env", err)
	}

	// Determine Target Ticker
	ticker := os.Getenv("TARGET_TICKER")
	if ticker == "" {
		ticker = "TSLA"
		t.Logf("No TARGET_TICKER set, defaulting to %s", ticker)
	}

	// 1. Initialize Components
	cacheDir := filepath.Join(projectRoot, ".cache", "edgar")

	fetcher := ingest.NewSECContentFetcher(cacheDir)
	rawProvider := &llm.DeepSeekProvider{}
	provider := &TestAIProvider{provider: rawProvider}

	// Create Orchestrator
	orchestrator := pipeline.NewPipelineOrchestrator(fetcher, provider)

	// 2. Resolve CIK and Metadata
	cik, err := ingest.LookupCIKByTicker(ticker)
	if err != nil {
		t.Fatalf("Failed to lookup CIK for ticker %s: %v", ticker, err)
	}
	t.Logf("Resolved %s -> CIK: %s", ticker, cik)

	client := ingest.NewEDGARClient()
	info, err := client.FetchCompanyInfo(cik)
	if err != nil {
		t.Fatalf("Failed to fetch company info for %s: %v", ticker, err)
	}

	// Get ALL 10-K filings (limit=0) and filter for 2020+
	allFilings := client.GetFilings(info, []string{"10-K"}, 0)
	var filings []edgar.FilingMetadata

	t.Logf("Found %d total 10-K filings. Filtering for 2020+...", len(allFilings))

	for _, f := range allFilings {
		// Filter: Keep only filings from 2020 onwards
		if f.FilingDate.Year() < 2020 {
			continue
		}

		t.Logf("  + Including: %s (Date: %s)", f.AccessionNumber, f.FilingDate.Format("2006-01-02"))

		meta := edgar.FilingMetadata{
			CompanyName:     info.Name,
			CIK:             cik,
			AccessionNumber: f.AccessionNumber,
			Form:            f.FormType,
			FilingDate:      f.FilingDate.Format("2006-01-02"),
		}

		// fiscal year logic
		if f.FilingDate.Month() < 6 {
			meta.FiscalYear = f.FilingDate.Year() - 1
		} else {
			meta.FiscalYear = f.FilingDate.Year()
		}

		filings = append(filings, meta)
	}

	if len(filings) == 0 {
		t.Fatalf("No 10-K filings found for %s since 2020", ticker)
	}

	// 3. Run Pipeline
	ctx := context.Background()
	// Execute RunForCompany
	err = orchestrator.RunForCompany(ctx, ticker, cik, filings)
	if err != nil {
		t.Fatalf("❌ Pipeline execution failed for %s: %v", ticker, err)
	}

	t.Logf("✅ Pipeline Test Completed Successfully for %s", ticker)
}

func findProjectRoot(startDir string) string {
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
