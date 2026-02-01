package pipeline

import (
	"agentic_valuation/pkg/core/analysis"
	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/store"
	"agentic_valuation/pkg/core/synthesis"
	"context"
	"fmt"
	"math"
	"sort"
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

// ValidationConfig defines thresholds and behavior for Stage 2 Validation
type ValidationConfig struct {
	EnableStrictValidation bool    // If true, validation errors stop the pipeline
	BalanceSheetTolerance  float64 // Allowed gap for A = L + E (default 0.01)
	CashFlowTolerance      float64 // Allowed gap for Net Change check (default 0.01)
}

// PipelineOrchestrator manages the end-to-end data flow:
// v2.0 Architecture: V2Extractor (Navigator->Mapper->GoExtractor) -> Synthesis -> Analysis -> Storage
type PipelineOrchestrator struct {
	fetcher          ContentFetcher
	v2Extractor      *edgar.V2Extractor
	zipper           *synthesis.ZipperEngine
	analyzer         *analysis.AnalysisEngine
	repo             store.AnalysisRepository
	validationConfig ValidationConfig
}

// NewPipelineOrchestrator creates a new orchestrator with all required dependencies.
// fetcher: Implementation of ContentFetcher (e.g., SECFetcher, CacheFetcher)
// aiProvider: LLM provider for extraction (e.g., GeminiProvider, DeepSeekProvider)
func NewPipelineOrchestrator(fetcher ContentFetcher, aiProvider edgar.AIProvider) *PipelineOrchestrator {
	return &PipelineOrchestrator{
		fetcher:     fetcher,
		v2Extractor: edgar.NewV2Extractor(aiProvider),
		zipper:      synthesis.NewZipperEngine(),
		analyzer:    analysis.NewAnalysisEngine(),
		repo:        store.NewAnalysisRepo(),
		validationConfig: ValidationConfig{
			EnableStrictValidation: false, // Default: Log warnings but proceed
			BalanceSheetTolerance:  0.1,   // Default: Allow small rounding differences
			CashFlowTolerance:      0.1,
		},
	}
}

// SetRepository allows injecting a custom repository (e.g., for testing).
func (p *PipelineOrchestrator) SetRepository(repo store.AnalysisRepository) {
	p.repo = repo
}

// SetValidationConfig updates the validation configuration
func (p *PipelineOrchestrator) SetValidationConfig(config ValidationConfig) {
	p.validationConfig = config
}

// RunForCompany executes the full pipeline for a single company.
// ticker: Stock ticker (e.g., "AAPL")
// cik: SEC CIK number (e.g., "0000320193")
// filings: List of filings to process (from SEC submissions API or cache)
// RunForCompany executes the full pipeline for a single company.
// ticker: Stock ticker (e.g., "AAPL")
// cik: SEC CIK number (e.g., "0000320193")
// filings: List of filings to process (from SEC submissions API or cache)
func (p *PipelineOrchestrator) RunForCompany(ctx context.Context, ticker string, cik string, filings []edgar.FilingMetadata) error {
	fmt.Printf("Starting v2.0 pipeline for %s (CIK: %s)...\n", ticker, cik)
	start := time.Now()

	// 0. Smart Ingestion: Check Existing Data
	var existingRecord *synthesis.GoldenRecord
	existingRecord, _, err := p.repo.Load(ctx, ticker)
	existingAccessions := make(map[string]bool)

	if err == nil && existingRecord != nil {
		fmt.Printf("Found existing analysis for %s. Checking for new filings...\n", ticker)
		for _, period := range existingRecord.Timeline {
			existingAccessions[period.SourceFiling.AccessionNumber] = true
		}
	} else {
		// Log the error if it's not just "not found", otherwise just proceed
		fmt.Printf("No existing analysis found for %s (or scan error). Performing full extraction.\n", ticker)
	}

	var filingsToProcess []edgar.FilingMetadata
	for _, filing := range filings {
		if !existingAccessions[filing.AccessionNumber] {
			filingsToProcess = append(filingsToProcess, filing)
		} else {
			fmt.Printf("Skipping %s (Already processed)\n", filing.AccessionNumber)
		}
	}

	if len(filingsToProcess) == 0 {
		fmt.Printf("All %d filings are up to date. No new extraction needed.\n", len(filings))
		return nil
	}

	fmt.Printf("Queueing %d new filings for extraction...\n", len(filingsToProcess))

	// 1. Extraction Loop (v2.0: Navigator -> Mapper -> GoExtractor)
	var snapshots []synthesis.ExtractionSnapshot
	for _, filing := range filingsToProcess {
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

		// [Validation] Run Stage 2 validation checks
		p.validateExtraction(data, filing.AccessionNumber)

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

	// 2. Synthesis (Smart Merge)
	if len(snapshots) == 0 {
		return fmt.Errorf("no snapshots extracted for %s, cannot proceed", ticker)
	}

	var golden *synthesis.GoldenRecord
	if existingRecord != nil {
		// Incremental Update: Merge new snapshots into existing record
		p.zipper.MergeSnapshots(existingRecord, snapshots)
		golden = existingRecord
		fmt.Printf("Merged %d new snapshots into existing GoldenRecord.\n", len(snapshots))
	} else {
		// Fresh Synthesis
		golden, err = p.zipper.Stitch(ticker, cik, snapshots)
		if err != nil {
			return fmt.Errorf("synthesis failed: %w", err)
		}
		fmt.Printf("Fresh synthesis complete: %d years in timeline\n", len(golden.Timeline))
	}

	// --- NEW: Post-Synthesis Validation ---
	p.validateSynthesis(golden)
	// --- END NEW ---

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

// extractV2 performs v2.0 Decoupled Extraction using the encapsulated V2Extractor
func (p *PipelineOrchestrator) extractV2(ctx context.Context, markdown string, meta *edgar.FilingMetadata) (*edgar.FSAPDataResponse, error) {
	return p.v2Extractor.Extract(ctx, markdown, meta)
}

// validateExtraction runs accounting integrity checks on the extracted data
// and logs warnings or errors based on ValidationConfig.
// validateExtraction validates a single extraction response.
func (p *PipelineOrchestrator) validateExtraction(data *edgar.FSAPDataResponse, accessionNumber string) {
	fmt.Printf("\n--- [Stage 2] Validation for %s (%d) ---\n", accessionNumber, data.FiscalYear)
	p.validateFinancials(data.FiscalYear, &data.BalanceSheet, &data.IncomeStatement, &data.CashFlowStatement, accessionNumber)
}

// validateSynthesis validates the synthesized Golden Record.
func (p *PipelineOrchestrator) validateSynthesis(golden *synthesis.GoldenRecord) {
	fmt.Printf("\n--- [Stage 2.5] Post-Synthesis Validation for %s ---\n", golden.Ticker)

	// Sort years for consistent output
	var years []int
	for year := range golden.Timeline {
		years = append(years, year)
	}
	sort.Ints(years)

	for _, year := range years {
		snapshot := golden.Timeline[year]
		fmt.Printf("Validating Synthesized Year %d (Source: %s)...\n", year, snapshot.SourceFiling.AccessionNumber)
		p.validateFinancials(year, &snapshot.BalanceSheet, &snapshot.IncomeStatement, &snapshot.CashFlowStatement, "SYNTHESIS")
	}
}

// validateFinancials runs accounting integrity checks on financial statements.
func (p *PipelineOrchestrator) validateFinancials(year int, bs *edgar.BalanceSheet, is *edgar.IncomeStatement, cf *edgar.CashFlowStatement, contextLabel string) {
	yearStr := fmt.Sprintf("%d", year)

	// --- A. Balance Sheet Checks (Assets = Liabilities + Equity) ---
	bsResult := calc.CalculateBalanceSheetByYear(bs, yearStr)
	if bsResult != nil {
		diff := math.Abs(bsResult.BalanceCheck)
		var diffPercent float64
		if bsResult.TotalAssets > 0 {
			diffPercent = (diff / bsResult.TotalAssets) * 100
		}

		fmt.Printf("  [BS] Assets: %.2f | L+E: %.2f | Diff: %.2f (%.4f%%)\n",
			bsResult.TotalAssets, bsResult.TotalLiabilities+bsResult.TotalEquity, bsResult.BalanceCheck, diffPercent)

		p.checkTolerance("Balance Sheet Equation", diffPercent, diff, p.validationConfig.BalanceSheetTolerance)
	}

	// --- B. Income Statement Checks (Flow-through Validation) ---
	isResult := calc.CalculateIncomeStatementTotalsByYear(is, yearStr)
	if isResult != nil {
		// 1. Gross Profit
		if isResult.GrossProfitReported != 0 {
			gpDiff := math.Abs(isResult.GrossProfitCalc - isResult.GrossProfitReported)
			gpPercent := (gpDiff / math.Abs(isResult.GrossProfitReported)) * 100
			fmt.Printf("  [IS] Gross Profit Calc: %.2f | Rep: %.2f | Diff: %.2f (%.2f%%)\n",
				isResult.GrossProfitCalc, isResult.GrossProfitReported, gpDiff, gpPercent)
			p.checkTolerance("Gross Profit", gpPercent, gpDiff, 1.0)
		}

		// 2. Operating Income
		if isResult.OperatingIncomeReported != 0 {
			opDiff := math.Abs(isResult.OperatingIncomeCalc - isResult.OperatingIncomeReported)
			opPercent := (opDiff / math.Abs(isResult.OperatingIncomeReported)) * 100
			fmt.Printf("  [IS] Op Income Calc: %.2f | Rep: %.2f | Diff: %.2f (%.2f%%)\n",
				isResult.OperatingIncomeCalc, isResult.OperatingIncomeReported, opDiff, opPercent)
			p.checkTolerance("Operating Income", opPercent, opDiff, 1.0)
		}

		// 3. Net Income
		if isResult.NetIncomeReported != 0 {
			niDiff := math.Abs(isResult.NetIncomeCalc - isResult.NetIncomeReported)
			niPercent := (niDiff / math.Abs(isResult.NetIncomeReported)) * 100
			fmt.Printf("  [IS] Net Income Calc: %.2f | Rep: %.2f | Diff: %.2f (%.2f%%)\n",
				isResult.NetIncomeCalc, isResult.NetIncomeReported, niDiff, niPercent)
			p.checkTolerance("Net Income", niPercent, niDiff, 1.0)
		}
	}

	// --- C. Cash Flow Checks (Section Totals & Roll-forward) ---
	cfResult := calc.CalculateCashFlowTotalsByYear(cf, yearStr)
	if cfResult != nil {
		// 1. Operating Activities
		if cfResult.OperatingReported != 0 {
			diff := math.Abs(cfResult.OperatingCalc - cfResult.OperatingReported)
			pct := (diff / math.Abs(cfResult.OperatingReported)) * 100
			fmt.Printf("  [CF] Operating Calc: %.2f | Rep: %.2f | Diff: %.2f (%.2f%%)\n",
				cfResult.OperatingCalc, cfResult.OperatingReported, diff, pct)
			p.checkTolerance("CF Operating", pct, diff, p.validationConfig.CashFlowTolerance)
		}

		// 2. Investing Activities
		if cfResult.InvestingReported != 0 {
			diff := math.Abs(cfResult.InvestingCalc - cfResult.InvestingReported)
			pct := (diff / math.Abs(cfResult.InvestingReported)) * 100
			fmt.Printf("  [CF] Investing Calc: %.2f | Rep: %.2f | Diff: %.2f (%.2f%%)\n",
				cfResult.InvestingCalc, cfResult.InvestingReported, diff, pct)
			p.checkTolerance("CF Investing", pct, diff, p.validationConfig.CashFlowTolerance)
		}

		// 3. Financing Activities
		if cfResult.FinancingReported != 0 {
			diff := math.Abs(cfResult.FinancingCalc - cfResult.FinancingReported)
			pct := (diff / math.Abs(cfResult.FinancingReported)) * 100
			fmt.Printf("  [CF] Financing Calc: %.2f | Rep: %.2f | Diff: %.2f (%.2f%%)\n",
				cfResult.FinancingCalc, cfResult.FinancingReported, diff, pct)
			p.checkTolerance("CF Financing", pct, diff, p.validationConfig.CashFlowTolerance)
		}

		// 4. Net Change in Cash (Equation Check)
		if cfResult.NetChangeReported != 0 {
			diff := math.Abs(cfResult.NetChangeCalc - cfResult.NetChangeReported)
			pct := (diff / math.Abs(cfResult.NetChangeReported)) * 100
			fmt.Printf("  [CF] Net Change Calc: %.2f | Rep: %.2f | Diff: %.2f (%.2f%%)\n",
				cfResult.NetChangeCalc, cfResult.NetChangeReported, diff, pct)
			p.checkTolerance("CF Net Change", pct, diff, p.validationConfig.CashFlowTolerance)
		}

		// 5. Cash Roll-forward (Beg + Change = Ending)
		if cfResult.CashEnding != 0 {
			rollDiff := math.Abs(cfResult.CashEndingCalc - cfResult.CashEnding)
			rollPct := (rollDiff / math.Abs(cfResult.CashEnding)) * 100
			fmt.Printf("  [CF] Cash Ending Calc: %.2f | Rep: %.2f | Diff: %.2f (%.2f%%)\n",
				cfResult.CashEndingCalc, cfResult.CashEnding, rollDiff, rollPct)
			p.checkTolerance("CF Roll-forward", rollPct, rollDiff, p.validationConfig.CashFlowTolerance)
		}
	}
}

// checkTolerance is a helper to log validation results
func (p *PipelineOrchestrator) checkTolerance(label string, diffPercent float64, absoluteDiff float64, tolerance float64) {
	if diffPercent > tolerance {
		msg := fmt.Sprintf("%s mismatch > %.2f%% tolerance (Diff: %.2f)", label, tolerance, absoluteDiff)
		if p.validationConfig.EnableStrictValidation {
			fmt.Printf("    ❌ CRITICAL: %s\n", msg)
		} else {
			fmt.Printf("    ⚠️ WARNING: %s\n", msg)
		}
	} else {
		fmt.Printf("    ✅ %s Valid\n", label)
	}
}
