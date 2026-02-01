package e2e_test

import (
	"agentic_valuation/pkg/core/edgar/converter"
	"agentic_valuation/pkg/core/ingest"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// RealSECFetcher implements pipeline.ContentFetcher using real SEC EDGAR calls.
type RealSECFetcher struct {
	client     *http.Client
	filingMap  map[string]ingest.Filing // Accession -> Filing
	mu         sync.RWMutex
	converter  *converter.PandocAdapter
}

// NewRealSECFetcher creates a new fetcher with a pre-populated map of filings.
func NewRealSECFetcher(filings []ingest.Filing) *RealSECFetcher {
	fMap := make(map[string]ingest.Filing)
	for _, f := range filings {
		fMap[f.AccessionNumber] = f
	}

	return &RealSECFetcher{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		filingMap: fMap,
		converter: converter.NewPandocAdapter(),
	}
}

// FetchMarkdown retrieves the content for a given filing.
// It looks up the filing URL from the internal map using the accession number.
func (f *RealSECFetcher) FetchMarkdown(ctx context.Context, cik string, accessionNumber string) (string, error) {
	f.mu.RLock()
	filing, ok := f.filingMap[accessionNumber]
	f.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("filing not found in local cache for accession %s", accessionNumber)
	}

	// 1. Download HTML
	htmlContent, err := f.downloadHTML(ctx, filing.URL)
	if err != nil {
		return "", fmt.Errorf("failed to download HTML from %s: %w", filing.URL, err)
	}

	// 2. Convert to Markdown
	if !f.converter.IsAvailable() {
		return "", fmt.Errorf("pandoc is not available")
	}

	markdown, err := f.converter.HTMLToMarkdown(htmlContent)
	if err != nil {
		return "", fmt.Errorf("HTML to Markdown conversion failed: %w", err)
	}

	return markdown, nil
}

func (f *RealSECFetcher) downloadHTML(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	// SEC requires User-Agent
	req.Header.Set("User-Agent", ingest.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SEC returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}


// TestReporter collects and generates test results.
type TestReporter struct {
	mu      sync.Mutex
	results []TestResult
}

type TestResult struct {
	Ticker    string
	Industry  string
	Status    string // SUCCESS, FAIL, SKIP
	Steps     []string
	Errors    []string
	Duration  time.Duration
	Timestamp time.Time
}

func NewTestReporter() *TestReporter {
	return &TestReporter{
		results: make([]TestResult, 0),
	}
}

func (r *TestReporter) LogResult(res TestResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, res)
}

func (r *TestReporter) GenerateMarkdownReport(filepath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# Real Data Pipeline Test Report\n\n")
	sb.WriteString(fmt.Sprintf("**Date:** %s\n\n", time.Now().Format(time.RFC1123)))

	// Summary
	total := len(r.results)
	success := 0
	failed := 0
	for _, res := range r.results {
		if res.Status == "SUCCESS" {
			success++
		} else if res.Status == "FAIL" {
			failed++
		}
	}

	sb.WriteString("## Summary\n")
	sb.WriteString(fmt.Sprintf("- **Total Companies:** %d\n", total))
	sb.WriteString(fmt.Sprintf("- **Success:** %d\n", success))
	sb.WriteString(fmt.Sprintf("- **Failed:** %d\n", failed))
	sb.WriteString(fmt.Sprintf("- **Skipped:** %d\n\n", total-success-failed))

	// Detailed Table
	sb.WriteString("## Detailed Results\n")
	sb.WriteString("| Ticker | Industry | Status | Duration | Errors |\n")
	sb.WriteString("|---|---|---|---|---|\n")

	for _, res := range r.results {
		errStr := "-"
		if len(res.Errors) > 0 {
			errStr = strings.ReplaceAll(strings.Join(res.Errors, "; "), "|", "\\|")
			if len(errStr) > 50 {
				errStr = errStr[:47] + "..."
			}
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			res.Ticker, res.Industry, res.Status, res.Duration.Round(time.Second), errStr))
	}

	// Logs Section
	sb.WriteString("\n## Execution Logs\n")
	for _, res := range r.results {
		sb.WriteString(fmt.Sprintf("### %s (%s)\n", res.Ticker, res.Status))
		sb.WriteString(fmt.Sprintf("- **Duration:** %v\n", res.Duration))
		if len(res.Errors) > 0 {
			sb.WriteString("**Errors:**\n")
			for _, e := range res.Errors {
				sb.WriteString(fmt.Sprintf("- %s\n", e))
			}
		}
		if len(res.Steps) > 0 {
			sb.WriteString("**Steps:**\n")
			for _, s := range res.Steps {
				sb.WriteString(fmt.Sprintf("- %s\n", s))
			}
		}
		sb.WriteString("\n")
	}

	return os.WriteFile(filepath, []byte(sb.String()), 0644)
}
