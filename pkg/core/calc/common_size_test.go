package calc

import (
	"agentic_valuation/pkg/core/edgar"
	"math"
	"testing"
)

func TestCalculateCommonSize_ExpandedDrivers(t *testing.T) {
	// Setup
	rev := 1000.0
	// IS
	otherOp := 30.0   // 3%
	disconOps := 50.0 // 5%
	// BS (Assets)
	ar := 100.0      // 10%
	inv := 50.0      // 5%
	dynAsset := 20.0 // 2% (Dynamic)
	// BS (Liabs)
	ap := 75.0      // 7.5%
	defRev := 25.0  // 2.5%
	dynLiab := 15.0 // 1.5% (Dynamic)

	histData := &edgar.YearData{
		IncomeStatement: edgar.IncomeStatement{
			GrossProfitSection: &edgar.GrossProfitSection{
				Revenues: &edgar.FSAPValue{Value: &rev},
			},
			OperatingCostSection: &edgar.OperatingCostSection{
				OtherOperatingExpenses: &edgar.FSAPValue{Value: &otherOp},
			},
			TaxAdjustments: &edgar.TaxAdjustmentsSection{
				DiscontinuedOperations: &edgar.FSAPValue{Value: &disconOps},
			},
		},
		BalanceSheet: edgar.BalanceSheet{
			CurrentAssets: edgar.CurrentAssets{
				AccountsReceivableNet: &edgar.FSAPValue{Value: &ar},
				Inventories:           &edgar.FSAPValue{Value: &inv},
				AdditionalItems: []edgar.FSAPValue{
					{Label: "Legal Asset", Value: &dynAsset},
				},
			},
			CurrentLiabilities: edgar.CurrentLiabilities{
				AccountsPayable:        &edgar.FSAPValue{Value: &ap},
				DeferredRevenueCurrent: &edgar.FSAPValue{Value: &defRev},
				AdditionalItems: []edgar.FSAPValue{
					{Label: "Legal Liability", Value: &dynLiab},
				},
			},
		},
	}

	// Execute
	defaults := CalculateCommonSizeDefaults(histData)

	// Verify IS
	if math.Abs(defaults.OtherOperatingPercent-0.03) > 0.0001 {
		t.Errorf("OtherOperatingPercent expected 0.03, got %f", defaults.OtherOperatingPercent)
	}
	if math.Abs(defaults.DiscontinuedOpsPercent-0.05) > 0.0001 {
		t.Errorf("DiscontinuedOpsPercent expected 0.05, got %f", defaults.DiscontinuedOpsPercent)
	}

	// Verify BS Standard
	if math.Abs(defaults.ReceivablesPercent-0.10) > 0.0001 {
		t.Errorf("ReceivablesPercent expected 0.10, got %f", defaults.ReceivablesPercent)
	}
	if math.Abs(defaults.InventoryPercent-0.05) > 0.0001 {
		t.Errorf("InventoryPercent expected 0.05, got %f", defaults.InventoryPercent)
	}
	if math.Abs(defaults.APPercent-0.075) > 0.0001 {
		t.Errorf("APPercent expected 0.075, got %f", defaults.APPercent)
	}
	if math.Abs(defaults.DeferredRevPercent-0.025) > 0.0001 {
		t.Errorf("DeferredRevPercent expected 0.025, got %f", defaults.DeferredRevPercent)
	}

	// Verify Dynamic Items
	// Note: Keys follow the implementation naming convention
	assetKey := "Legal Asset (BS-Asset)"
	if val, ok := defaults.CustomItems[assetKey]; !ok {
		t.Errorf("Missing dynamic asset key: %s", assetKey)
	} else if math.Abs(val-0.02) > 0.0001 {
		t.Errorf("Dynamic Asset expected 0.02, got %f", val)
	}

	liabKey := "Legal Liability (BS-Liab)"
	if val, ok := defaults.CustomItems[liabKey]; !ok {
		t.Errorf("Missing dynamic liability key: %s", liabKey)
	} else if math.Abs(val-0.015) > 0.0001 {
		t.Errorf("Dynamic Liability expected 0.015, got %f", val)
	}
}
