package pipeline

import (
	"agentic_valuation/pkg/core/analysis"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/store"
	"agentic_valuation/pkg/core/synthesis"
	"context"
	"fmt"
	"time"
)

// Interfaces for dependency injection

type Zipper interface {
	Stitch(ticker, cik string, snapshots []synthesis.ExtractionSnapshot) (*synthesis.GoldenRecord, error)
}

type Analyzer interface {
	Analyze(record *synthesis.GoldenRecord) (*analysis.CompanyAnalysis, error)
}

type Repository interface {
	Save(ctx context.Context, record *synthesis.GoldenRecord, anal *analysis.CompanyAnalysis) error
}

type Extractor interface {
	Extract(ctx context.Context, markdown string, provider edgar.AIProvider, meta *edgar.FilingMetadata) (*edgar.FSAPDataResponse, error)
}

type ContentFetcher interface {
	FetchContent(filing edgar.FilingMetadata) (string, error)
}

// Default implementations

type DefaultExtractor struct{}

func (d *DefaultExtractor) Extract(ctx context.Context, markdown string, provider edgar.AIProvider, meta *edgar.FilingMetadata) (*edgar.FSAPDataResponse, error) {
	return edgar.ParallelExtract(ctx, markdown, provider, meta)
}

type DefaultContentFetcher struct{}

func (d *DefaultContentFetcher) FetchContent(filing edgar.FilingMetadata) (string, error) {
	// TODO: Implement actual fetching logic (e.g. from S3 or EDGAR)
	// For now, return empty string or error if not mocked
	return "", fmt.Errorf("content fetching not implemented")
}

// PipelineOrchestrator manages the end-to-end data flow:
// Extraction -> Synthesis -> Analysis -> Storage
type PipelineOrchestrator struct {
	zipper    Zipper
	analyzer  Analyzer
	repo      Repository
	extractor Extractor
	fetcher   ContentFetcher
	provider  edgar.AIProvider
}

// NewPipelineOrchestrator creates a new orchestrator with default implementations.
// Note: This requires a provider to be passed, as we can't default it easily without config.
func NewPipelineOrchestrator(provider edgar.AIProvider) *PipelineOrchestrator {
	return &PipelineOrchestrator{
		zipper:    synthesis.NewZipperEngine(),
		analyzer:  analysis.NewAnalysisEngine(),
		repo:      store.NewAnalysisRepo(),
		extractor: &DefaultExtractor{},
		fetcher:   &DefaultContentFetcher{},
		provider:  provider,
	}
}

// NewPipelineOrchestratorWithDeps allows injecting dependencies for testing.
func NewPipelineOrchestratorWithDeps(
	zipper Zipper,
	analyzer Analyzer,
	repo Repository,
	extractor Extractor,
	fetcher ContentFetcher,
	provider edgar.AIProvider,
) *PipelineOrchestrator {
	return &PipelineOrchestrator{
		zipper:    zipper,
		analyzer:  analyzer,
		repo:      repo,
		extractor: extractor,
		fetcher:   fetcher,
		provider:  provider,
	}
}

// RunForCompany executes the pipeline for a single ticker.
func (p *PipelineOrchestrator) RunForCompany(ctx context.Context, ticker string, cik string, filings []edgar.FilingMetadata) error {
	fmt.Printf("Starting pipeline for %s...\n", ticker)
	start := time.Now()

	var snapshots []synthesis.ExtractionSnapshot

	// 1. Extraction Loop
	for _, filing := range filings {
		fmt.Printf("Extracting %s (%s)...\n", filing.AccessionNumber, filing.FiscalPeriod)

		// Create a local copy of filing for pointer safety if needed
		meta := filing

		// Retrieve markdown content
		fullMarkdown, err := p.fetcher.FetchContent(meta)
		if err != nil {
			fmt.Printf("Error fetching content for %s: %v\n", filing.AccessionNumber, err)
			continue
		}

		data, err := p.extractor.Extract(ctx, fullMarkdown, p.provider, &meta)
		if err != nil {
			fmt.Printf("Error extracting %s: %v\n", filing.AccessionNumber, err)
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
		fmt.Println("Warning: No snapshots extracted. Skipping Synthesis.")
		return fmt.Errorf("no snapshots extracted")
	}

	golden, err := p.zipper.Stitch(ticker, cik, snapshots)
	if err != nil {
		return fmt.Errorf("synthesis failed: %w", err)
	}

	// 3. Analysis
	companyAnalysis, err := p.analyzer.Analyze(golden)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// 4. Storage
	err = p.repo.Save(ctx, golden, companyAnalysis)
	if err != nil {
		return fmt.Errorf("storage failed: %w", err)
	}

	fmt.Printf("Pipeline completed for %s in %v\n", ticker, time.Since(start))
	return nil
}
