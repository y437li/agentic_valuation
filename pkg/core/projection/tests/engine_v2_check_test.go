package projection_test

import (
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/projection"
	"math"
	"testing"
)

// TestProjectionEngine_V2_Compliance verifies that the engine correctly handles:
// 1. Granular OpEx assumptions (Selling/Admin)
// 2. Full Balance Sheet Rollforward
// 3. Balance Sheet Balancing (Plug)
func TestProjectionEngine_V2_Compliance(t *testing.T) {
	// Setup Helper
	val := func(v float64) *edgar.FSAPValue {
		return &edgar.FSAPValue{Value: &v}
	}

	// 1. Setup Previous Year (Balanced)
	// Assets: Cash 100, AR 200, PPE 500, Goodwill 300 = 1100
	// Liab: AP 100, Debt 400 = 500
	// Equity: Stock 200, RE 400 = 600
	// Check: 1100 = 500 + 600 (OK)
	prevBS := &edgar.BalanceSheet{
		CurrentAssets: edgar.CurrentAssets{
			CashAndEquivalents:    val(100),
			AccountsReceivableNet: val(200),
			Inventories:           val(0),
		},
		NoncurrentAssets: edgar.NoncurrentAssets{
			PPEAtCost:               val(1000),
			AccumulatedDepreciation: val(-500),
			PPENet:                  val(500),
			Goodwill:                val(300), // New field test
		},
		CurrentLiabilities: edgar.CurrentLiabilities{
			AccountsPayable: val(100),
		},
		NoncurrentLiabilities: edgar.NoncurrentLiabilities{
			LongTermDebt: val(400),
		},
		Equity: edgar.Equity{
			CommonStockAPIC:         val(200),
			RetainedEarningsDeficit: val(400),
		},
	}

	prevIS := &edgar.IncomeStatement{
		GrossProfitSection: &edgar.GrossProfitSection{
			Revenues: val(2000),
		},
		NonOperatingSection: &edgar.NonOperatingSection{
			InterestExpense: val(20),
		},
	}

	// 2. Setup Assumptions
	assume := projection.ProjectionAssumptions{
		RevenueGrowth: 0.10, // +10% -> 2200

		// Granular OpEx
		SellingMarketingPercent: 0.15, // 330
		GeneralAdminPercent:     0.05, // 110
		// Total OpEx should be 440 (20% of 2200)

		TaxRate: 0.25,

		// WC
		DSO:         30,   // AR = 2200 * 30/365 ~ 180.8
		DPO:         40,   // AP (Assume COGS=0 for simple test). Wait, COGS needed.
		COGSPercent: 0.50, // COGS = 1100
		// AP = 1100 * 40/365 ~ 120.5

		CapexPercent:        0.05, // 110
		DepreciationPercent: 0.04, // 40
	}

	skeleton := projection.NewStandardSkeleton()
	engine := projection.NewProjectionEngine(skeleton)

	// 3. Run Projection
	proj := engine.ProjectYear(prevIS, prevBS, nil, assume, 2025)

	// 4. Validate Results

	// A. Check Granular OpEx
	selling := *proj.IncomeStatement.OperatingCostSection.SellingMarketing.Value
	admin := *proj.IncomeStatement.OperatingCostSection.GeneralAdmin.Value
	sga := *proj.IncomeStatement.OperatingCostSection.SGAExpenses.Value

	if selling != -330 {
		t.Errorf("Expected Selling -330, got %f", selling)
	}
	if admin != -110 {
		t.Errorf("Expected Admin -110, got %f", admin)
	}
	if sga != -440 {
		t.Errorf("Expected SGA -440, got %f", sga)
	}

	// B. Check Balance Sheet Identification
	// Goodwill should roll forward unchanged
	gw := *proj.BalanceSheet.NoncurrentAssets.Goodwill.Value
	if gw != 300 {
		t.Errorf("Expected Goodwill 300, got %f", gw)
	}

	// C. Check Balance (The Plug)
	bs := proj.BalanceSheet
	assets := *bs.CurrentAssets.CalculatedTotal + *bs.NoncurrentAssets.CalculatedTotal
	liabs := *bs.CurrentLiabilities.CalculatedTotal + *bs.NoncurrentLiabilities.CalculatedTotal
	equity := *bs.Equity.CalculatedTotal

	diff := assets - (liabs + equity)

	if math.Abs(diff) > 0.01 {
		t.Errorf("Balance Sheet Imbalance! Assets: %.2f, L+E: %.2f, Diff: %.2f", assets, liabs+equity, diff)
		t.Logf("Revolver Plug: %f", *bs.CurrentLiabilities.NotesPayableShortTermDebt.Value)
		t.Logf("Cash Plug: %f", *bs.CurrentAssets.CashAndEquivalents.Value)
	} else {
		t.Logf("Balance Sheet Balanced. Diff: %.5f", diff)
	}

	// Output Summary for Review
	t.Logf("Revenue: %.2f", *proj.IncomeStatement.GrossProfitSection.Revenues.Value)
	t.Logf("Net Income: %.2f", *proj.IncomeStatement.NetIncomeSection.NetIncomeToCommon.Value)
}
