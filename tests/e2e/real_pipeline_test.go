package e2e_test

import (
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/ingest"
	"agentic_valuation/pkg/core/llm"
	"agentic_valuation/pkg/core/pipeline"
	"agentic_valuation/pkg/core/store"
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

// Wrapper for DeepSeek provider to match edgar.AIProvider interface
type TestAIProvider struct {
	provider *llm.DeepSeekProvider
}

func (p *TestAIProvider) Generate(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return p.provider.GenerateResponse(ctx, userPrompt, systemPrompt, map[string]interface{}{})
}

func TestRealPipeline_Subset(t *testing.T) {
	runRealPipelineTest(t, true)
}

func TestRealPipeline_Full(t *testing.T) {
	if os.Getenv("RUN_FULL_SUITE") != "true" {
		t.Skip("Skipping full suite. Set RUN_FULL_SUITE=true to run.")
	}
	runRealPipelineTest(t, false)
}

func runRealPipelineTest(t *testing.T, subsetOnly bool) {
	// 1. Setup Environment
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}

	// Database Setup (Optional)
	dbEnabled := false
	if os.Getenv("DATABASE_URL") != "" {
		if err := store.InitDB(context.Background()); err == nil {
			dbEnabled = true
			t.Log("Database connected for persistence.")
		} else {
			t.Logf("Database connection failed: %v. Proceeding without persistence.", err)
		}
	} else {
		t.Log("DATABASE_URL not set. Persistence disabled (Dry Run).")
	}

	// 2. Select Companies
	companiesToTest := selectCompanies(subsetOnly)
	t.Logf("Selected %d companies for testing.", len(companiesToTest))

	// 3. Initialize Reporter and Tools
	reporter := NewTestReporter()
	aiProvider := &TestAIProvider{provider: &llm.DeepSeekProvider{}}
	client := ingest.NewEDGARClient()

	// 4. Execution Loop
	for _, company := range companiesToTest {
		t.Run(company.Ticker, func(t *testing.T) {
			start := time.Now()
			result := TestResult{
				Ticker:    company.Ticker,
				Industry:  company.Industry,
				Status:    "RUNNING",
				Steps:     []string{},
				Errors:    []string{},
				Timestamp: start,
			}

			defer func() {
				result.Duration = time.Since(start)
				reporter.LogResult(result)
				// Flush report periodically
				reporter.GenerateMarkdownReport("test_report.md")
			}()

			logStep := func(step string) {
				t.Logf("[%s] %s", company.Ticker, step)
				result.Steps = append(result.Steps, fmt.Sprintf("%s - %s", time.Now().Format("15:04:05"), step))
			}

			// Step A: Fetch Filing List
			logStep("Fetching filing list from SEC...")
			info, err := client.FetchCompanyInfo(company.CIK)
			if err != nil {
				result.Status = "FAIL"
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to fetch company info: %v", err))
				return
			}

			// Step B: Filter Filings (10-K, 2020-2025)
			rawFilings := client.GetFilings(info, []string{"10-K"}, 0)

			var targetFilings []edgar.FilingMetadata
			var ingestFilings []ingest.Filing // For the fetcher map

			for _, f := range rawFilings {
				fy := f.ReportDate.Year()
				fyStr := strconv.Itoa(fy)

				// Check if this fiscal year is in our target list
				isTarget := false
				for _, targetFY := range FiscalYears {
					if fyStr == targetFY {
						isTarget = true
						break
					}
				}

				if isTarget {
					meta := mapToMetadata(company, f, fy)
					targetFilings = append(targetFilings, meta)
					ingestFilings = append(ingestFilings, f)
				}
			}

			if len(targetFilings) == 0 {
				logStep("No matching 10-K filings found for target years.")
				result.Status = "SKIP"
				result.Errors = append(result.Errors, "No matching filings found")
				return
			}
			logStep(fmt.Sprintf("Found %d relevant filings.", len(targetFilings)))

			// Step C: Initialize Pipeline
			// We create a new fetcher for each company to keep the map clean,
			// though a global one would work too.
			fetcher := NewRealSECFetcher(ingestFilings)
			orch := pipeline.NewPipelineOrchestrator(fetcher, aiProvider)

			// Step D: Run Pipeline
			logStep("Starting Pipeline Orchestration...")
			ctx := context.Background()
			err = orch.RunForCompany(ctx, company.Ticker, company.CIK, targetFilings)

			if err != nil {
				result.Status = "FAIL"
				result.Errors = append(result.Errors, fmt.Sprintf("Pipeline execution failed: %v", err))
				logStep("Pipeline failed.")
			} else {
				result.Status = "SUCCESS"
				logStep("Pipeline completed successfully.")

				if dbEnabled {
					logStep("Data persisted to Supabase.")
				} else {
					logStep("Persistence skipped (Dry Run).")
				}
			}
		})
	}

	// Final Report Generation
	if err := reporter.GenerateMarkdownReport("test_report.md"); err != nil {
		t.Logf("Failed to write report: %v", err)
	} else {
		t.Logf("Test Report generated at test_report.md")
	}
}

func selectCompanies(subsetOnly bool) []CompanyInfo {
	if !subsetOnly {
		return CompanyUniverse
	}

	// Select one per industry
	selected := make([]CompanyInfo, 0)
	seenIndustries := make(map[string]bool)

	for _, c := range CompanyUniverse {
		if !seenIndustries[c.Industry] {
			selected = append(selected, c)
			seenIndustries[c.Industry] = true
		}
	}
	return selected
}

func mapToMetadata(c CompanyInfo, f ingest.Filing, fiscalYear int) edgar.FilingMetadata {
	return edgar.FilingMetadata{
		CIK:             c.CIK,
		CompanyName:     c.Name,
		Tickers:         []string{c.Ticker},
		AccessionNumber: f.AccessionNumber,
		FilingDate:      f.FilingDate.Format("2006-01-02"),
		Form:            f.FormType,
		FiscalYear:      fiscalYear,
		FiscalPeriod:    "FY", // 10-K is always FY
		PrimaryDocument: f.PrimaryDocument,
		FilingURL:       f.URL,
	}
}
