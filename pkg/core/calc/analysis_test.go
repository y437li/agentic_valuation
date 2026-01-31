package calc

import (
	"math"
	"testing"
)

func TestPenmanDecomposition(t *testing.T) {
	// Setup
	// Operating Assets = 1000, Operating Liabs = 200 => NOA = 800
	// Debt = 500, Cash = 100 => Net Debt (NFO) = 400
	// Equity = Assets (1100) - Liabs (700) = 400.
	// Valid Logic: NOA (800) = NFO (400) + Equity (400). Checks out.

	avgNOA := 800.0
	avgNFO := 400.0
	avgEquity := 400.0

	// Income
	// NOPAT = 80 (10% RNOA)
	// Net Interest After Tax = 20 (5% Net Cost of Debt)
	// Net Income = 60
	nopat := 80.0
	netInterestAT := 20.0

	// Verify Logic
	res := CalculatePenmanDecomposition(nopat, netInterestAT, avgNOA, avgNFO, avgEquity)

	if res.RNOA != 0.10 {
		t.Errorf("Expected RNOA 0.10, got %f", res.RNOA)
	}
	if res.NBC != 0.05 {
		t.Errorf("Expected NBC 0.05, got %f", res.NBC)
	}
	if res.FLEV != 1.0 {
		t.Errorf("Expected FLEV 1.0 (400/400), got %f", res.FLEV)
	}
	if res.Spread != 0.05 {
		t.Errorf("Expected Spread 0.05 (0.10 - 0.05), got %f", res.Spread)
	}

	// ROCE = RNOA + (FLEV * Spread) = 0.10 + (1.0 * 0.05) = 0.15
	// Check against direct calculation: Net Income / Equity = 60 / 400 = 0.15
	if math.Abs(res.ROCE-0.15) > 0.0001 {
		t.Errorf("Expected ROCE 0.15, got %f", res.ROCE)
	}

}

func TestRiskModels(t *testing.T) {
	// Altman Z-Score Safe
	// Dummy data
	z := AltmanZScoreManufacturing(100, 50, 20, 200, 300, 1000, 500)
	// A = 0.1, B = 0.05, C = 0.02, D = 0.4, E = 0.3
	// Z = 1.2*0.1 + 1.4*0.05 + 3.3*0.02 + 0.6*0.4 + 1.0*0.3
	//   = 0.12    + 0.07     + 0.066    + 0.24    + 0.3
	//   = 0.796
	expected := 1.2*0.1 + 1.4*0.05 + 3.3*0.02 + 0.6*0.4 + 1.0*0.3

	if math.Abs(z-expected) > 0.0001 {
		t.Errorf("Z-Score expected %f, got %f", expected, z)
	}
}

func TestBeneishMScore(t *testing.T) {
	// Values that suggest manipulation (high growth, high accruals)
	input := BeneishInput{
		DSRI: 1.5, // Receivable Days skyrocketing
		GMI:  1.2, // Gross Margin deteriorating (Index > 1 means bad? No, GMI = Prior/Current. If GM falls, GMI > 1. Means pressure to Fake.)
		AQI:  1.0,
		SGI:  1.3, // High Sales Growth
		DEPI: 1.0,
		SGAI: 1.0,
		LVGI: 1.0,
		TATA: 0.1, // High Accruals
	}

	// M = -4.84 + 0.92*1.5 + 0.528*1.2 + 0.404*1.0 + 0.892*1.3 + 0.115*1.0 - 0.172*1.0 + 4.679*0.1 - 0.327*1.0
	//   = -4.84 + 1.38     + 0.6336    + 0.404     + 1.1596    + 0.115     - 0.172      + 0.4679     - 0.327
	// Approximate manually

	m := BeneishMScore(input)
	// Just verify it runs and logical direction.
	// High DSRI/SGI/TATA should increase M (more likely manipulation).
	// Current inputs are aggressive. M should be > -1.78 (Manipulation Probable threshold).

	expectedM := -4.84 + 0.92*1.5 + 0.528*1.2 + 0.404*1.0 + 0.892*1.3 + 0.115*1.0 - 0.172*1.0 + 4.679*0.1 - 0.327*1.0
	if math.Abs(m-expectedM) > 0.0001 {
		t.Errorf("Beneish M expected %f, got %f", expectedM, m)
	}
}

func TestBenfordAnalysis(t *testing.T) {
	// Synthesize a dataset that follows Benford perfectly
	// 30 numbers starting with 1, 17 with 2, etc. (Total 100)
	// To minimize code, we just check digit extraction logic.

	values := []float64{
		105.0, 1500.0, 19.0, // Should be '1' -> 3 count
		200.0, 25.0, // '2' -> 2 count
		-300.0, // '3' -> 1 count (Abs)
		0.5,    // Skipped (< 1)
		9.9,    // '9' -> 1 count
	}

	res := AnalyzeBenfordsLaw(values)

	// Total processed: 3+2+1+1 = 7. (0.5 skipped)
	if res.TotalCount != 7 {
		t.Errorf("Expected 7 processed values, got %d", res.TotalCount)
	}

	if res.DigitCounts[1] != 3 {
		t.Errorf("Expected 3 ones, got %d", res.DigitCounts[1])
	}
	if res.DigitCounts[9] != 1 {
		t.Errorf("Expected 1 nine, got %d", res.DigitCounts[9])
	}

	// Check Frequency
	freq1 := 3.0 / 7.0
	if math.Abs(res.DigitFrequencies[1]-freq1) > 0.0001 {
		t.Error("Frequency calc wrong")
	}
}
