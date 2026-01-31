package analysis

import (
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/synthesis"
	"testing"
)

func TestAnalysisEngine_Analyze(t *testing.T) {
	// 1. Setup Mock Data
	currentYear := 2024
	prevYear := 2023

	// Helper to create a dummy YearSnapshot
	createSnapshot := func(year int, revenue, netIncome float64) *synthesis.YearlySnapshot {
		shareCount := 1000000.0
		sharePrice := 10.0

		// Helper for FSAPValue
		val := func(v float64) *edgar.FSAPValue {
			return &edgar.FSAPValue{Value: &v}
		}

		return &synthesis.YearlySnapshot{
			FiscalYear: year,
			IncomeStatement: edgar.IncomeStatement{
				GrossProfitSection: &edgar.GrossProfitSection{
					Revenues:    val(revenue),
					GrossProfit: val(revenue * 0.4), // 40% margin
				},
				OperatingCostSection: &edgar.OperatingCostSection{
					OperatingIncome: val(revenue * 0.15),
					SGAExpenses:     val(revenue * 0.20),
				},
				NetIncomeSection: &edgar.NetIncomeSection{
					NetIncomeToCommon: val(netIncome),
				},
				NonOperatingSection: &edgar.NonOperatingSection{
					InterestExpense: val(-5.0),
				},
				TaxAdjustments: &edgar.TaxAdjustmentsSection{
					IncomeTaxExpense: val(revenue * 0.05),
				},
			},
			BalanceSheet: edgar.BalanceSheet{
				ReportedForValidation: edgar.ReportedForValidation{
					TotalAssets:             val(revenue * 2), // Assets 2x Rev
					TotalEquity:             val(revenue),     // Equity 1x Rev
					TotalCurrentAssets:      val(revenue * 0.5),
					TotalCurrentLiabilities: val(revenue * 0.3),
					TotalLiabilities:        val(revenue),
				},
				CurrentAssets: edgar.CurrentAssets{
					CashAndEquivalents:    val(50.0),
					Inventories:           val(20.0),
					AccountsReceivableNet: val(30.0), // Needed for DSRI
				},
				CurrentLiabilities: edgar.CurrentLiabilities{
					NotesPayableShortTermDebt: val(10.0),
				},
				NoncurrentLiabilities: edgar.NoncurrentLiabilities{
					LongTermDebt: val(100.0),
				},
				NoncurrentAssets: edgar.NoncurrentAssets{
					PPENet: val(100.0), // Needed for Soft Assets ratio
				},
				Equity: edgar.Equity{
					RetainedEarningsDeficit: val(50.0),
				},
			},
			CashFlowStatement: edgar.CashFlowStatement{
				CashSummary: &edgar.CashSummarySection{
					NetCashOperating: val(netIncome * 1.2), // CFO often > NI
				},
				OperatingActivities: &edgar.CFOperatingSection{
					DepreciationAmortization: val(10.0), // Needed for DEPI
				},
				InvestingActivities: &edgar.CFInvestingSection{
					Capex: val(-10.0),
				},
			},
			SupplementalData: edgar.SupplementalData{
				EPSDiluted:               val(netIncome / shareCount * 1000000),
				SharesOutstandingDiluted: val(shareCount),
				SharePriceYearEnd:        &sharePrice, // Direct *float64
				DepreciationExpense:      val(10.0),   // Needed for Beneish check
			},
		}
	}

	mockRecord := &synthesis.GoldenRecord{
		Ticker: "TEST",
		CIK:    "0000000000",
		Timeline: map[int]*synthesis.YearlySnapshot{
			prevYear:    createSnapshot(prevYear, 100, 10),
			currentYear: createSnapshot(currentYear, 110, 12), // 10% Rev growth, 20% NI growth
		},
	}

	// 2. Initialize Engine
	engine := NewAnalysisEngine()

	// 3. Execute Analysis
	result, err := engine.Analyze(mockRecord)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// 4. Verify Results
	if result.Ticker != "TEST" {
		t.Errorf("Expected Ticker TEST, got %s", result.Ticker)
	}

	// Check Timeline existence
	currAnalysis, ok := result.Timeline[currentYear]
	if !ok {
		t.Fatalf("Missing analysis for year %d", currentYear)
	}

	// Verify Growth Metrics
	// Rev Growth: (110 - 100) / 100 = 0.10
	expectedRevGrowth := 0.10
	if currAnalysis.Growth.RevenueGrowth != expectedRevGrowth {
		t.Errorf("Expected Revenue Growth %.2f, got %.2f", expectedRevGrowth, currAnalysis.Growth.RevenueGrowth)
	}

	// Net Income Growth: (12 - 10) / 10 = 0.20
	expectedNIGrowth := 0.20
	if abs(currAnalysis.Growth.NetIncomeGrowth-expectedNIGrowth) > 0.0001 {
		t.Errorf("Expected Net Income Growth %.2f, got %.2f", expectedNIGrowth, currAnalysis.Growth.NetIncomeGrowth)
	}

	// Verify Three-Level Analysis populated (implied by no panic)
	if currAnalysis.Ratios == nil {
		t.Error("Three-Level Analysis (Ratios) should not be nil")
	} else {
		// Quick check on ROE
		// ROE = NetIncome / AverageEquity
		// Current NI = 12
		// Avg Equity = (100 + 110) / 2 = 105
		// Expected ROE = 12 / 105 ~= 0.114
		if currAnalysis.Ratios.Level2.ROE == 0 {
			t.Error("ROE should be calculated")
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
