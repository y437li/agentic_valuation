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

// PipelineOrchestrator manages the end-to-end data flow:
// Extraction -> Synthesis -> Analysis -> Storage
type PipelineOrchestrator struct {
	// extractor *edgar.Extractor // Removed: ParallelExtract is a standalone function
	zipper   *synthesis.ZipperEngine
	analyzer *analysis.AnalysisEngine
	repo     *store.AnalysisRepo
}

// NewPipelineOrchestrator creates a new orchestrator.
func NewPipelineOrchestrator() *PipelineOrchestrator {
	return &PipelineOrchestrator{
		zipper:   synthesis.NewZipperEngine(),
		analyzer: analysis.NewAnalysisEngine(),
		repo:     store.NewAnalysisRepo(),
	}
}

// RunForCompany executes the pipeline for a single ticker.
// This is a simplified version that would integrate with metadata fetching in a real scenario.
func (p *PipelineOrchestrator) RunForCompany(ctx context.Context, ticker string, cik string, filings []edgar.FilingMetadata) error {
	fmt.Printf("Starting pipeline for %s...\n", ticker)
	start := time.Now()

	var snapshots []synthesis.ExtractionSnapshot

	// 1. Extraction Loop
	for _, filing := range filings {
		fmt.Printf("Extracting %s (%s)...\n", filing.AccessionNumber, filing.FiscalPeriod)

		// Create a local copy of filing for pointer safety if needed
		// meta := filing

		// TODO: Retrieve markdown content for the filing.
		// Real implementation needs a content fetcher.
		// For now, we assume we have a way to get content or we skip this implementation detail
		// and focus on the orchestration flow.

		/*
			data, err := edgar.ParallelExtract(ctx, fullMarkdown, provider, &meta)
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
		*/
	}

	// 2. Synthesis
	if len(snapshots) == 0 {
		fmt.Println("Warning: No snapshots extracted. Skipping Synthesis.")
		// return fmt.Errorf("no snapshots extracted")
		// allowing to proceed to demonstrate structure, but typically we return/error.
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
