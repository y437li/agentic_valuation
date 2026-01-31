package e2e_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/projection"
	"agentic_valuation/pkg/core/valuation"
)

func TestE2E_CalcEngine_AAPL(t *testing.T) {
	// 1. Load Real Data
	// Relative path from tests/e2e to batch_data
	dataPath := filepath.Join("..", "..", "batch_data", "AAPL", "AAPL_FY2024.json")

	content, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("Failed to read AAPL data file: %v. Ensure you are running from tests/e2e or root.", err)
	}

	var resp edgar.FSAPDataResponse
	if err := json.Unmarshal(content, &resp); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// -------------------------------------------------------------------------
	// CRITICAL FIX: MIGRATE FLAT JSON TO NESTED SECTIONS
	// -------------------------------------------------------------------------
	// The AAPL_FY2024.json uses legacy flat structure, but Calc/Projection engine
	// expects nested 6-section structure. We manually adapt it here.

	is := &resp.IncomeStatement

	// 1. Gross Profit Section
	if is.GrossProfitSection == nil {
		is.GrossProfitSection = &edgar.GrossProfitSection{
			Revenues:        is.Revenues,
			CostOfGoodsSold: is.CostOfGoodsSold,
		}
		// Try to calculate Gross Profit if possible
		if is.Revenues != nil && is.Revenues.Value != nil && is.CostOfGoodsSold != nil && is.CostOfGoodsSold.Value != nil {
			gp := *is.Revenues.Value - *is.CostOfGoodsSold.Value
			is.GrossProfitSection.GrossProfit = &edgar.FSAPValue{Value: &gp}
		} else if is.ReportedForValidation.GrossProfit.Value != nil {
			is.GrossProfitSection.GrossProfit = &edgar.FSAPValue{Value: is.ReportedForValidation.GrossProfit.Value}
		}
	}

	// 2. Operating Cost Section
	if is.OperatingCostSection == nil {
		is.OperatingCostSection = &edgar.OperatingCostSection{
			SGAExpenses: is.SGAExpenses,
			RDExpenses:  is.RDExpenses,
		}
		// Operating Income
		if is.ReportedForValidation.OperatingIncome.Value != nil {
			is.OperatingCostSection.OperatingIncome = &edgar.FSAPValue{Value: is.ReportedForValidation.OperatingIncome.Value}
		}
	}

	// 3. Non-Operating Section
	if is.NonOperatingSection == nil {
		is.NonOperatingSection = &edgar.NonOperatingSection{
			InterestExpense: is.InterestExpense,
		}
	}

	// 4. Tax Section
	if is.TaxAdjustments == nil {
		is.TaxAdjustments = &edgar.TaxAdjustmentsSection{
			IncomeTaxExpense: is.IncomeTaxExpense,
		}
	}

	// 5. Net Income Section
	if is.NetIncomeSection == nil {
		is.NetIncomeSection = &edgar.NetIncomeSection{
			// NetIncome map?
		}
		// Usually NetIncome is derived, but we need it for history check
	}

	yearData := &edgar.YearData{
		BalanceSheet:      resp.BalanceSheet,
		IncomeStatement:   resp.IncomeStatement, // Now patched
		CashFlowStatement: resp.CashFlowStatement,
		SupplementalData:  resp.SupplementalData,
	}

	// 2. Run Common Size (The Logic Being Verified)
	fmt.Println(">>> Step 1: Calculating Common Size Drivers...")
	defaults := calc.CalculateCommonSizeDefaults(yearData)

	fmt.Printf("   Revenue Growth Default: %.2f%%\n", defaults.RevenueAction*100)
	fmt.Printf("   COGS %%: %.2f%%\n", defaults.COGSPercent*100)
	fmt.Printf("   SG&A %%: %.2f%%\n", defaults.SGAPercent*100)
	fmt.Printf("   R&D %%: %.2f%%\n", defaults.RDPercent*100)
	fmt.Printf("   Receivables %%: %.2f%%\n", defaults.ReceivablesPercent*100)
	fmt.Printf("   Inventory %%: %.2f%%\n", defaults.InventoryPercent*100)
	fmt.Printf("   AP %%: %.2f%%\n", defaults.APPercent*100)
	fmt.Printf("   Deferred Rev %%: %.2f%%\n", defaults.DeferredRevPercent*100)

	// Check that we got non-zero values for Balance Sheet drivers if data exists
	if defaults.ReceivablesPercent == 0 && resp.BalanceSheet.CurrentAssets.AccountsReceivableNet != nil {
		t.Log("WARNING: Receivables % is 0.00% despite AR data existing. Revenue might be 0?")
	}

	// 3. Projections
	fmt.Println(">>> Step 2: Projecting Financials (5 Years)...")

	// Initialize Engine
	skeleton := projection.NewStandardSkeleton()
	engine := projection.NewProjectionEngine(skeleton)

	// Convert Assumptions
	assumptions := projection.ProjectionAssumptions{
		RevenueGrowth:          0.05, // Hardcode 5% growth
		COGSPercent:            defaults.COGSPercent,
		SGAPercent:             defaults.SGAPercent,
		RDPercent:              defaults.RDPercent,
		TaxRate:                0.21,
		ReceivablesPercent:     defaults.ReceivablesPercent,
		InventoryPercent:       defaults.InventoryPercent,
		AccountsPayablePercent: defaults.APPercent,          // Map APPercent
		DeferredRevenuePercent: defaults.DeferredRevPercent, // Map DeferredRevPercent
		DebtInterestRate:       defaults.DebtInterestRate,
		TerminalGrowth:         0.025, // 2.5% Terminal Growth
		SharesOutstanding:      15000,
	}

	var projections []*projection.ProjectedFinancials
	currentIS := &yearData.IncomeStatement
	currentBS := &yearData.BalanceSheet
	startYear := 2024

	for i := 1; i <= 5; i++ {
		targetYear := startYear + i
		// Pass clean previous pointers. Because currentIS was patched, it has sections.
		proj := engine.ProjectYear(currentIS, currentBS, nil, assumptions, targetYear)
		projections = append(projections, proj)

		currentIS = proj.IncomeStatement
		currentBS = proj.BalanceSheet
	}

	if len(projections) != 5 {
		t.Fatalf("Expected 5 years of projections, got %d", len(projections))
	}

	fmt.Printf("   Yr1 Revenue: $%.0fM\n", convertVal(projections[0].IncomeStatement.GrossProfitSection.Revenues)/1e6)
	fmt.Printf("   Yr5 Revenue: $%.0fM\n", convertVal(projections[4].IncomeStatement.GrossProfitSection.Revenues)/1e6)

	// 4. Valuation
	fmt.Println(">>> Step 3: Performing DCF Valuation...")

	dcfInput := valuation.DCFInput{
		Projections:       projections,
		WACC:              0.10,
		TerminalGrowth:    0.03,
		SharesOutstanding: 15200,
		NetDebt:           0,
		TaxRate:           0.21,
	}

	valuationRes := valuation.CalculateDCF(dcfInput)

	fmt.Printf("   Enterprise Value: $%.0fM\n", valuationRes.EnterpriseValue/1e6)
	fmt.Printf("   Equity Value: $%.0fM\n", valuationRes.EquityValue/1e6)
	fmt.Printf("   Implied Share Price: $%.2f\n", valuationRes.SharePrice)

	if valuationRes.EnterpriseValue == 0 {
		t.Errorf("Enterprise Value is 0. Check calculation.")
	}
}

// Helper to safely get value for printing
func convertVal(v *edgar.FSAPValue) float64 {
	if v != nil && v.Value != nil {
		return *v.Value
	}
	return 0.0
}
