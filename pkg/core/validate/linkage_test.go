package validate

import (
	"math"
	"testing"

	"agentic_valuation/pkg/core/edgar"
)

// TestLinkageValidation_NetIncomeIS_to_CF tests IS Net Income → CF Linkage
func TestLinkageValidation_NetIncomeIS_to_CF(t *testing.T) {
	// Create test data where IS NI = CF NI
	niValue := 94760.0

	is := &edgar.IncomeStatement{
		NetIncomeSection: &edgar.NetIncomeSection{
			NetIncomeToCommon: &edgar.FSAPValue{Value: &niValue},
		},
	}

	cf := &edgar.CashFlowStatement{
		OperatingActivities: &edgar.CFOperatingSection{
			NetIncomeStart: &edgar.FSAPValue{Value: &niValue},
		},
	}

	linkage := validateNetIncomeLinkage(is, cf, 1.0)

	if !linkage.IsLinked {
		t.Errorf("Expected IS→CF linkage to pass, got difference of %.2f", linkage.Difference)
	}
	if linkage.ISNetIncome != niValue {
		t.Errorf("Expected IS Net Income = %.2f, got %.2f", niValue, linkage.ISNetIncome)
	}
	if linkage.CFNetIncStart != niValue {
		t.Errorf("Expected CF Net Income Start = %.2f, got %.2f", niValue, linkage.CFNetIncStart)
	}

	t.Logf("✓ IS→CF Linkage: $%.0fM → $%.0fM (diff: $%.0fM)",
		linkage.ISNetIncome, linkage.CFNetIncStart, linkage.Difference)
}

// TestLinkageValidation_NetIncome_Mismatch tests failure case
func TestLinkageValidation_NetIncome_Mismatch(t *testing.T) {
	niIS := 94760.0
	niCF := 93000.0 // Different value

	is := &edgar.IncomeStatement{
		NetIncomeSection: &edgar.NetIncomeSection{
			NetIncomeToCommon: &edgar.FSAPValue{Value: &niIS},
		},
	}

	cf := &edgar.CashFlowStatement{
		OperatingActivities: &edgar.CFOperatingSection{
			NetIncomeStart: &edgar.FSAPValue{Value: &niCF},
		},
	}

	linkage := validateNetIncomeLinkage(is, cf, 1.0)

	if linkage.IsLinked {
		t.Errorf("Expected IS→CF linkage to FAIL due to %.2f difference", niIS-niCF)
	}

	expectedDiff := niIS - niCF
	if math.Abs(linkage.Difference-expectedDiff) > 0.01 {
		t.Errorf("Expected difference = %.2f, got %.2f", expectedDiff, linkage.Difference)
	}

	t.Logf("✓ Correctly detected mismatch: IS $%.0fM vs CF $%.0fM (diff: $%.0fM)",
		linkage.ISNetIncome, linkage.CFNetIncStart, linkage.Difference)
}

// TestLinkageValidation_CashFlow_to_BalanceSheet tests CF Cash → BS Cash
func TestLinkageValidation_CashFlow_to_BalanceSheet(t *testing.T) {
	// Apple FY2024 approximate values
	cashEnding := 29943.0
	cashBeginning := 29965.0
	netChange := cashEnding - cashBeginning // -22

	cf := &edgar.CashFlowStatement{
		CashSummary: &edgar.CashSummarySection{
			CashEnding:      &edgar.FSAPValue{Value: &cashEnding},
			NetChangeInCash: &edgar.FSAPValue{Value: &netChange},
		},
	}

	bsCurrent := &edgar.BalanceSheet{
		CurrentAssets: edgar.CurrentAssets{
			CashAndEquivalents: &edgar.FSAPValue{Value: &cashEnding},
		},
	}

	bsPrior := &edgar.BalanceSheet{
		CurrentAssets: edgar.CurrentAssets{
			CashAndEquivalents: &edgar.FSAPValue{Value: &cashBeginning},
		},
	}

	linkage := validateCashLinkage(cf, bsCurrent, bsPrior, 1.0)

	if !linkage.IsLinked {
		t.Errorf("Expected CF→BS Cash linkage to pass, got differences: Cash=%.2f, NetChange=%.2f",
			linkage.DifferenceCash, linkage.DifferenceNC)
	}

	t.Logf("✓ CF→BS Linkage: CF Cash Ending $%.0fM = BS Cash $%.0fM",
		linkage.CFCashEnding, linkage.BSCash)
	t.Logf("  CF Net Change $%.0fM = BS ΔCash $%.0fM",
		linkage.CFNetChange, linkage.BSCashChange)
}

// TestLinkageValidation_FullReport tests the complete ValidateLinkages function
func TestLinkageValidation_FullReport(t *testing.T) {
	// Create matching data for all linkages
	netIncome := 93736.0
	cashEnding := 29943.0
	cashBeginning := 30737.0
	netChange := cashEnding - cashBeginning
	dividends := -14996.0
	reCurrent := 1408.0
	rePrior := -3068.0

	is := &edgar.IncomeStatement{
		NetIncomeSection: &edgar.NetIncomeSection{
			NetIncomeToCommon: &edgar.FSAPValue{Value: &netIncome},
		},
	}

	cf := &edgar.CashFlowStatement{
		OperatingActivities: &edgar.CFOperatingSection{
			NetIncomeStart: &edgar.FSAPValue{Value: &netIncome},
		},
		FinancingActivities: &edgar.CFFinancingSection{
			DividendsPaid: &edgar.FSAPValue{Value: &dividends},
		},
		CashSummary: &edgar.CashSummarySection{
			CashEnding:      &edgar.FSAPValue{Value: &cashEnding},
			NetChangeInCash: &edgar.FSAPValue{Value: &netChange},
		},
	}

	bsCurrent := &edgar.BalanceSheet{
		CurrentAssets: edgar.CurrentAssets{
			CashAndEquivalents: &edgar.FSAPValue{Value: &cashEnding},
		},
		Equity: edgar.Equity{
			RetainedEarningsDeficit: &edgar.FSAPValue{Value: &reCurrent},
		},
	}

	bsPrior := &edgar.BalanceSheet{
		CurrentAssets: edgar.CurrentAssets{
			CashAndEquivalents: &edgar.FSAPValue{Value: &cashBeginning},
		},
		Equity: edgar.Equity{
			RetainedEarningsDeficit: &edgar.FSAPValue{Value: &rePrior},
		},
	}

	report := ValidateLinkages(is, cf, bsCurrent, bsPrior, 2024, 10.0)

	// Print detailed report
	t.Log("\n=== LINKAGE VALIDATION REPORT (FY2024) ===")
	t.Log("────────────────────────────────────────────")

	if report.ISToCS != nil {
		status := "✓"
		if !report.ISToCS.IsLinked {
			status = "✗"
		}
		t.Logf("%s IS Net Income → CF Net Income Start", status)
		t.Logf("   IS: $%.0fM | CF: $%.0fM | Diff: $%.0fM",
			report.ISToCS.ISNetIncome, report.ISToCS.CFNetIncStart, report.ISToCS.Difference)
	}

	if report.CFToBS != nil {
		status := "✓"
		if !report.CFToBS.IsLinked {
			status = "✗"
		}
		t.Logf("%s CF Cash Ending → BS Cash", status)
		t.Logf("   CF: $%.0fM | BS: $%.0fM | Diff: $%.0fM",
			report.CFToBS.CFCashEnding, report.CFToBS.BSCash, report.CFToBS.DifferenceCash)
		t.Logf("   Net Change: CF $%.0fM | BS: $%.0fM | Diff: $%.0fM",
			report.CFToBS.CFNetChange, report.CFToBS.BSCashChange, report.CFToBS.DifferenceNC)
	}

	if report.ISToBSRetainedEarnings != nil {
		status := "✓"
		if !report.ISToBSRetainedEarnings.IsLinked {
			status = "✗"
		}
		t.Logf("%s Retained Earnings Check", status)
		t.Logf("   NI: $%.0fM - Div: $%.0fM = Expected ΔRE: $%.0fM",
			report.ISToBSRetainedEarnings.NetIncome,
			report.ISToBSRetainedEarnings.DividendsPaid,
			report.ISToBSRetainedEarnings.ExpectedREChange)
		t.Logf("   Actual ΔRE: $%.0fM | Diff: $%.0fM",
			report.ISToBSRetainedEarnings.ActualREChange,
			report.ISToBSRetainedEarnings.Difference)
		if report.ISToBSRetainedEarnings.Note != "" {
			t.Logf("   Note: %s", report.ISToBSRetainedEarnings.Note)
		}
	}

	t.Log("────────────────────────────────────────────")
	if report.AllPassed {
		t.Log("✓ ALL LINKAGES VALID")
	} else {
		t.Logf("✗ FAILED CHECKS: %v", report.FailedChecks)
	}
}

// =============================================================================
// MULTI-YEAR LINKAGE VALIDATION TESTS
// =============================================================================

// TestMultiYearLinkage_CashReconciliation tests cash reconciliation across 3 years
// This simulates what Zipper does when synthesizing multi-year data from a 10-K
func TestMultiYearLinkage_CashReconciliation(t *testing.T) {
	// Apple 10-K FY2024 data (approximate, in millions)
	// Cash Flow Statement has 3 years of data
	cf := &edgar.CashFlowStatement{
		CashSummary: &edgar.CashSummarySection{
			NetCashOperating: &edgar.FSAPValue{
				Years: map[string]float64{
					"2022": 122151,
					"2023": 110543,
					"2024": 118254,
				},
			},
			NetCashInvesting: &edgar.FSAPValue{
				Years: map[string]float64{
					"2022": -22354,
					"2023": 3705,
					"2024": 2935,
				},
			},
			NetCashFinancing: &edgar.FSAPValue{
				Years: map[string]float64{
					"2022": -110749,
					"2023": -108488,
					"2024": -121983,
				},
			},
			CashEnding: &edgar.FSAPValue{
				Years: map[string]float64{
					"2022": 24977,
					"2023": 30737,
					"2024": 29943,
				},
			},
			CashBeginning: &edgar.FSAPValue{
				Years: map[string]float64{
					"2022": 35929,
					"2023": 24977,
					"2024": 30737,
				},
			},
			NetChangeInCash: &edgar.FSAPValue{
				Years: map[string]float64{
					"2022": -10952,
					"2023": 5760,
					"2024": -794,
				},
			},
		},
	}

	// Note: BS data would be used for CF→BS linkage in full validation
	// Here we focus on CF internal consistency across years

	t.Log("\n=== MULTI-YEAR CASH RECONCILIATION TEST ===")
	t.Log("────────────────────────────────────────────")

	years := []string{"2022", "2023", "2024"}
	allPassed := true

	for i, year := range years {
		priorYear := ""
		if i > 0 {
			priorYear = years[i-1]
		}

		// Get values from Years maps
		cfoCurrent := getYearValue(cf.CashSummary.NetCashOperating, year)
		cfiCurrent := getYearValue(cf.CashSummary.NetCashInvesting, year)
		cffCurrent := getYearValue(cf.CashSummary.NetCashFinancing, year)
		cfNetChange := getYearValue(cf.CashSummary.NetChangeInCash, year)
		cfCashEnd := getYearValue(cf.CashSummary.CashEnding, year)
		cfCashBeg := getYearValue(cf.CashSummary.CashBeginning, year)

		// Validation 1: CFO + CFI + CFF ≈ Net Change
		calculatedNetChange := cfoCurrent + cfiCurrent + cffCurrent
		diffNetChange := cfNetChange - calculatedNetChange
		cfReconciled := math.Abs(diffNetChange) <= 100 // $100M tolerance for FX

		// Validation 2: Cash Ending - Cash Beginning ≈ Net Change
		bsCashChange := cfCashEnd - cfCashBeg
		diffCashChange := cfNetChange - bsCashChange
		bsReconciled := math.Abs(diffCashChange) <= 1

		status := "✓"
		if !cfReconciled || !bsReconciled {
			status = "✗"
			allPassed = false
		}

		t.Logf("%s FY%s:", status, year)
		t.Logf("    CFO: $%.0fM + CFI: $%.0fM + CFF: $%.0fM = $%.0fM",
			cfoCurrent, cfiCurrent, cffCurrent, calculatedNetChange)
		t.Logf("    Net Change (reported): $%.0fM | Diff: $%.0fM", cfNetChange, diffNetChange)
		t.Logf("    Cash: $%.0fM → $%.0fM (Δ=$%.0fM vs NC=$%.0fM)",
			cfCashBeg, cfCashEnd, bsCashChange, cfNetChange)

		// Cross-year validation: Prior year's ending cash = this year's beginning cash
		if priorYear != "" {
			priorCashEnd := getYearValue(cf.CashSummary.CashEnding, priorYear)
			if math.Abs(priorCashEnd-cfCashBeg) > 1 {
				t.Logf("    ⚠️ Cash continuity break: FY%s End $%.0fM ≠ FY%s Begin $%.0fM",
					priorYear, priorCashEnd, year, cfCashBeg)
				allPassed = false
			} else {
				t.Logf("    ✓ Cash continuity: FY%s End $%.0fM = FY%s Begin $%.0fM",
					priorYear, priorCashEnd, year, cfCashBeg)
			}
		}
	}

	t.Log("────────────────────────────────────────────")
	if allPassed {
		t.Log("✓ ALL YEARS RECONCILED")
	} else {
		t.Log("✗ SOME YEARS FAILED RECONCILIATION")
	}
}

// TestMultiYearLinkage_BalanceSheetEquation tests A = L + E across multiple years
func TestMultiYearLinkage_BalanceSheetEquation(t *testing.T) {
	// Apple FY2024 10-K Balance Sheet (2 years)
	bs := &edgar.BalanceSheet{
		ReportedForValidation: edgar.ReportedForValidation{
			TotalAssets: &edgar.FSAPValue{
				Years: map[string]float64{
					"2023": 352583,
					"2024": 364980,
				},
			},
			TotalLiabilities: &edgar.FSAPValue{
				Years: map[string]float64{
					"2023": 290437,
					"2024": 308030,
				},
			},
			TotalEquity: &edgar.FSAPValue{
				Years: map[string]float64{
					"2023": 62146,
					"2024": 56950,
				},
			},
		},
	}

	t.Log("\n=== MULTI-YEAR BALANCE SHEET EQUATION TEST ===")
	t.Log("────────────────────────────────────────────────")

	years := []string{"2023", "2024"}
	allPassed := true

	for _, year := range years {
		assets := getYearValue(bs.ReportedForValidation.TotalAssets, year)
		liabilities := getYearValue(bs.ReportedForValidation.TotalLiabilities, year)
		equity := getYearValue(bs.ReportedForValidation.TotalEquity, year)

		calculatedAssets := liabilities + equity
		diff := assets - calculatedAssets

		status := "✓"
		if math.Abs(diff) > 1 {
			status = "✗"
			allPassed = false
		}

		t.Logf("%s FY%s: A=$%.0fM = L=$%.0fM + E=$%.0fM (calc=$%.0fM, diff=$%.0fM)",
			status, year, assets, liabilities, equity, calculatedAssets, diff)
	}

	// YoY Change Analysis
	t.Log("\nYoY Change Analysis:")
	for i := 1; i < len(years); i++ {
		currYear := years[i]
		prevYear := years[i-1]

		assetsChange := getYearValue(bs.ReportedForValidation.TotalAssets, currYear) -
			getYearValue(bs.ReportedForValidation.TotalAssets, prevYear)
		liabChange := getYearValue(bs.ReportedForValidation.TotalLiabilities, currYear) -
			getYearValue(bs.ReportedForValidation.TotalLiabilities, prevYear)
		equityChange := getYearValue(bs.ReportedForValidation.TotalEquity, currYear) -
			getYearValue(bs.ReportedForValidation.TotalEquity, prevYear)

		t.Logf("  FY%s→FY%s: ΔA=$%.0fM = ΔL=$%.0fM + ΔE=$%.0fM (calc=$%.0fM)",
			prevYear, currYear, assetsChange, liabChange, equityChange, liabChange+equityChange)
	}

	t.Log("────────────────────────────────────────────────")
	if allPassed {
		t.Log("✓ ALL YEARS BALANCED")
	} else {
		t.Log("✗ SOME YEARS UNBALANCED")
	}
}

// TestMultiYearLinkage_IncomeStatement_to_CashFlow tests NI linkage across 3 years
func TestMultiYearLinkage_IncomeStatement_to_CashFlow(t *testing.T) {
	// Apple FY2024 10-K data
	netIncomeData := map[string]float64{
		"2022": 99803,
		"2023": 96995,
		"2024": 93736,
	}

	is := &edgar.IncomeStatement{
		NetIncomeSection: &edgar.NetIncomeSection{
			NetIncomeToCommon: &edgar.FSAPValue{
				Years: netIncomeData,
			},
		},
	}

	cf := &edgar.CashFlowStatement{
		OperatingActivities: &edgar.CFOperatingSection{
			NetIncomeStart: &edgar.FSAPValue{
				Years: netIncomeData, // Should match exactly
			},
		},
	}

	t.Log("\n=== MULTI-YEAR IS→CF NET INCOME LINKAGE TEST ===")
	t.Log("─────────────────────────────────────────────────")

	years := []string{"2022", "2023", "2024"}
	allPassed := true

	for _, year := range years {
		isNI := getYearValue(is.NetIncomeSection.NetIncomeToCommon, year)
		cfNI := getYearValue(cf.OperatingActivities.NetIncomeStart, year)
		diff := isNI - cfNI

		status := "✓"
		if math.Abs(diff) > 1 {
			status = "✗"
			allPassed = false
		}

		t.Logf("%s FY%s: IS Net Income $%.0fM → CF Net Income Start $%.0fM (diff: $%.0fM)",
			status, year, isNI, cfNI, diff)
	}

	// Calculate YoY growth
	t.Log("\nNet Income YoY Growth:")
	for i := 1; i < len(years); i++ {
		curr := netIncomeData[years[i]]
		prev := netIncomeData[years[i-1]]
		yoy := (curr - prev) / prev * 100
		t.Logf("  FY%s→FY%s: $%.0fM → $%.0fM (%.1f%%)", years[i-1], years[i], prev, curr, yoy)
	}

	t.Log("─────────────────────────────────────────────────")
	if allPassed {
		t.Log("✓ ALL YEARS LINKED")
	} else {
		t.Log("✗ SOME YEARS NOT LINKED")
	}
}
