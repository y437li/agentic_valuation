package synthesis

import (
	"agentic_valuation/pkg/core/edgar"
	"testing"
)

// Helper to create a dummy FSAPDataResponse
func createDummyData(year int, revenue float64) *edgar.FSAPDataResponse {
	revVal := revenue
	return &edgar.FSAPDataResponse{
		FiscalYear: year,
		IncomeStatement: edgar.IncomeStatement{
			GrossProfitSection: &edgar.GrossProfitSection{
				Revenues: &edgar.FSAPValue{
					Value: &revVal,
					Years: map[string]float64{
						"2022": revenue, // Explicitly testing 2022 data
					},
					Label: "Net Revenue",
				},
			},
			NetIncomeSection: &edgar.NetIncomeSection{
				NetIncomeToCommon: &edgar.FSAPValue{
					Value: &revVal, // Dummy value
					Years: map[string]float64{"2022": revenue},
				},
			},
		},
		BalanceSheet: edgar.BalanceSheet{
			ReportedForValidation: edgar.ReportedForValidation{
				TotalAssets:      &edgar.FSAPValue{Value: float64Ptr(100), Years: map[string]float64{"2022": 100}},
				TotalLiabilities: &edgar.FSAPValue{Value: float64Ptr(50), Years: map[string]float64{"2022": 50}},
				TotalEquity:      &edgar.FSAPValue{Value: float64Ptr(50), Years: map[string]float64{"2022": 50}},
			},
		},
		CashFlowStatement: edgar.CashFlowStatement{
			OperatingActivities: &edgar.CFOperatingSection{
				NetIncomeStart: &edgar.FSAPValue{Value: &revVal, Years: map[string]float64{"2022": revenue}},
			},
		},
	}
}

func float64Ptr(v float64) *float64 { return &v }

// TestZipper_RecencyBias verifies that newer filings overwrite older data for the SAME fiscal year.
func TestZipper_RecencyBias(t *testing.T) {
	// Snapshot 1: Old Filing (e.g., 2023 10-K) reporting 2022 Revenue as 100
	snap1 := ExtractionSnapshot{
		FilingMetadata: SourceMetadata{
			AccessionNumber: "OLD-FILING-2023",
			FilingDate:      "2023-03-01",
			Form:            "10-K",
		},
		FiscalYear: 2023,
		Data:       createDummyData(2022, 100.0),
	}

	// Snapshot 2: New Filing (e.g., 2024 10-K) reporting 2022 Revenue as 105 (Restated)
	snap2 := ExtractionSnapshot{
		FilingMetadata: SourceMetadata{
			AccessionNumber: "NEW-FILING-2024",
			FilingDate:      "2024-03-01", // Newer date
			Form:            "10-K",
		},
		FiscalYear: 2024,
		Data:       createDummyData(2022, 105.0),
	}

	zipper := NewZipperEngine()
	// Pass in arbitrary order; Stitch should sort by date
	record, err := zipper.Stitch("TEST", "000", []ExtractionSnapshot{snap1, snap2})
	if err != nil {
		t.Fatalf("Stitch failed: %v", err)
	}

	// Verify 2022 Data
	snapshot2022, ok := record.Timeline[2022]
	if !ok {
		t.Fatal("2022 data missing from timeline")
	}

	actualRev := *snapshot2022.IncomeStatement.GrossProfitSection.Revenues.Value
	if actualRev != 105.0 {
		t.Errorf("Recency Bias Failed: Expected 105.0 (New), got %.1f", actualRev)
	}

	// Verify Restatement Log
	if len(record.Restatements) == 0 {
		t.Error("Failed to detect restatement")
	} else {
		log := record.Restatements[0]
		if log.OldValue != 100.0 || log.NewValue != 105.0 {
			t.Errorf("Restatement Log Incorrect: %+v", log)
		}
	}
}

// TestZipper_AmendmentDominance verifies 10-K/A beats 10-K regardless of date (usually)
// ideally checks IsAmended flag
func TestZipper_AmendmentDominance(t *testing.T) {
	// Snapshot 1: Standard 10-K
	snap1 := ExtractionSnapshot{
		FilingMetadata: SourceMetadata{
			AccessionNumber: "ORIGINAL-2023",
			FilingDate:      "2023-03-01",
			Form:            "10-K",
			IsAmended:       false,
		},
		FiscalYear: 2023,
		Data:       createDummyData(2022, 100.0),
	}

	// Snapshot 2: Amendment 10-K/A (Same date/year context, but amended)
	snap2 := ExtractionSnapshot{
		FilingMetadata: SourceMetadata{
			AccessionNumber: "AMENDMENT-2023",
			FilingDate:      "2023-03-01",
			Form:            "10-K/A",
			IsAmended:       true,
		},
		FiscalYear: 2023,
		Data:       createDummyData(2022, 110.0),
	}

	zipper := NewZipperEngine()
	// Pass snap1 first, then snap2
	record, err := zipper.Stitch("TEST", "000", []ExtractionSnapshot{snap1, snap2})
	if err != nil {
		t.Fatalf("Stitch failed: %v", err)
	}

	snapshot2022 := record.Timeline[2022]
	actualRev := *snapshot2022.IncomeStatement.GrossProfitSection.Revenues.Value

	if actualRev != 110.0 {
		t.Errorf("Amendment Dominance Failed: Expected 110.0, got %.1f", actualRev)
	}
}

// TestZipper_GoldenStandard verifies accounting identity validation
func TestZipper_GoldenStandard(t *testing.T) {
	// Create INVALID data (Assets != L + E)
	badData := createDummyData(2022, 100.0)

	// FIX: Update both Value and Years map because Zipper prioritizes Years map
	*badData.BalanceSheet.ReportedForValidation.TotalAssets.Value = 200.0
	badData.BalanceSheet.ReportedForValidation.TotalAssets.Years["2022"] = 200.0 // Mismatch (200 != 50+50)

	snap := ExtractionSnapshot{
		FilingMetadata: SourceMetadata{FilingDate: "2023-01-01"},
		FiscalYear:     2023,
		Data:           badData,
	}

	zipper := NewZipperEngine()
	record, _ := zipper.Stitch("TEST", "000", []ExtractionSnapshot{snap})

	// The Validation is currently just a fmt.Printf in Stitch,
	// but we can call ValidateGoldenRecord directly to test the logic
	err := ValidateGoldenRecord(record)
	if err == nil {
		t.Error("Expected validation error for mismatched balance sheet, got nil")
	} else {
		t.Logf("✅ Successfully caught validation error: %v", err)
	}
}

// TestZipper_DeepSlicing verifies nested fields are preserved
func TestZipper_DeepSlicing(t *testing.T) {
	// Create deep nested data
	data := createDummyData(2022, 100.0)

	// Add Additional Item in IS
	data.IncomeStatement.GrossProfitSection.AdditionalItems = []edgar.AdditionalItem{
		{
			Label: "Digital Revenue",
			Value: &edgar.FSAPValue{Value: float64Ptr(25.0), Years: map[string]float64{"2022": 25.0}},
		},
	}

	snap := ExtractionSnapshot{
		FilingMetadata: SourceMetadata{FilingDate: "2023-01-01"},
		FiscalYear:     2023,
		Data:           data,
	}

	zipper := NewZipperEngine()
	record, _ := zipper.Stitch("TEST", "000", []ExtractionSnapshot{snap})

	// Check if Additional Item survived slicing
	snapshot := record.Timeline[2022]
	items := snapshot.IncomeStatement.GrossProfitSection.AdditionalItems
	if len(items) != 1 {
		t.Fatalf("Deep Slicing Failed: Expected 1 Additional Item, got %d", len(items))
	}
	if *items[0].Value.Value != 25.0 {
		t.Errorf("Deep Slicing Value Mismatch: Expected 25.0, got %.1f", *items[0].Value.Value)
	}
}
