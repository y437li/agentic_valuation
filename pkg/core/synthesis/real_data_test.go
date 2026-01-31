package synthesis

import (
	"testing"

	"agentic_valuation/pkg/core/edgar"
)

// =============================================================================
// REAL APPLE DATA - HARDCODED FROM FY2024 10-K
// =============================================================================
// Source: Apple Inc. 10-K for FY2024 (Filed November 2024)
// Accession: 0000320193-24-000123
// All values in millions USD

// Apple FY2024 10-K Real Data (3-year comparative)
var appleRealData = struct {
	Ticker          string
	CIK             string
	CompanyName     string
	AccessionNumber string
	FilingDate      string
	FiscalYear      int

	// Net Income (from Cash Flow - Net Income Start)
	NetIncome map[int]float64

	// Operating Cash Flow
	CFOOperating map[int]float64

	// CapEx
	CapEx map[int]float64

	// Cash and Equivalents (Balance Sheet)
	Cash map[int]float64

	// Share Repurchases (Financing)
	ShareRepurchases map[int]float64

	// Dividends Paid
	DividendsPaid map[int]float64

	// Total Assets (Balance Sheet)
	TotalAssets map[int]float64
}{
	Ticker:          "AAPL",
	CIK:             "0000320193",
	CompanyName:     "Apple Inc.",
	AccessionNumber: "0000320193-24-000123",
	FilingDate:      "2024-11-01",
	FiscalYear:      2024,

	// Real Net Income data from 10-K
	NetIncome: map[int]float64{
		2024: 93736,
		2023: 96995,
		2022: 99803,
	},

	// Real Operating Cash Flow from 10-K
	CFOOperating: map[int]float64{
		2024: 118254,
		2023: 110543,
		2022: 122151,
	},

	// Real CapEx from 10-K (negative values as outflows)
	CapEx: map[int]float64{
		2024: -9447,
		2023: -10959,
		2022: -10708,
	},

	// Real Cash from Balance Sheet
	Cash: map[int]float64{
		2024: 29943,
		2023: 29965,
	},

	// Real Share Repurchases
	ShareRepurchases: map[int]float64{
		2024: -94949,
		2023: -77550,
		2022: -89402,
	},

	// Real Dividends Paid
	DividendsPaid: map[int]float64{
		2024: -15234,
		2023: -15025,
		2022: -14841,
	},

	// Real Total Assets
	TotalAssets: map[int]float64{
		2024: 364980,
		2023: 352583, // Approximate from prior 10-K
	},
}

// =============================================================================
// TEST: YoY CALCULATION WITH REAL APPLE DATA (HARDCODED)
// =============================================================================

func TestAppleReal_NetIncomeYoY(t *testing.T) {
	data := appleRealData

	t.Logf("Company: %s (FY%d)", data.CompanyName, data.FiscalYear)
	t.Logf("Source: %s", data.AccessionNumber)

	// Extract values
	ni2024 := data.NetIncome[2024]
	ni2023 := data.NetIncome[2023]
	ni2022 := data.NetIncome[2022]

	t.Logf("Net Income:")
	t.Logf("  2024: $%.0fM", ni2024)
	t.Logf("  2023: $%.0fM", ni2023)
	t.Logf("  2022: $%.0fM", ni2022)

	// Calculate YoY
	yoy_2024 := (ni2024 - ni2023) / ni2023 * 100
	yoy_2023 := (ni2023 - ni2022) / ni2022 * 100

	t.Logf("YoY Change:")
	t.Logf("  2024 vs 2023: %.2f%% (expected ~-3.4%%)", yoy_2024)
	t.Logf("  2023 vs 2022: %.2f%% (expected ~-2.8%%)", yoy_2023)

	// Validate: Apple's net income was declining slightly in these years
	// 93736 / 96995 - 1 = -3.36%
	expectedYoY2024 := (93736.0 - 96995.0) / 96995.0 * 100
	if diff := yoy_2024 - expectedYoY2024; diff > 0.01 || diff < -0.01 {
		t.Errorf("YoY 2024 calculation error: got %.4f%%, expected %.4f%%", yoy_2024, expectedYoY2024)
	}

	// 96995 / 99803 - 1 = -2.81%
	expectedYoY2023 := (96995.0 - 99803.0) / 99803.0 * 100
	if diff := yoy_2023 - expectedYoY2023; diff > 0.01 || diff < -0.01 {
		t.Errorf("YoY 2023 calculation error: got %.4f%%, expected %.4f%%", yoy_2023, expectedYoY2023)
	}

	t.Log("✓ Net Income YoY calculations verified")
}

func TestAppleReal_OperatingCashFlowYoY(t *testing.T) {
	data := appleRealData

	cfo2024 := data.CFOOperating[2024]
	cfo2023 := data.CFOOperating[2023]
	cfo2022 := data.CFOOperating[2022]

	t.Logf("Operating Cash Flow:")
	t.Logf("  2024: $%.0fM", cfo2024)
	t.Logf("  2023: $%.0fM", cfo2023)
	t.Logf("  2022: $%.0fM", cfo2022)

	yoy_2024 := (cfo2024 - cfo2023) / cfo2023 * 100
	yoy_2023 := (cfo2023 - cfo2022) / cfo2022 * 100

	t.Logf("YoY Change:")
	t.Logf("  2024 vs 2023: %.2f%% (CFO increased)", yoy_2024)
	t.Logf("  2023 vs 2022: %.2f%% (CFO decreased)", yoy_2023)

	// 2024: CFO went UP (+6.98%)
	if yoy_2024 < 0 {
		t.Errorf("Expected positive CFO YoY for 2024, got %.2f%%", yoy_2024)
	}

	// 2023: CFO went DOWN (-9.5%)
	if yoy_2023 > 0 {
		t.Errorf("Expected negative CFO YoY for 2023, got %.2f%%", yoy_2023)
	}

	t.Log("✓ Operating Cash Flow YoY calculations verified")
}

func TestAppleReal_FreeCashFlow(t *testing.T) {
	data := appleRealData

	// FCF = CFO + CapEx (CapEx is negative)
	fcf2024 := data.CFOOperating[2024] + data.CapEx[2024]
	fcf2023 := data.CFOOperating[2023] + data.CapEx[2023]
	fcf2022 := data.CFOOperating[2022] + data.CapEx[2022]

	t.Logf("Free Cash Flow (CFO + CapEx):")
	t.Logf("  2024: $%.0fM (118254 - 9447)", fcf2024)
	t.Logf("  2023: $%.0fM (110543 - 10959)", fcf2023)
	t.Logf("  2022: $%.0fM (122151 - 10708)", fcf2022)

	// Validate FCF calculation
	expectedFCF2024 := 118254.0 - 9447.0
	if fcf2024 != expectedFCF2024 {
		t.Errorf("FCF 2024 calculation error: got %.0f, expected %.0f", fcf2024, expectedFCF2024)
	}

	yoy := (fcf2024 - fcf2023) / fcf2023 * 100
	t.Logf("FCF YoY 2024: %.2f%%", yoy)

	t.Log("✓ Free Cash Flow calculation verified")
}

func TestAppleReal_ShareholderReturns(t *testing.T) {
	data := appleRealData

	// Total shareholder returns = Repurchases + Dividends
	returns2024 := -(data.ShareRepurchases[2024] + data.DividendsPaid[2024]) // Make positive
	returns2023 := -(data.ShareRepurchases[2023] + data.DividendsPaid[2023])
	returns2022 := -(data.ShareRepurchases[2022] + data.DividendsPaid[2022])

	t.Logf("Total Shareholder Returns (Buybacks + Dividends):")
	t.Logf("  2024: $%.0fM", returns2024)
	t.Logf("  2023: $%.0fM", returns2023)
	t.Logf("  2022: $%.0fM", returns2022)

	// 2024: 94949 + 15234 = 110,183
	expectedReturns2024 := 94949.0 + 15234.0
	if returns2024 != expectedReturns2024 {
		t.Errorf("Returns 2024 error: got %.0f, expected %.0f", returns2024, expectedReturns2024)
	}

	yoy := (returns2024 - returns2023) / returns2023 * 100
	t.Logf("Shareholder Returns YoY 2024: %.2f%%", yoy)

	// Apple increased buybacks in 2024
	if yoy < 0 {
		t.Errorf("Expected increased shareholder returns in 2024, got %.2f%%", yoy)
	}

	t.Log("✓ Shareholder returns calculation verified")
}

// =============================================================================
// TEST: ZIPPER WITH REAL APPLE DATA STRUCTURE
// =============================================================================

func TestAppleReal_ZipperSynthesis(t *testing.T) {
	// Build FSAPDataResponse with real Apple data
	data := buildAppleFSAPData()

	snapshot := ExtractionSnapshot{
		FilingMetadata: SourceMetadata{
			AccessionNumber: appleRealData.AccessionNumber,
			FilingDate:      appleRealData.FilingDate,
			Form:            "10-K",
			IsAmended:       false,
		},
		FiscalYear: appleRealData.FiscalYear,
		Data:       data,
	}

	zipper := NewZipperEngine()
	record, err := zipper.Stitch(appleRealData.Ticker, appleRealData.CIK, []ExtractionSnapshot{snapshot})
	if err != nil {
		t.Fatalf("Stitch failed: %v", err)
	}

	// All 3 years should be extracted from the multi-year filing
	for _, year := range []int{2022, 2023, 2024} {
		if record.Timeline[year] == nil {
			t.Errorf("Missing year %d in timeline", year)
		} else {
			t.Logf("Year %d synthesized, source: %s", year, record.Timeline[year].SourceFiling.AccessionNumber)
		}
	}

	// Calculate YoY from GoldenRecord
	if record.Timeline[2024] != nil && record.Timeline[2023] != nil {
		// Get Net Income from CFO start
		if record.Timeline[2024].CashFlowStatement.OperatingActivities != nil {
			ni2024 := record.Timeline[2024].CashFlowStatement.OperatingActivities.NetIncomeStart
			ni2023 := record.Timeline[2023].CashFlowStatement.OperatingActivities.NetIncomeStart

			if ni2024 != nil && ni2024.Value != nil && ni2023 != nil && ni2023.Value != nil {
				yoy := (*ni2024.Value - *ni2023.Value) / *ni2023.Value * 100
				t.Logf("Net Income YoY from GoldenRecord: %.2f%%", yoy)
			}
		}
	}

	t.Logf("✓ Zipper synthesized %d years from real Apple data", len(record.Timeline))
}

// buildAppleFSAPData constructs a realistic FSAPDataResponse from hardcoded Apple data.
func buildAppleFSAPData() *edgar.FSAPDataResponse {
	d := appleRealData

	return &edgar.FSAPDataResponse{
		Company:      d.CompanyName,
		CIK:          d.CIK,
		FiscalYear:   d.FiscalYear,
		FiscalPeriod: "FY",

		CashFlowStatement: edgar.CashFlowStatement{
			OperatingActivities: &edgar.CFOperatingSection{
				NetIncomeStart: &edgar.FSAPValue{
					Label: "Net income",
					Value: floatPtr(d.NetIncome[2024]),
					Years: map[string]float64{
						"2024": d.NetIncome[2024],
						"2023": d.NetIncome[2023],
						"2022": d.NetIncome[2022],
					},
				},
			},
			InvestingActivities: &edgar.CFInvestingSection{
				Capex: &edgar.FSAPValue{
					Label: "Capital expenditures",
					Value: floatPtr(d.CapEx[2024]),
					Years: map[string]float64{
						"2024": d.CapEx[2024],
						"2023": d.CapEx[2023],
						"2022": d.CapEx[2022],
					},
				},
			},
			CashSummary: &edgar.CashSummarySection{
				NetCashOperating: &edgar.FSAPValue{
					Label: "Net cash from operating activities",
					Value: floatPtr(d.CFOOperating[2024]),
					Years: map[string]float64{
						"2024": d.CFOOperating[2024],
						"2023": d.CFOOperating[2023],
						"2022": d.CFOOperating[2022],
					},
				},
			},
		},

		BalanceSheet: edgar.BalanceSheet{
			CurrentAssets: edgar.CurrentAssets{
				CashAndEquivalents: &edgar.FSAPValue{
					Label: "Cash and cash equivalents",
					Value: floatPtr(d.Cash[2024]),
					Years: map[string]float64{
						"2024": d.Cash[2024],
						"2023": d.Cash[2023],
					},
				},
			},
			ReportedForValidation: edgar.ReportedForValidation{
				TotalAssets: &edgar.FSAPValue{
					Label: "Total assets",
					Value: floatPtr(d.TotalAssets[2024]),
					Years: map[string]float64{
						"2024": d.TotalAssets[2024],
						"2023": d.TotalAssets[2023],
					},
				},
			},
		},
	}
}

func floatPtr(f float64) *float64 {
	return &f
}
