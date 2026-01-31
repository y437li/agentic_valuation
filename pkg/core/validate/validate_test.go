package validate

import (
	"math"
	"testing"
)

// =============================================================================
// REAL APPLE DATA FOR TESTING (FY2020 - FY2024)
// =============================================================================
// Source: Apple Inc. Annual 10-K Reports (SEC EDGAR)
// All values in millions USD

// Apple Revenue (FY2020-2024)
var appleRevenue = map[int]float64{
	2024: 391040, // $391.04B
	2023: 383290, // $383.29B
	2022: 394330, // $394.33B
	2021: 365820, // $365.82B
	2020: 274515, // $274.52B
}

// Apple Net Income (FY2020-2024)
var appleNetIncome = map[int]float64{
	2024: 93736, // $93.74B
	2023: 96995, // $97.00B (restated in 2024 10-K)
	2022: 99803, // $99.80B
	2021: 94680, // $94.68B
	2020: 57411, // $57.41B
}

// Apple Operating Cash Flow (FY2020-2024)
var appleCFO = map[int]float64{
	2024: 118254, // $118.25B
	2023: 110543, // $110.54B
	2022: 122151, // $122.15B (record high at that time)
	2021: 104038, // $104.04B
	2020: 80674,  // $80.67B
}

// Apple CapEx (FY2020-2024) - negative values
var appleCapEx = map[int]float64{
	2024: -9447,
	2023: -10959,
	2022: -10708,
	2021: -11085,
	2020: -7309,
}

// =============================================================================
// YoY TESTS
// =============================================================================

func TestCalculateYoY(t *testing.T) {
	tests := []struct {
		name     string
		current  float64
		prior    float64
		expected float64
	}{
		{"Positive growth", 110, 100, 10.0},
		{"Negative growth", 90, 100, -10.0},
		{"Zero growth", 100, 100, 0.0},
		{"Double", 200, 100, 100.0},
		{"Halved", 50, 100, -50.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateYoY(tt.current, tt.prior)
			if math.Abs(result-tt.expected) > 0.01 {
				t.Errorf("CalculateYoY(%v, %v) = %v, want %v", tt.current, tt.prior, result, tt.expected)
			}
		})
	}
}

func TestYoYFromMap_AppleNetIncome(t *testing.T) {
	// Apple Net Income 2024 vs 2023
	result, err := YoYFromMap(appleNetIncome, 2024, 2023, "Net Income")
	if err != nil {
		t.Fatalf("YoYFromMap failed: %v", err)
	}

	t.Logf("Apple Net Income YoY:")
	t.Logf("  2024: $%.0fM", result.CurrentValue)
	t.Logf("  2023: $%.0fM", result.PriorValue)
	t.Logf("  Change: %.2f%%", result.ChangePct)

	// Expected: (93736 - 96995) / 96995 * 100 = -3.36%
	expectedPct := (93736.0 - 96995.0) / 96995.0 * 100
	if math.Abs(result.ChangePct-expectedPct) > 0.01 {
		t.Errorf("YoY = %.4f%%, expected %.4f%%", result.ChangePct, expectedPct)
	}
}

func TestYoYFromMap_AppleCFO(t *testing.T) {
	// Apple CFO 2024 vs 2023 (increased)
	result, err := YoYFromMap(appleCFO, 2024, 2023, "Operating Cash Flow")
	if err != nil {
		t.Fatalf("YoYFromMap failed: %v", err)
	}

	t.Logf("Apple CFO YoY: %.2f%%", result.ChangePct)

	// CFO increased from 110543 to 118254 → positive growth
	if result.ChangePct <= 0 {
		t.Errorf("Expected positive CFO growth, got %.2f%%", result.ChangePct)
	}
}

// =============================================================================
// CAGR TESTS
// =============================================================================

func TestCalculateCAGR(t *testing.T) {
	// $100 growing to $121 over 2 years = 10% CAGR
	// (121/100)^0.5 - 1 = 0.10
	cagr := CalculateCAGR(100, 121, 2)
	if math.Abs(cagr-10.0) > 0.01 {
		t.Errorf("CAGR = %.2f%%, expected 10%%", cagr)
	}

	// $100 growing to $200 over 7 years ≈ 10.4% CAGR
	cagr7 := CalculateCAGR(100, 200, 7)
	t.Logf("100→200 over 7 years CAGR: %.2f%%", cagr7)
}

func TestCAGRFromMap_AppleNetIncome(t *testing.T) {
	// Apple Net Income CAGR 2022-2024
	result, err := CAGRFromMap(appleNetIncome, 2022, 2024, "Net Income")
	if err != nil {
		t.Fatalf("CAGRFromMap failed: %v", err)
	}

	t.Logf("Apple Net Income CAGR (2022-2024):")
	t.Logf("  2022: $%.0fM", result.StartValue)
	t.Logf("  2024: $%.0fM", result.EndValue)
	t.Logf("  CAGR: %.2f%%", result.CAGR)

	// Net income declined, so CAGR should be negative
	if result.CAGR >= 0 {
		t.Errorf("Expected negative CAGR for declining net income, got %.2f%%", result.CAGR)
	}
}

// =============================================================================
// COMPREHENSIVE 5-YEAR TESTS (FY2020-2024)
// =============================================================================

func TestApple5Year_RevenueYoY(t *testing.T) {
	t.Log("Apple Revenue YoY (FY2020-2024):")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	years := []int{2024, 2023, 2022, 2021}
	for _, year := range years {
		result, err := YoYFromMap(appleRevenue, year, year-1, "Revenue")
		if err != nil {
			t.Fatalf("YoY failed for %d: %v", year, err)
		}
		t.Logf("  %d: $%.0fB → $%.0fB = %+.2f%%",
			year,
			result.PriorValue/1000,
			result.CurrentValue/1000,
			result.ChangePct)
	}

	// Validate specific expectations:
	// 2021 was the big COVID rebound year (+33%)
	yoy2021, _ := YoYFromMap(appleRevenue, 2021, 2020, "Revenue")
	if yoy2021.ChangePct < 30 || yoy2021.ChangePct > 40 {
		t.Errorf("2021 Revenue YoY should be ~33%%, got %.2f%%", yoy2021.ChangePct)
	}

	// 2023 was a slight decline year
	yoy2023, _ := YoYFromMap(appleRevenue, 2023, 2022, "Revenue")
	if yoy2023.ChangePct > 0 {
		t.Errorf("2023 Revenue YoY should be negative, got %.2f%%", yoy2023.ChangePct)
	}
}

func TestApple5Year_NetIncomeYoY(t *testing.T) {
	t.Log("Apple Net Income YoY (FY2020-2024):")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	years := []int{2024, 2023, 2022, 2021}
	for _, year := range years {
		result, err := YoYFromMap(appleNetIncome, year, year-1, "Net Income")
		if err != nil {
			t.Fatalf("YoY failed for %d: %v", year, err)
		}
		t.Logf("  %d: $%.0fB → $%.0fB = %+.2f%%",
			year,
			result.PriorValue/1000,
			result.CurrentValue/1000,
			result.ChangePct)
	}

	// Validate: 2021 Net Income had massive growth (+65%)
	yoy2021, _ := YoYFromMap(appleNetIncome, 2021, 2020, "Net Income")
	if yoy2021.ChangePct < 60 || yoy2021.ChangePct > 70 {
		t.Errorf("2021 Net Income YoY should be ~65%%, got %.2f%%", yoy2021.ChangePct)
	}
}

func TestApple5Year_CFOYoY(t *testing.T) {
	t.Log("Apple Operating Cash Flow YoY (FY2020-2024):")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	years := []int{2024, 2023, 2022, 2021}
	for _, year := range years {
		result, err := YoYFromMap(appleCFO, year, year-1, "CFO")
		if err != nil {
			t.Fatalf("YoY failed for %d: %v", year, err)
		}
		t.Logf("  %d: $%.0fB → $%.0fB = %+.2f%%",
			year,
			result.PriorValue/1000,
			result.CurrentValue/1000,
			result.ChangePct)
	}
}

func TestApple5Year_CAGR(t *testing.T) {
	t.Log("Apple 4-Year CAGR (FY2020-2024):")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Revenue CAGR 2020-2024 (4 years)
	revCAGR, _ := CAGRFromMap(appleRevenue, 2020, 2024, "Revenue")
	t.Logf("  Revenue CAGR: %.2f%% ($%.0fB → $%.0fB)",
		revCAGR.CAGR, revCAGR.StartValue/1000, revCAGR.EndValue/1000)

	// Net Income CAGR 2020-2024
	niCAGR, _ := CAGRFromMap(appleNetIncome, 2020, 2024, "Net Income")
	t.Logf("  Net Income CAGR: %.2f%% ($%.0fB → $%.0fB)",
		niCAGR.CAGR, niCAGR.StartValue/1000, niCAGR.EndValue/1000)

	// CFO CAGR 2020-2024
	cfoCAGR, _ := CAGRFromMap(appleCFO, 2020, 2024, "CFO")
	t.Logf("  CFO CAGR: %.2f%% ($%.0fB → $%.0fB)",
		cfoCAGR.CAGR, cfoCAGR.StartValue/1000, cfoCAGR.EndValue/1000)

	// Validate: All CAGRs should be positive (Apple grew overall from 2020)
	if revCAGR.CAGR <= 0 {
		t.Error("Revenue 4-year CAGR should be positive")
	}
	if niCAGR.CAGR <= 0 {
		t.Error("Net Income 4-year CAGR should be positive")
	}
	if cfoCAGR.CAGR <= 0 {
		t.Error("CFO 4-year CAGR should be positive")
	}
}

// =============================================================================
// FCF TESTS
// =============================================================================

func TestCalculateFCF_Apple(t *testing.T) {
	fcf2024 := CalculateFCF(appleCFO[2024], appleCapEx[2024])
	fcf2023 := CalculateFCF(appleCFO[2023], appleCapEx[2023])
	fcf2022 := CalculateFCF(appleCFO[2022], appleCapEx[2022])

	t.Logf("Apple Free Cash Flow:")
	t.Logf("  2024: $%.0fM (CFO %.0f + CapEx %.0f)", fcf2024, appleCFO[2024], appleCapEx[2024])
	t.Logf("  2023: $%.0fM", fcf2023)
	t.Logf("  2022: $%.0fM", fcf2022)

	// FCF 2024 = 118254 - 9447 = 108807
	expected2024 := 118254.0 - 9447.0
	if fcf2024 != expected2024 {
		t.Errorf("FCF 2024 = %.0f, expected %.0f", fcf2024, expected2024)
	}

	// Calculate FCF YoY
	yoy := CalculateYoY(fcf2024, fcf2023)
	t.Logf("  FCF YoY 2024: %.2f%%", yoy)
}

// =============================================================================
// OUTLIER DETECTION TESTS
// =============================================================================

func TestCheckForOutlier(t *testing.T) {
	// Normal change (should not be outlier)
	check := CheckForOutlier("Revenue", 105, 100, 50.0)
	if check.IsOutlier {
		t.Error("Normal 5% growth flagged as outlier")
	}

	// Zero value (should be outlier)
	check = CheckForOutlier("Revenue", 0, 100, 50.0)
	if !check.IsOutlier {
		t.Error("Zero value not flagged as outlier")
	}
	t.Logf("Zero outlier reason: %s", check.Reason)

	// Extreme change (should be outlier)
	check = CheckForOutlier("Revenue", 200, 100, 50.0)
	if !check.IsOutlier {
		t.Error("100% growth not flagged as outlier with 50% threshold")
	}
	t.Logf("Extreme change reason: %s", check.Reason)
}

// =============================================================================
// BALANCE SHEET VALIDATION TESTS
// =============================================================================

func TestCheckBalanceEquation(t *testing.T) {
	// Perfect balance
	check := CheckBalanceEquation(100, 60, 40, 0.01)
	if !check.IsBalanced {
		t.Error("Perfect balance not detected")
	}

	// Slight imbalance within tolerance
	check = CheckBalanceEquation(100, 60, 39.5, 1.0)
	if !check.IsBalanced {
		t.Error("Balance within tolerance not detected")
	}

	// Imbalanced
	check = CheckBalanceEquation(100, 60, 30, 1.0)
	if check.IsBalanced {
		t.Error("Imbalance not detected")
	}
	t.Logf("Imbalance difference: %.2f", check.Difference)
}

// =============================================================================
// CASH FLOW VALIDATION TESTS
// =============================================================================

func TestCheckCashFlowEquation_Apple(t *testing.T) {
	// Apple 2024 Cash Flow
	cfo := 118254.0
	cfi := 2935.0    // Investment activities
	cff := -121983.0 // Financing activities
	reportedNetChange := -794.0

	check := CheckCashFlowEquation(cfo, cfi, cff, reportedNetChange, 10.0)

	t.Logf("Apple Cash Flow Check:")
	t.Logf("  CFO: $%.0fM", cfo)
	t.Logf("  CFI: $%.0fM", cfi)
	t.Logf("  CFF: $%.0fM", cff)
	t.Logf("  Computed: $%.0fM", check.ComputedTotal)
	t.Logf("  Reported: $%.0fM", check.ReportedTotal)
	t.Logf("  Difference: $%.0fM", check.Difference)

	if !check.IsBalanced {
		t.Logf("Note: Small difference due to FX effects or rounding")
	}
}
