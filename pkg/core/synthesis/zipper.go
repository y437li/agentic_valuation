// Package synthesis implements the "Zipper" algorithm for time-series data synthesis.
// This package merges financial data from multiple SEC filings (10-K, 10-K/A, 10-Q)
// into a single, authoritative "Golden Record" per company.
//
// Core Philosophy: Decoupled from Extraction.
//   - Extraction (ETL) produces immutable, atomic snapshots (one per filing).
//   - Synthesis (this package) is a mutable, recomputed "view" that merges those snapshots.
//
// The Zipper algorithm prioritizes:
//  1. Amendment Dominance: 10-K/A always wins over 10-K for the same fiscal year.
//  2. Recency Bias: For the same line item, data from the *latest* filing wins.
//  3. Restatement Detection: When a newer filing provides a *different* value for a past
//     year, it's logged as a Restatement for audit and agent review.
package synthesis

import (
	"agentic_valuation/pkg/core/edgar"
	"fmt"
	"sort"
	"time"
)

// =============================================================================
// CORE DATA STRUCTURES
// =============================================================================

// GoldenRecord represents the synthesized, authoritative time-series for a single company.
// This is the output of the Zipper engine.
type GoldenRecord struct {
	Ticker       string                  `json:"ticker"`
	CIK          string                  `json:"cik"`
	LastUpdated  time.Time               `json:"last_updated"`
	Timeline     map[int]*YearlySnapshot `json:"timeline"` // Key: Fiscal Year (e.g., 2023)
	Restatements []RestatementLog        `json:"restatements"`
}

// YearlySnapshot contains the final, authoritative financial data for a single fiscal year.
type YearlySnapshot struct {
	FiscalYear        int                     `json:"fiscal_year"`
	BalanceSheet      edgar.BalanceSheet      `json:"balance_sheet"`
	IncomeStatement   edgar.IncomeStatement   `json:"income_statement"`
	CashFlowStatement edgar.CashFlowStatement `json:"cash_flow_statement"`
	SupplementalData  edgar.SupplementalData  `json:"supplemental_data"`
	SourceFiling      SourceMetadata          `json:"source_filing"` // Which filing provided this data
	Completeness      float64                 `json:"completeness"`  // 0-1 coverage ratio
}

// SourceMetadata identifies the origin of a piece of data.
type SourceMetadata struct {
	AccessionNumber string `json:"accession_number"`
	FilingDate      string `json:"filing_date"`
	Form            string `json:"form"` // "10-K", "10-K/A"
	IsAmended       bool   `json:"is_amended"`
}

// RestatementLog records a detected restatement/revision.
type RestatementLog struct {
	Year         int       `json:"year"`
	Item         string    `json:"item"` // e.g., "Revenue"
	OldValue     float64   `json:"old_value"`
	NewValue     float64   `json:"new_value"`
	DeltaPercent float64   `json:"delta_percent"` // e.g., -5.2%
	DetectedAt   time.Time `json:"detected_at"`
	OldSource    string    `json:"old_source"` // Accession of old filing
	NewSource    string    `json:"new_source"` // Accession of new filing
}

// ExtractionSnapshot represents a single atomic extraction from one SEC filing.
// This is the input to the Zipper engine.
type ExtractionSnapshot struct {
	FilingMetadata SourceMetadata          `json:"filing_metadata"`
	FiscalYear     int                     `json:"fiscal_year"` // The "primary" year of the filing (e.g., 2024 for a 2024 10-K)
	Data           *edgar.FSAPDataResponse `json:"data"`
}

// =============================================================================
// ZIPPER ENGINE
// =============================================================================

// ZipperEngine is the main synthesizer.
type ZipperEngine struct {
	// Configuration for conflict resolution
	OutlierThreshold float64 // Percentage change threshold for "Outlier Guard" (e.g., 0.5 = 50%)
}

// NewZipperEngine creates a new ZipperEngine with default settings.
func NewZipperEngine() *ZipperEngine {
	return &ZipperEngine{
		OutlierThreshold: 0.5, // Default: 50% change triggers outlier detection
	}
}

// Stitch merges multiple ExtractionSnapshots into a single GoldenRecord.
// The snapshots MUST be sorted by FilingDate ASCENDING (oldest first) before calling.
// This ensures the "recency bias" rule: later filings overwrite earlier ones.
func (z *ZipperEngine) Stitch(ticker, cik string, snapshots []ExtractionSnapshot) (*GoldenRecord, error) {
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots provided")
	}

	// Sort by filing date (oldest first for correct overwrite logic)
	sortedSnapshots := make([]ExtractionSnapshot, len(snapshots))
	copy(sortedSnapshots, snapshots)
	sort.Slice(sortedSnapshots, func(i, j int) bool {
		return sortedSnapshots[i].FilingMetadata.FilingDate < sortedSnapshots[j].FilingMetadata.FilingDate
	})

	record := &GoldenRecord{
		Ticker:       ticker,
		CIK:          cik,
		LastUpdated:  time.Now(),
		Timeline:     make(map[int]*YearlySnapshot),
		Restatements: []RestatementLog{},
	}

	// Process each snapshot in chronological order
	for _, snap := range sortedSnapshots {
		z.mergeSnapshot(record, &snap)
	}

	return record, nil
}

// mergeSnapshot integrates a single ExtractionSnapshot into the GoldenRecord.
// It handles multi-year data within a single filing (Comparative Columns).
func (z *ZipperEngine) mergeSnapshot(record *GoldenRecord, snap *ExtractionSnapshot) {
	data := snap.Data
	if data == nil {
		return
	}

	// Extract all years present in the filing's data
	yearsInFiling := z.findAllYears(data)

	for _, year := range yearsInFiling {
		// Check if this filing should supersede existing data for this year
		existing, hasExisting := record.Timeline[year]

		if !hasExisting || z.shouldSupersede(existing.SourceFiling, snap.FilingMetadata) {
			// Create a new YearlySnapshot from this filing's data for the given year
			newSnapshot := z.extractYearSlice(data, year, snap.FilingMetadata)

			// Detect restatements if we're overwriting
			if hasExisting {
				z.detectRestatements(record, existing, newSnapshot, year, snap.FilingMetadata.AccessionNumber)
			}

			record.Timeline[year] = newSnapshot
		}
	}
}

// shouldSupersede determines if a new filing should replace the existing one.
// Rule: 10-K/A always wins. Otherwise, newer filing date wins.
func (z *ZipperEngine) shouldSupersede(existing, incoming SourceMetadata) bool {
	// Amendment dominance: 10-K/A > 10-K
	if incoming.IsAmended && !existing.IsAmended {
		return true
	}
	if !incoming.IsAmended && existing.IsAmended {
		return false // Existing amended data wins
	}
	// Recency bias: newer filing date wins
	return incoming.FilingDate > existing.FilingDate
}

// findAllYears discovers all fiscal years present in the extracted data.
// It scans the `Years` map of key fields to find all year keys.
func (z *ZipperEngine) findAllYears(data *edgar.FSAPDataResponse) []int {
	yearSet := make(map[int]bool)

	// Helper to extract years from FSAPValue
	addYears := func(v *edgar.FSAPValue) {
		if v == nil {
			return
		}
		for y := range v.Years {
			yearInt := parseYear(y)
			if yearInt > 0 {
				yearSet[yearInt] = true
			}
		}
	}

	// Check Income Statement revenue
	if data.IncomeStatement.GrossProfitSection != nil {
		addYears(data.IncomeStatement.GrossProfitSection.Revenues)
	}

	// Check Balance Sheet total assets
	addYears(data.BalanceSheet.ReportedForValidation.TotalAssets)
	addYears(data.BalanceSheet.CurrentAssets.CashAndEquivalents)

	// Check Cash Flow Statement (often has 3 years)
	if data.CashFlowStatement.OperatingActivities != nil {
		addYears(data.CashFlowStatement.OperatingActivities.NetIncomeStart)
	}
	if data.CashFlowStatement.CashSummary != nil {
		addYears(data.CashFlowStatement.CashSummary.NetCashOperating)
	}

	// Also include the primary fiscal year from metadata
	if data.FiscalYear > 0 {
		yearSet[data.FiscalYear] = true
	}

	years := make([]int, 0, len(yearSet))
	for y := range yearSet {
		years = append(years, y)
	}
	sort.Ints(years)
	return years
}

// extractYearSlice creates a YearlySnapshot by pulling the data for a specific year
// from an FSAPDataResponse that may contain multiple years.
func (z *ZipperEngine) extractYearSlice(data *edgar.FSAPDataResponse, year int, source SourceMetadata) *YearlySnapshot {
	yearStr := fmt.Sprintf("%d", year)

	snapshot := &YearlySnapshot{
		FiscalYear:   year,
		SourceFiling: source,
	}

	// Deep copy and slice to specific year
	// This is a simplified version; a full implementation would slice every field.
	snapshot.BalanceSheet = sliceBalanceSheet(data.BalanceSheet, yearStr)
	snapshot.IncomeStatement = sliceIncomeStatement(data.IncomeStatement, yearStr)
	snapshot.CashFlowStatement = sliceCashFlowStatement(data.CashFlowStatement, yearStr)
	snapshot.SupplementalData = data.SupplementalData // TODO: Slice this too

	// Calculate completeness
	snapshot.Completeness = z.calculateCompleteness(snapshot)

	return snapshot
}

// detectRestatements compares old and new data to find value changes.
func (z *ZipperEngine) detectRestatements(
	record *GoldenRecord,
	old, new *YearlySnapshot,
	year int,
	newSource string,
) {
	// Compare key figures (simplified; expand for full coverage)
	compareAndLog := func(itemName string, oldVal, newVal *float64) {
		if oldVal == nil || newVal == nil {
			return
		}
		if *oldVal == 0 && *newVal == 0 {
			return
		}
		if *oldVal != *newVal {
			deltaPercent := 0.0
			if *oldVal != 0 {
				deltaPercent = (*newVal - *oldVal) / *oldVal * 100
			}
			log := RestatementLog{
				Year:         year,
				Item:         itemName,
				OldValue:     *oldVal,
				NewValue:     *newVal,
				DeltaPercent: deltaPercent,
				DetectedAt:   time.Now(),
				OldSource:    old.SourceFiling.AccessionNumber,
				NewSource:    newSource,
			}
			record.Restatements = append(record.Restatements, log)
		}
	}

	// Income Statement comparisons
	if old.IncomeStatement.GrossProfitSection != nil && new.IncomeStatement.GrossProfitSection != nil {
		if old.IncomeStatement.GrossProfitSection.Revenues != nil && new.IncomeStatement.GrossProfitSection.Revenues != nil {
			compareAndLog("Revenue",
				old.IncomeStatement.GrossProfitSection.Revenues.Value,
				new.IncomeStatement.GrossProfitSection.Revenues.Value,
			)
		}
	}

	// Balance Sheet comparisons (with nil guards)
	if old.BalanceSheet.ReportedForValidation.TotalAssets != nil && new.BalanceSheet.ReportedForValidation.TotalAssets != nil {
		compareAndLog("TotalAssets",
			old.BalanceSheet.ReportedForValidation.TotalAssets.Value,
			new.BalanceSheet.ReportedForValidation.TotalAssets.Value,
		)
	}
	if old.BalanceSheet.ReportedForValidation.TotalLiabilities != nil && new.BalanceSheet.ReportedForValidation.TotalLiabilities != nil {
		compareAndLog("TotalLiabilities",
			old.BalanceSheet.ReportedForValidation.TotalLiabilities.Value,
			new.BalanceSheet.ReportedForValidation.TotalLiabilities.Value,
		)
	}
	if old.BalanceSheet.ReportedForValidation.TotalEquity != nil && new.BalanceSheet.ReportedForValidation.TotalEquity != nil {
		compareAndLog("TotalEquity",
			old.BalanceSheet.ReportedForValidation.TotalEquity.Value,
			new.BalanceSheet.ReportedForValidation.TotalEquity.Value,
		)
	}
}

// calculateCompleteness estimates how much data is available for the year.
func (z *ZipperEngine) calculateCompleteness(snapshot *YearlySnapshot) float64 {
	// Simplified: count non-nil key fields
	total := 0
	filled := 0

	// Check Balance Sheet totals
	checks := []interface{}{
		snapshot.BalanceSheet.ReportedForValidation.TotalAssets,
		snapshot.BalanceSheet.ReportedForValidation.TotalLiabilities,
		snapshot.BalanceSheet.ReportedForValidation.TotalEquity,
	}
	// Check Income Statement sections
	if snapshot.IncomeStatement.GrossProfitSection != nil {
		checks = append(checks, snapshot.IncomeStatement.GrossProfitSection.Revenues)
	}
	if snapshot.IncomeStatement.NetIncomeSection != nil {
		checks = append(checks, snapshot.IncomeStatement.NetIncomeSection.NetIncomeToCommon)
	}

	for _, c := range checks {
		total++
		if c != nil {
			filled++
		}
	}

	if total == 0 {
		return 0.0
	}
	return float64(filled) / float64(total)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func parseYear(s string) int {
	var y int
	fmt.Sscanf(s, "%d", &y)
	return y
}

// sliceBalanceSheet extracts data for a single year from a multi-year BalanceSheet.
func sliceBalanceSheet(bs edgar.BalanceSheet, yearStr string) edgar.BalanceSheet {
	sliced := bs // Shallow copy

	// Slice Reported Totals
	sliced.ReportedForValidation.TotalAssets = sliceFSAPValue(bs.ReportedForValidation.TotalAssets, yearStr)
	sliced.ReportedForValidation.TotalLiabilities = sliceFSAPValue(bs.ReportedForValidation.TotalLiabilities, yearStr)
	sliced.ReportedForValidation.TotalEquity = sliceFSAPValue(bs.ReportedForValidation.TotalEquity, yearStr)
	sliced.ReportedForValidation.TotalCurrentAssets = sliceFSAPValue(bs.ReportedForValidation.TotalCurrentAssets, yearStr)
	sliced.ReportedForValidation.TotalCurrentLiabilities = sliceFSAPValue(bs.ReportedForValidation.TotalCurrentLiabilities, yearStr)

	// Slice Current Assets
	sliced.CurrentAssets.CashAndEquivalents = sliceFSAPValue(bs.CurrentAssets.CashAndEquivalents, yearStr)
	sliced.CurrentAssets.AccountsReceivableNet = sliceFSAPValue(bs.CurrentAssets.AccountsReceivableNet, yearStr)
	sliced.CurrentAssets.Inventories = sliceFSAPValue(bs.CurrentAssets.Inventories, yearStr)

	// TODO: Add all other fields for full fidelity

	return sliced
}

// sliceIncomeStatement extracts data for a single year from a multi-year IncomeStatement.
func sliceIncomeStatement(is edgar.IncomeStatement, yearStr string) edgar.IncomeStatement {
	sliced := is // Shallow copy

	// Slice Gross Profit Section
	if is.GrossProfitSection != nil {
		sliced.GrossProfitSection = &edgar.GrossProfitSection{
			Revenues:        sliceFSAPValue(is.GrossProfitSection.Revenues, yearStr),
			CostOfGoodsSold: sliceFSAPValue(is.GrossProfitSection.CostOfGoodsSold, yearStr),
			GrossProfit:     sliceFSAPValue(is.GrossProfitSection.GrossProfit, yearStr),
		}
	}

	// Slice Net Income Section
	if is.NetIncomeSection != nil {
		sliced.NetIncomeSection = &edgar.NetIncomeSection{
			NetIncomeToCommon: sliceFSAPValue(is.NetIncomeSection.NetIncomeToCommon, yearStr),
		}
	}

	// TODO: Add all other fields for full fidelity

	return sliced
}

// sliceCashFlowStatement extracts data for a single year from a multi-year CashFlowStatement.
func sliceCashFlowStatement(cf edgar.CashFlowStatement, yearStr string) edgar.CashFlowStatement {
	sliced := cf // Shallow copy

	// Slice Reported Totals
	sliced.ReportedForValidation.NetCashOperating = sliceFSAPValue(cf.ReportedForValidation.NetCashOperating, yearStr)
	sliced.ReportedForValidation.NetCashInvesting = sliceFSAPValue(cf.ReportedForValidation.NetCashInvesting, yearStr)
	sliced.ReportedForValidation.NetCashFinancing = sliceFSAPValue(cf.ReportedForValidation.NetCashFinancing, yearStr)
	sliced.ReportedForValidation.NetChangeInCash = sliceFSAPValue(cf.ReportedForValidation.NetChangeInCash, yearStr)

	// Slice CashSummary (used by e2e output)
	if cf.CashSummary != nil {
		sliced.CashSummary = &edgar.CashSummarySection{
			NetCashOperating: sliceFSAPValue(cf.CashSummary.NetCashOperating, yearStr),
			NetCashInvesting: sliceFSAPValue(cf.CashSummary.NetCashInvesting, yearStr),
			NetCashFinancing: sliceFSAPValue(cf.CashSummary.NetCashFinancing, yearStr),
		}
	}

	// Slice InvestingActivities (for CapEx)
	if cf.InvestingActivities != nil {
		sliced.InvestingActivities = &edgar.CFInvestingSection{
			Capex: sliceFSAPValue(cf.InvestingActivities.Capex, yearStr),
		}
	}

	// Slice OperatingActivities (for NetIncomeStart)
	if cf.OperatingActivities != nil {
		sliced.OperatingActivities = &edgar.CFOperatingSection{
			NetIncomeStart: sliceFSAPValue(cf.OperatingActivities.NetIncomeStart, yearStr),
		}
	}

	return sliced
}

// sliceFSAPValue creates a new FSAPValue with only the specified year's value.
func sliceFSAPValue(v *edgar.FSAPValue, yearStr string) *edgar.FSAPValue {
	if v == nil {
		return nil
	}

	sliced := &edgar.FSAPValue{
		Label:        v.Label,
		XBRLTag:      v.XBRLTag,
		FSAPVariable: v.FSAPVariable,
		Provenance:   v.Provenance,
		Years:        make(map[string]float64),
	}

	// Extract only the requested year
	if val, ok := v.Years[yearStr]; ok {
		sliced.Value = &val
		sliced.Years[yearStr] = val
	} else if v.Value != nil {
		// Fallback: use the top-level Value if Years map doesn't have the year
		// This handles backward compatibility
		sliced.Value = v.Value
		sliced.Years[yearStr] = *v.Value
	}

	return sliced
}
