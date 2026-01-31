package projection_test

import (
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/projection"
	"math"
	"testing"
)

func floatPtr(v float64) *float64 {
	return &v
}

func val(v float64) *edgar.FSAPValue {
	return &edgar.FSAPValue{Value: floatPtr(v)}
}

// Local helper to replace internal getValue from projection package
func getValue(v *edgar.FSAPValue) float64 {
	if v != nil && v.Value != nil {
		return *v.Value
	}
	return 0
}

// Helper for extracting float64 pointers from CalculatedTotal
func getF(v *float64) float64 {
	if v != nil {
		return *v
	}
	return 0
}

func TestProjectYear_Balancing(t *testing.T) {
	// Setup simple history
	// Assets: Cash 100, AR 100, Inv 100, NetPPE 500 = 800
	// Liabilities: AP 100, LTD 200 = 300
	// Equity: Stock 100, RE 400 = 500
	// Total L+E = 800. Matches.

	prevIS := &edgar.IncomeStatement{
		GrossProfitSection: &edgar.GrossProfitSection{
			Revenues: val(1000),
		},
		NonOperatingSection: &edgar.NonOperatingSection{
			InterestExpense: val(-10),
		},
	}

	prevBS := &edgar.BalanceSheet{
		CurrentAssets: edgar.CurrentAssets{
			CashAndEquivalents:    val(100),
			AccountsReceivableNet: val(100), // DSO = (100/1000)*365 = 36.5
			Inventories:           val(100),
		},
		NoncurrentAssets: edgar.NoncurrentAssets{
			PPENet:                  val(500),
			PPEAtCost:               val(1000), // Gross
			AccumulatedDepreciation: val(-500),
		},
		CurrentLiabilities: edgar.CurrentLiabilities{
			AccountsPayable: val(100),
		},
		NoncurrentLiabilities: edgar.NoncurrentLiabilities{
			LongTermDebt: val(200),
		},
		Equity: edgar.Equity{
			CommonStockAPIC:         val(100),
			RetainedEarningsDeficit: val(400),
		},
	}

	// Assumption 1: Growth that consumes massive cash (Capex)
	// Rev 10% growth = 1100
	// Capex 50% of Rev = 550 (Huge Spend)
	// Margins normal.
	assumptions := projection.ProjectionAssumptions{
		RevenueGrowth:      0.10,
		COGSPercent:        0.60,
		SGAPercent:         0.20,
		RDPercent:          0.05,
		TaxRate:            0.25,
		DSO:                36.5,
		DSI:                36.5,
		DPO:                36.5,
		CapexPercent:       0.50, // High Capex to trigger plug?
		UsefulLifeForecast: 10.0, // Dep = Gross / 10 = 1000 / 10 = 100
	}

	engine := projection.NewProjectionEngine(nil)
	proj := engine.ProjectYear(prevIS, prevBS, nil, assumptions, 2025)

	// Check IS
	// Rev = 1100
	// OpInc = 1100 - 660(COGS) - 220(SGA) - 55(RD) = 165
	// EBT = 165 - 10(Int) = 155
	// Tax = 155 * 0.25 = 38.75
	// NI = 116.25
	expNI := 116.25
	gotNI := getValue(proj.IncomeStatement.NetIncomeSection.NetIncomeToCommon)
	if math.Abs(gotNI-expNI) > 0.1 {
		t.Errorf("Diff NI: got %v, exp %v", gotNI, expNI)
	}

	// Check BS PPE Logic
	// PrevGross = 1000
	// Capex = 1100 * 0.5 = 550
	// ProjGross = 1550
	// Dep = 1000 / 10 = 100
	// ProjAccum = 500 + 100 = 600 (Stored as -600?)
	// Check Net = 1550 - 600 = 950
	gotNetPPE := getValue(proj.BalanceSheet.NoncurrentAssets.PPENet)
	if math.Abs(gotNetPPE-950) > 0.1 {
		t.Errorf("Diff NetPPE: got %v, exp 950", gotNetPPE)
	}

	// Check Plug logic
	// Proj RE = 400 + 116.25 = 516.25
	// Proj Stock = 100
	// Proj LTD = 200
	// Proj Cl/AP...
	// Proj AR = (1100/365)*36.5 = 110
	// Proj Inv = (660/365)*36.5 = 66
	// Proj AP = (660/365)*36.5 = 66

	// Total Non-Cash Assets = 110(AR) + 66(Inv) + 950(PPE) = 1126
	// Total Liabilities (pre-revolver) + Equity = 66(AP) + 200(LTD) + 100(Stock) + 516.25(RE) = 882.25

	// Gap = 882.25 - 1126 = -243.75
	// We are short 243.75
	// So Derived Cash should be 0.
	// Revolver should be 243.75.
	// Total L+E should be 882.25 + 243.75 = 1126.
	// Assets = 1126. Balanced.

	gotCash := getValue(proj.BalanceSheet.CurrentAssets.CashAndEquivalents)
	if gotCash != 0 {
		t.Errorf("Expected 0 cash due to deficit, got %v", gotCash)
	}
	gotRevolver := getValue(proj.BalanceSheet.CurrentLiabilities.NotesPayableShortTermDebt)
	if math.Abs(gotRevolver-243.75) > 0.1 {
		t.Errorf("Expected Revolver ~243.75, got %v", gotRevolver)
	}

	// Check Totals manually
	// Assets
	ca := getF(proj.BalanceSheet.CurrentAssets.CalculatedTotal)
	if ca == 0 {
		ca = getValue(proj.BalanceSheet.CurrentAssets.CashAndEquivalents) +
			getValue(proj.BalanceSheet.CurrentAssets.AccountsReceivableNet) +
			getValue(proj.BalanceSheet.CurrentAssets.Inventories)
	}
	nca := getF(proj.BalanceSheet.NoncurrentAssets.CalculatedTotal)
	if nca == 0 {
		nca = getValue(proj.BalanceSheet.NoncurrentAssets.PPENet)
	}

	totalAssets := ca + nca

	// L+E
	cl := getF(proj.BalanceSheet.CurrentLiabilities.CalculatedTotal)
	if cl == 0 {
		cl = getValue(proj.BalanceSheet.CurrentLiabilities.AccountsPayable) +
			getValue(proj.BalanceSheet.CurrentLiabilities.NotesPayableShortTermDebt)
	}
	ncl := getF(proj.BalanceSheet.NoncurrentLiabilities.CalculatedTotal)
	if ncl == 0 {
		ncl = getValue(proj.BalanceSheet.NoncurrentLiabilities.LongTermDebt)
	}
	eq := getF(proj.BalanceSheet.Equity.CalculatedTotal)
	if eq == 0 {
		eq = getValue(proj.BalanceSheet.Equity.CommonStockAPIC) +
			getValue(proj.BalanceSheet.Equity.RetainedEarningsDeficit)
	}

	totalLE := cl + ncl + eq

	if math.Abs(totalAssets-totalLE) > 0.1 {
		t.Errorf("Balance Sheet Imbalance: Assets %v != L+E %v (Diff: %v)", totalAssets, totalLE, totalAssets-totalLE)
	}
}
