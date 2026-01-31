package pipeline

import (
	"agentic_valuation/pkg/core/analysis"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/store"
	"agentic_valuation/pkg/core/synthesis"
	"context"
	"fmt"
	"strings"
	"time"
)

// ContentFetcher retrieves the Markdown content for a given SEC filing.
// Implementations may fetch from:
// - Live SEC EDGAR (HTML -> Pandoc -> Markdown)
// - Local cache
// - Supabase knowledge_assets table
type ContentFetcher interface {
	FetchMarkdown(ctx context.Context, cik string, accessionNumber string) (string, error)
}

// PipelineOrchestrator manages the end-to-end data flow:
// v2.0 Architecture: Navigator -> Mapper -> GoExtractor -> Synthesis -> Analysis -> Storage
type PipelineOrchestrator struct {
	fetcher   ContentFetcher
	navigator *edgar.NavigatorAgent
	mapper    *edgar.TableMapperAgent
	extractor *edgar.GoExtractor
	zipper    *synthesis.ZipperEngine
	analyzer  *analysis.AnalysisEngine
	repo      *store.AnalysisRepo
}

// NewPipelineOrchestrator creates a new orchestrator with all required dependencies.
// fetcher: Implementation of ContentFetcher (e.g., SECFetcher, CacheFetcher)
// aiProvider: LLM provider for extraction (e.g., GeminiProvider, DeepSeekProvider)
func NewPipelineOrchestrator(fetcher ContentFetcher, aiProvider edgar.AIProvider) *PipelineOrchestrator {
	return &PipelineOrchestrator{
		fetcher:   fetcher,
		navigator: edgar.NewNavigatorAgent(aiProvider),
		mapper:    edgar.NewTableMapperAgent(aiProvider),
		extractor: edgar.NewGoExtractor(),
		zipper:    synthesis.NewZipperEngine(),
		analyzer:  analysis.NewAnalysisEngine(),
		repo:      store.NewAnalysisRepo(),
	}
}

// statementConfig defines extraction parameters for each statement type
type statementConfig struct {
	name      string
	tableType string
	patterns  []string
}

// RunForCompany executes the full pipeline for a single company.
// ticker: Stock ticker (e.g., "AAPL")
// cik: SEC CIK number (e.g., "0000320193")
// filings: List of filings to process (from SEC submissions API or cache)
func (p *PipelineOrchestrator) RunForCompany(ctx context.Context, ticker string, cik string, filings []edgar.FilingMetadata) error {
	fmt.Printf("Starting v2.0 pipeline for %s (CIK: %s)...\n", ticker, cik)
	start := time.Now()

	var snapshots []synthesis.ExtractionSnapshot

	// 1. Extraction Loop (v2.0: Navigator -> Mapper -> GoExtractor)
	for _, filing := range filings {
		fmt.Printf("Extracting %s (%s)...\n", filing.AccessionNumber, filing.FiscalPeriod)

		// Fetch markdown content for this filing
		markdown, err := p.fetcher.FetchMarkdown(ctx, cik, filing.AccessionNumber)
		if err != nil {
			fmt.Printf("Warning: Failed to fetch content for %s: %v. Skipping.\n", filing.AccessionNumber, err)
			continue
		}

		// Extract using v2.0 Decoupled Architecture
		data, err := p.extractV2(ctx, markdown, &filing)
		if err != nil {
			fmt.Printf("Warning: v2.0 Extraction failed for %s: %v. Skipping.\n", filing.AccessionNumber, err)
			continue
		}

		snapshot := synthesis.ExtractionSnapshot{
			FilingMetadata: synthesis.SourceMetadata{
				AccessionNumber: filing.AccessionNumber,
				FilingDate:      filing.FilingDate,
			},
			FiscalYear: filing.FiscalYear,
			Data:       data,
		}
		snapshots = append(snapshots, snapshot)
	}

	// 2. Synthesis
	if len(snapshots) == 0 {
		return fmt.Errorf("no snapshots extracted for %s, cannot proceed", ticker)
	}

	golden, err := p.zipper.Stitch(ticker, cik, snapshots)
	if err != nil {
		return fmt.Errorf("synthesis failed: %w", err)
	}
	fmt.Printf("Synthesis complete: %d years in timeline\n", len(golden.Timeline))

	// 3. Analysis
	companyAnalysis, err := p.analyzer.Analyze(golden)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}
	fmt.Printf("Analysis complete: Generated %d yearly analyses\n", len(companyAnalysis.Timeline))

	// 4. Storage
	err = p.repo.Save(ctx, golden, companyAnalysis)
	if err != nil {
		return fmt.Errorf("storage failed: %w", err)
	}

	fmt.Printf("Pipeline completed for %s in %v\n", ticker, time.Since(start))
	return nil
}

// extractV2 performs v2.0 Decoupled Extraction: Navigator -> Mapper -> GoExtractor
func (p *PipelineOrchestrator) extractV2(ctx context.Context, markdown string, meta *edgar.FilingMetadata) (*edgar.FSAPDataResponse, error) {
	// Step 1: Parse TOC using NavigatorAgent
	toc, err := p.navigator.ParseTOC(ctx, markdown)
	if err != nil {
		fmt.Printf("Warning: NavigatorAgent.ParseTOC failed: %v\n", err)
		// Continue with fallback patterns
	}

	// Statement configurations with fallback patterns
	statements := []statementConfig{
		{
			name:      "Income_Statement",
			tableType: "income_statement",
			patterns:  []string{"CONSOLIDATED STATEMENTS OF INCOME", "CONSOLIDATED STATEMENTS OF OPERATIONS", "STATEMENTS OF INCOME"},
		},
		{
			name:      "Balance_Sheet",
			tableType: "balance_sheet",
			patterns:  []string{"CONSOLIDATED BALANCE SHEETS", "CONSOLIDATED BALANCE SHEET"},
		},
		{
			name:      "Cash_Flow",
			tableType: "cash_flow",
			patterns:  []string{"CONSOLIDATED STATEMENTS OF CASH FLOWS", "STATEMENTS OF CASH FLOWS"},
		},
	}

	// Add LLM-discovered titles from TOC
	if toc != nil {
		if toc.IncomeStatement != nil && toc.IncomeStatement.Title != "" {
			statements[0].patterns = append([]string{toc.IncomeStatement.Title}, statements[0].patterns...)
		}
		if toc.BalanceSheet != nil && toc.BalanceSheet.Title != "" {
			statements[1].patterns = append([]string{toc.BalanceSheet.Title}, statements[1].patterns...)
		}
		if toc.CashFlow != nil && toc.CashFlow.Title != "" {
			statements[2].patterns = append([]string{toc.CashFlow.Title}, statements[2].patterns...)
		}
	}

	// Result container
	result := &edgar.FSAPDataResponse{
		FiscalYear: meta.FiscalYear,
		Company:    meta.CompanyName,
		CIK:        meta.CIK,
	}

	// Step 2: Extract each statement
	for _, stmt := range statements {
		values, err := p.extractStatement(ctx, markdown, stmt)
		if err != nil {
			fmt.Printf("Warning: %s extraction failed: %v\n", stmt.name, err)
			continue
		}

		// Map values to result structure
		p.mapValuesToResult(result, stmt.tableType, values)
	}

	return result, nil
}

// extractStatement extracts a single financial statement using v2.0 pattern
func (p *PipelineOrchestrator) extractStatement(ctx context.Context, markdown string, stmt statementConfig) ([]*edgar.FSAPValue, error) {
	// Find table position
	startLine := findTableLine(markdown, stmt.patterns)
	if startLine == 0 {
		return nil, fmt.Errorf("%s not found in document", stmt.name)
	}

	// Slice table content (60 lines should cover most tables)
	tableMarkdown := sliceLines(markdown, startLine, startLine+60)

	// Step 2a: TableMapperAgent - LLM identifies row semantics
	mapping, err := p.mapper.MapTable(ctx, stmt.tableType, tableMarkdown)
	if err != nil {
		return nil, fmt.Errorf("MapTable failed: %w", err)
	}

	// Step 2b: GoExtractor - Deterministic value extraction
	parsedTable := p.extractor.ParseMarkdownTableWithOffset(tableMarkdown, stmt.tableType, startLine)
	values := p.extractor.ExtractValues(parsedTable, mapping)

	return values, nil
}

// mapValuesToResult maps extracted values to FSAPDataResponse structure
func (p *PipelineOrchestrator) mapValuesToResult(result *edgar.FSAPDataResponse, tableType string, values []*edgar.FSAPValue) {
	// Use the v2_extractor's mapping logic
	edgar.MapFSAPValuesToResult(result, tableType, values)
	fmt.Printf("  Mapped %d values for %s\n", len(values), tableType)
}

// findTableLine finds the line number where a table starts based on patterns
func findTableLine(markdown string, patterns []string) int {
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		upperLine := strings.ToUpper(line)
		for _, pattern := range patterns {
			if strings.Contains(upperLine, strings.ToUpper(pattern)) {
				return i + 1 // 1-indexed
			}
		}
	}
	return 0
}

// sliceLines extracts a range of lines from markdown
func sliceLines(markdown string, start, end int) string {
	lines := strings.Split(markdown, "\n")
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		return ""
	}
	return strings.Join(lines[start-1:end-1], "\n")
}
