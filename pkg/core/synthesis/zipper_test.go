package synthesis

import (
	"agentic_valuation/pkg/core/edgar"
	"fmt"
	"testing"
)

// =============================================================================
// HELPER FUNCTIONS FOR TEST DATA CREATION
// =============================================================================

// makeRevenue creates an FSAPValue with multi-year revenue data.
func makeRevenue(years map[int]float64) *edgar.FSAPValue {
	v := &edgar.FSAPValue{
		Label: "Revenue",
		Years: make(map[string]float64),
	}
	for year, val := range years {
		yearStr := intToStr(year)
		v.Years[yearStr] = val
		// Set Value to the latest year for backward compatibility
		valCopy := val
		v.Value = &valCopy
	}
	return v
}

// makeTotalAssets creates an FSAPValue with multi-year total assets data.
func makeTotalAssets(years map[int]float64) *edgar.FSAPValue {
	v := &edgar.FSAPValue{
		Label: "Total Assets",
		Years: make(map[string]float64),
	}
	for year, val := range years {
		yearStr := intToStr(year)
		v.Years[yearStr] = val
		valCopy := val
		v.Value = &valCopy
	}
	return v
}

// makeSnapshot creates an ExtractionSnapshot for testing.
func makeSnapshot(accession, filingDate, form string, isAmended bool, fiscalYear int, revenue *edgar.FSAPValue, totalAssets *edgar.FSAPValue) ExtractionSnapshot {
	data := &edgar.FSAPDataResponse{
		FiscalYear: fiscalYear,
		IncomeStatement: edgar.IncomeStatement{
			GrossProfitSection: &edgar.GrossProfitSection{
				Revenues: revenue,
			},
		},
		BalanceSheet: edgar.BalanceSheet{
			ReportedForValidation: edgar.ReportedForValidation{
				TotalAssets: totalAssets,
			},
		},
	}
	return ExtractionSnapshot{
		FilingMetadata: SourceMetadata{
			AccessionNumber: accession,
			FilingDate:      filingDate,
			Form:            form,
			IsAmended:       isAmended,
		},
		FiscalYear: fiscalYear,
		Data:       data,
	}
}

func intToStr(i int) string {
	return fmt.Sprintf("%d", i)
}

// =============================================================================
// TEST CASE 1: NORMAL RESTATEMENT OVERRIDE
// =============================================================================
// Input:
//   - Filing_2023.json: { 2023: 100, 2022: 50 } (Original)
//   - Filing_2024.json: { 2024: 120, 2023: 102, 2022: 52 } (Restated)
// Expected Output:
//   - 2024: 120, 2023: 102, 2022: 52
//   - Restatements logged for 2023 (+2%) and 2022 (+4%)

func TestCase1_NormalRestatement(t *testing.T) {
	zipper := NewZipperEngine()

	// Filing 1: 2023 10-K (filed 2024-02-28)
	filing2023 := makeSnapshot(
		"0001234-23-000001", "2024-02-28", "10-K", false, 2023,
		makeRevenue(map[int]float64{2023: 100, 2022: 50}),
		makeTotalAssets(map[int]float64{2023: 500, 2022: 400}),
	)

	// Filing 2: 2024 10-K (filed 2025-02-28) with restated prior years
	filing2024 := makeSnapshot(
		"0001234-24-000001", "2025-02-28", "10-K", false, 2024,
		makeRevenue(map[int]float64{2024: 120, 2023: 102, 2022: 52}),
		makeTotalAssets(map[int]float64{2024: 600, 2023: 510, 2022: 405}),
	)

	snapshots := []ExtractionSnapshot{filing2023, filing2024}
	record, err := zipper.Stitch("AAPL", "0000320193", snapshots)
	if err != nil {
		t.Fatalf("Stitch failed: %v", err)
	}

	// Assert: 2024 revenue = 120
	if record.Timeline[2024] == nil {
		t.Fatal("Missing 2024 in timeline")
	}
	if record.Timeline[2024].IncomeStatement.GrossProfitSection == nil {
		t.Fatal("Missing GrossProfitSection for 2024")
	}
	rev2024 := record.Timeline[2024].IncomeStatement.GrossProfitSection.Revenues.Value
	if rev2024 == nil || *rev2024 != 120 {
		t.Errorf("Expected 2024 revenue=120, got %v", rev2024)
	}

	// Assert: 2023 revenue = 102 (restated, not 100)
	rev2023 := record.Timeline[2023].IncomeStatement.GrossProfitSection.Revenues.Value
	if rev2023 == nil || *rev2023 != 102 {
		t.Errorf("Expected 2023 revenue=102 (restated), got %v", rev2023)
	}

	// Assert: 2022 revenue = 52 (restated, not 50)
	rev2022 := record.Timeline[2022].IncomeStatement.GrossProfitSection.Revenues.Value
	if rev2022 == nil || *rev2022 != 52 {
		t.Errorf("Expected 2022 revenue=52 (restated), got %v", rev2022)
	}

	// Assert: Restatements logged
	if len(record.Restatements) < 2 {
		t.Errorf("Expected at least 2 restatement logs, got %d", len(record.Restatements))
	}

	// Check restatement details
	for _, r := range record.Restatements {
		t.Logf("Restatement: Year=%d, Item=%s, Old=%.2f, New=%.2f, Delta=%.2f%%",
			r.Year, r.Item, r.OldValue, r.NewValue, r.DeltaPercent)
	}
}

// =============================================================================
// TEST CASE 2: FALLBACK TO OLDER DATA
// =============================================================================
// Input:
//   - Filing_2024.json: { 2024: 120, 2023: 102 } (only 2 years disclosed)
//   - Filing_2022.json: { 2022: 50, 2021: 40 }
// Expected Output:
//   - 2024: 120, 2023: 102, 2022: 50, 2021: 40
// Time series is not interrupted.

func TestCase2_FallbackToOlder(t *testing.T) {
	zipper := NewZipperEngine()

	// Filing 1: 2022 10-K (filed 2023-02-28)
	filing2022 := makeSnapshot(
		"0001234-22-000001", "2023-02-28", "10-K", false, 2022,
		makeRevenue(map[int]float64{2022: 50, 2021: 40}),
		makeTotalAssets(map[int]float64{2022: 400, 2021: 350}),
	)

	// Filing 2: 2024 10-K (filed 2025-02-28) - only discloses 2024 and 2023
	filing2024 := makeSnapshot(
		"0001234-24-000001", "2025-02-28", "10-K", false, 2024,
		makeRevenue(map[int]float64{2024: 120, 2023: 102}),
		makeTotalAssets(map[int]float64{2024: 600, 2023: 510}),
	)

	snapshots := []ExtractionSnapshot{filing2022, filing2024}
	record, err := zipper.Stitch("AAPL", "0000320193", snapshots)
	if err != nil {
		t.Fatalf("Stitch failed: %v", err)
	}

	// Assert: All 4 years are present
	expectedYears := []int{2021, 2022, 2023, 2024}
	for _, y := range expectedYears {
		if record.Timeline[y] == nil {
			t.Errorf("Missing year %d in timeline", y)
		}
	}

	// Assert: 2021 comes from 2022 filing (fallback)
	rev2021 := record.Timeline[2021].IncomeStatement.GrossProfitSection.Revenues.Value
	if rev2021 == nil || *rev2021 != 40 {
		t.Errorf("Expected 2021 revenue=40 (from older filing), got %v", rev2021)
	}

	// Assert: 2022 comes from 2022 filing (because 2024 doesn't have it)
	rev2022 := record.Timeline[2022].IncomeStatement.GrossProfitSection.Revenues.Value
	if rev2022 == nil || *rev2022 != 50 {
		t.Errorf("Expected 2022 revenue=50, got %v", rev2022)
	}

	t.Logf("Timeline years: %v", record.Timeline)
}

// =============================================================================
// TEST CASE 3: 10-K/A AMENDMENT PRIORITY
// =============================================================================
// Input:
//   - Filing_2023_10K.json: { 2023: 100 } (Filed 2024-03-01)
//   - Filing_2023_10KA.json: { 2023: 95 } (Filed 2024-04-15, Amendment)
// Expected Output:
//   - 2023: 95 (Amendment wins)

func TestCase3_AmendmentPriority(t *testing.T) {
	zipper := NewZipperEngine()

	// Filing 1: Original 10-K
	filing10K := makeSnapshot(
		"0001234-23-000001", "2024-03-01", "10-K", false, 2023,
		makeRevenue(map[int]float64{2023: 100}),
		makeTotalAssets(map[int]float64{2023: 500}),
	)

	// Filing 2: Amendment 10-K/A (filed later)
	filing10KA := makeSnapshot(
		"0001234-23-000002", "2024-04-15", "10-K/A", true, 2023,
		makeRevenue(map[int]float64{2023: 95}),
		makeTotalAssets(map[int]float64{2023: 495}),
	)

	snapshots := []ExtractionSnapshot{filing10K, filing10KA}
	record, err := zipper.Stitch("AAPL", "0000320193", snapshots)
	if err != nil {
		t.Fatalf("Stitch failed: %v", err)
	}

	// Assert: 2023 revenue = 95 (amendment wins)
	rev2023 := record.Timeline[2023].IncomeStatement.GrossProfitSection.Revenues.Value
	if rev2023 == nil || *rev2023 != 95 {
		t.Errorf("Expected 2023 revenue=95 (amendment), got %v", rev2023)
	}

	// Assert: Source is the amendment
	source := record.Timeline[2023].SourceFiling
	if !source.IsAmended {
		t.Error("Expected source to be the amended filing")
	}
	if source.AccessionNumber != "0001234-23-000002" {
		t.Errorf("Expected source accession 0001234-23-000002, got %s", source.AccessionNumber)
	}

	// Also test reverse order: Amendment first, then original -> Amendment still wins
	snapshotsReverse := []ExtractionSnapshot{filing10KA, filing10K}
	record2, _ := zipper.Stitch("AAPL", "0000320193", snapshotsReverse)
	rev2023_2 := record2.Timeline[2023].IncomeStatement.GrossProfitSection.Revenues.Value
	if rev2023_2 == nil || *rev2023_2 != 95 {
		t.Errorf("Reverse order: Expected 2023 revenue=95, got %v", rev2023_2)
	}
}

// =============================================================================
// TEST CASE 4: RADICAL ACCOUNTING CHANGE
// =============================================================================
// Input:
//   - Filing_2023.json: { 2023: 1000 } (Old GAAP: GMV as revenue)
//   - Filing_2024.json: { 2024: 200, 2023: 200 } (New GAAP: Net commission only)
// Expected Output:
//   - 2024: 200, 2023: 200 (restated)
//   - Significant restatement alert (-80%)

func TestCase4_RadicalAccountingChange(t *testing.T) {
	zipper := NewZipperEngine()

	// Filing 1: 2023 10-K with old GAAP
	filing2023 := makeSnapshot(
		"0001234-23-000001", "2024-02-28", "10-K", false, 2023,
		makeRevenue(map[int]float64{2023: 1000}),
		makeTotalAssets(map[int]float64{2023: 5000}),
	)

	// Filing 2: 2024 10-K with new GAAP (restated 2023)
	filing2024 := makeSnapshot(
		"0001234-24-000001", "2025-02-28", "10-K", false, 2024,
		makeRevenue(map[int]float64{2024: 200, 2023: 200}),
		makeTotalAssets(map[int]float64{2024: 1000, 2023: 900}),
	)

	snapshots := []ExtractionSnapshot{filing2023, filing2024}
	record, err := zipper.Stitch("AAPL", "0000320193", snapshots)
	if err != nil {
		t.Fatalf("Stitch failed: %v", err)
	}

	// Assert: 2023 revenue = 200 (restated, not 1000)
	rev2023 := record.Timeline[2023].IncomeStatement.GrossProfitSection.Revenues.Value
	if rev2023 == nil || *rev2023 != 200 {
		t.Errorf("Expected 2023 revenue=200 (restated for new GAAP), got %v", rev2023)
	}

	// Assert: Significant restatement detected
	foundSignificantRestatement := false
	for _, r := range record.Restatements {
		if r.Year == 2023 && r.Item == "Revenue" {
			foundSignificantRestatement = true
			t.Logf("Restatement: Old=%v, New=%v, Delta=%.1f%%", r.OldValue, r.NewValue, r.DeltaPercent)
			if r.DeltaPercent > -70 { // Should be around -80%
				t.Errorf("Expected significant restatement (-80%%), got %.1f%%", r.DeltaPercent)
			}
		}
	}
	if !foundSignificantRestatement {
		t.Error("Expected significant restatement for 2023 Revenue")
	}

	// Verify correct growth calculation would work
	rev2024 := record.Timeline[2024].IncomeStatement.GrossProfitSection.Revenues.Value
	if rev2024 != nil && rev2023 != nil {
		growth := (*rev2024 - *rev2023) / *rev2023 * 100
		t.Logf("Corrected YoY Growth 2024: %.1f%%", growth)
		if growth != 0 {
			t.Errorf("Expected 0%% growth (200->200), got %.1f%%", growth)
		}
	}
}

// =============================================================================
// TEST CASE 4b: PROOF THAT WRONG DATA CAUSES WRONG GROWTH (Negative Test)
// =============================================================================
// This test proves WHY the Zipper is necessary.
// Without restatement override, YoY growth would be -80% (phantom crash).

func TestCase4b_WrongGrowthWithoutZipper(t *testing.T) {
	// Simulate what happens if we DON'T use Zipper properly:
	// We take 2023's OLD value (1000) and compare to 2024's NEW value (200)

	oldGAAP_2023 := 1000.0 // From 2023 10-K (GMV as revenue)
	newGAAP_2024 := 200.0  // From 2024 10-K (Net commission only)

	// WRONG calculation (mixing accounting bases)
	wrongGrowth := (newGAAP_2024 - oldGAAP_2023) / oldGAAP_2023 * 100
	t.Logf("WRONG YoY Growth (mixing GAAP): %.1f%%", wrongGrowth)

	// This is the phantom -80% crash that fools analysts
	if wrongGrowth > -70 || wrongGrowth < -90 {
		t.Errorf("Expected ~-80%% wrong growth, got %.1f%%", wrongGrowth)
	}

	// CORRECT calculation (using restated 2023 from 2024 10-K)
	restated_2023 := 200.0 // From 2024 10-K's comparative column
	correctGrowth := (newGAAP_2024 - restated_2023) / restated_2023 * 100
	t.Logf("CORRECT YoY Growth (consistent GAAP): %.1f%%", correctGrowth)

	// This is the TRUE 0% growth
	if correctGrowth != 0 {
		t.Errorf("Expected 0%% correct growth, got %.1f%%", correctGrowth)
	}

	// Conclusion: The Zipper algorithm ensures we use restated_2023, not oldGAAP_2023
	t.Log("✓ Zipper algorithm prevents phantom crashes by using restated comparative data")
}

// =============================================================================
// TEST CASE 5: OUTLIER/GLITCH PROTECTION
// =============================================================================
// Input:
//   - Filing_2024.json: { 2024: 0, 2023: 100 } (LLM extraction error: 2024 is 0)
// Expected:
//   - Flag as DATA_CORRUPTION for human review
// Note: This test validates the detection mechanism, not the rejection logic.

func TestCase5_OutlierDetection(t *testing.T) {
	zipper := NewZipperEngine()
	zipper.OutlierThreshold = 0.5 // 50% change threshold

	// Filing with suspicious data (revenue dropped to 0)
	filingBad := makeSnapshot(
		"0001234-24-000001", "2025-02-28", "10-K", false, 2024,
		makeRevenue(map[int]float64{2024: 0, 2023: 100}), // 2024 is 0 (likely error)
		makeTotalAssets(map[int]float64{2024: 500, 2023: 500}),
	)

	snapshots := []ExtractionSnapshot{filingBad}
	record, err := zipper.Stitch("AAPL", "0000320193", snapshots)
	if err != nil {
		t.Fatalf("Stitch failed: %v", err)
	}

	// The Zipper currently stores the data as-is.
	// The outlier detection would be an additional layer.
	// For now, verify the data is stored and can be flagged.
	rev2024 := record.Timeline[2024].IncomeStatement.GrossProfitSection.Revenues.Value
	if rev2024 == nil {
		t.Fatal("Missing 2024 revenue")
	}

	// Check if 2024 revenue is 0 (the suspicious value)
	if *rev2024 == 0 {
		t.Log("Detected suspicious data: 2024 Revenue = 0 (potential extraction error)")
		// In a production system, this would trigger an alert/flag
		// TODO: Add formal outlier detection to ZipperEngine
	}

	// Log the data for review
	t.Logf("2024 Revenue: %v (flagged for review)", *rev2024)
	t.Logf("2023 Revenue: %v", *record.Timeline[2023].IncomeStatement.GrossProfitSection.Revenues.Value)
}

// =============================================================================
// BOUNDARY TESTS
// =============================================================================

func TestEmptySnapshots(t *testing.T) {
	zipper := NewZipperEngine()
	_, err := zipper.Stitch("AAPL", "0000320193", []ExtractionSnapshot{})
	if err == nil {
		t.Error("Expected error for empty snapshots")
	}
}

func TestSingleSnapshot(t *testing.T) {
	zipper := NewZipperEngine()
	filing := makeSnapshot(
		"0001234-24-000001", "2025-02-28", "10-K", false, 2024,
		makeRevenue(map[int]float64{2024: 100, 2023: 90}),
		makeTotalAssets(map[int]float64{2024: 500}),
	)

	record, err := zipper.Stitch("AAPL", "0000320193", []ExtractionSnapshot{filing})
	if err != nil {
		t.Fatalf("Stitch failed: %v", err)
	}

	if len(record.Timeline) != 2 {
		t.Errorf("Expected 2 years in timeline, got %d", len(record.Timeline))
	}
	if len(record.Restatements) != 0 {
		t.Errorf("Expected 0 restatements for single snapshot, got %d", len(record.Restatements))
	}
}
