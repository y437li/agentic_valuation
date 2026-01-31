package main

import (
	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/edgar"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	dataPath := filepath.Join("batch_data", "AAPL", "AAPL_FY2024.json")
	report, err := loadRealData(dataPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("--- Pre-Scaling ---")
	printStats(report)

	// PATCH DATA IF MISSING R&D (Before Scaling)
	is := &report.IncomeStatement
	if is.RDExpenses == nil || is.RDExpenses.Value == nil {
		fmt.Println(" [Patch] R&D missing. Patching with FY2022 value: 26251")
		val := 26251.0
		is.RDExpenses = &edgar.FSAPValue{
			Value:   &val,
			Years:   map[string]float64{"current": val},
			Label:   "us-gaap:ResearchAndDevelopmentExpense",
			XBRLTag: "us-gaap:ResearchAndDevelopmentExpense",
		}
		savePatchedData(dataPath, report)
	}

	scaleToAbsoluteUnits(report)
	fmt.Println("--- Post-Scaling ---")
	printStats(report)

	NormalizeReport(report)
	fmt.Println("--- Post-Normalization ---")

	// Debug directly after normalization
	if report.CashFlowStatement.OperatingActivities != nil {
		fmt.Println("DEBUG: report.CF.OperatingActivities is POPULATED")
	} else {
		fmt.Println("DEBUG: report.CF.OperatingActivities is NIL after Normalization!")
	}

	// DEBUG: Trace R&D
	fmt.Printf("DEBUG: IS.RDExpenses: %v\n", getValue(report.IncomeStatement.RDExpenses))
	if report.IncomeStatement.OperatingCostSection != nil {
		fmt.Printf("DEBUG: IS.OperatingCostSection.RD: %v\n", getValue(report.IncomeStatement.OperatingCostSection.RDExpenses))
	} else {
		fmt.Println("DEBUG: IS.OperatingCostSection is NIL")
	}

	printNestedStats(report)

	// Seed History
	// Verify History data

	if report.HistoricalData == nil {
		report.HistoricalData = make(map[int]edgar.YearData)
	}
	report.HistoricalData[report.FiscalYear] = edgar.YearData{
		BalanceSheet:      report.BalanceSheet,
		IncomeStatement:   report.IncomeStatement,
		CashFlowStatement: report.CashFlowStatement,
		SupplementalData:  report.SupplementalData,
	}

	// Verify History data
	hist := report.HistoricalData[report.FiscalYear]
	fmt.Println("--- History Data (Fed to Quant) ---")
	// Check nested fields in history
	if hist.IncomeStatement.GrossProfitSection != nil {
		fmt.Printf("History.IS.GrossProfitSection.Revenues: %v\n", getValue(hist.IncomeStatement.GrossProfitSection.Revenues))
	}
	if hist.CashFlowStatement.OperatingActivities != nil {
		fmt.Printf("History.CF.OperatingActivities.SBC: %v\n", getValue(hist.CashFlowStatement.OperatingActivities.StockBasedCompensation))
		fmt.Printf("History.CF.OperatingActivities.D&A: %v\n", getValue(hist.CashFlowStatement.OperatingActivities.DepreciationAmortization))
	} else {
		fmt.Println("History.CF.OperatingActivities IS NIL!")
	}
	if hist.CashFlowStatement.InvestingActivities != nil {
		fmt.Printf("History.CF.InvestingActivities.Capex: %v\n", getValue(hist.CashFlowStatement.InvestingActivities.Capex))
	} else {
		fmt.Println("History.CF.InvestingActivities IS NIL!")
	}

	// Inject Fake Dynamic Item for Demonstration
	// Note: Data is already scaled to absolute units (e.g. 3.94e11 for Rev)
	val := 1.97e10 // ~5% of Revenue
	is.OperatingCostSection.AdditionalItems = append(is.OperatingCostSection.AdditionalItems, edgar.AdditionalItem{
		Label: "Legal Settlement (Demo)",
		Value: &edgar.FSAPValue{Value: &val},
	})

	// Run Calc
	defaults := calc.CalculateCommonSizeDefaults(&hist)
	// --- Full Model View Generation ---
	fmt.Println("\n====================================================================================================")
	fmt.Println("                            T.I.E.D.  COMMON SIZE  ANALYSIS  (FY2024)")
	fmt.Println("====================================================================================================")
	fmt.Printf("%-35s | %15s | %15s | %s\n", "LINE ITEM", "VALUE ($M)", "% OF REV", "DRIVER LOGIC")
	fmt.Println("----------------------------------------------------------------------------------------------------")

	// Helper to print row
	pRow := func(label string, val float64, pct float64, logic string) {
		fmt.Printf("%-35s | %15.1f | %14.1f%% | %s\n", label, val/1e6, pct*100, logic)
	}

	rev := *report.IncomeStatement.GrossProfitSection.Revenues.Value
	pRow("Total Net Sales", rev, 1.0, "Input")
	pRow("Cost of Goods Sold", *report.IncomeStatement.GrossProfitSection.CostOfGoodsSold.Value, defaults.COGSPercent, "% of Revenue")
	pRow("Gross Profit", *report.IncomeStatement.GrossProfitSection.GrossProfit.Value, (*report.IncomeStatement.GrossProfitSection.GrossProfit.Value)/rev, "Calculated")

	fmt.Println("----------------------------------------------------------------------------------------------------")
	pRow("Research & Development", *report.IncomeStatement.OperatingCostSection.RDExpenses.Value, defaults.RDPercent, "% of Revenue")
	pRow("Selling, General & Admin", *report.IncomeStatement.OperatingCostSection.SGAExpenses.Value, defaults.SGAPercent, "% of Revenue")

	// Print Dynamic Items in OpEx
	for k, v := range defaults.CustomItems {
		// Try to find value magnitude for demo (back-calculated)
		pRow(k, v*rev, v, "% of Revenue (Dynamic)")
	}

	if report.IncomeStatement.OperatingCostSection.OtherOperatingExpenses != nil && report.IncomeStatement.OperatingCostSection.OtherOperatingExpenses.Value != nil {
		pRow("Other Operating Expenses", *report.IncomeStatement.OperatingCostSection.OtherOperatingExpenses.Value, defaults.OtherOperatingPercent, "% of Revenue")
	}

	fmt.Println("----------------------------------------------------------------------------------------------------")
	pRow("Operating Income (EBIT)", *report.IncomeStatement.OperatingCostSection.OperatingIncome.Value, (*report.IncomeStatement.OperatingCostSection.OperatingIncome.Value)/rev, "Calculated")

	if report.IncomeStatement.NonOperatingSection.InterestExpense != nil && report.IncomeStatement.NonOperatingSection.InterestExpense.Value != nil {
		pRow("Interest Expense", *report.IncomeStatement.NonOperatingSection.InterestExpense.Value, defaults.DebtInterestRate, "% of Debt (Implied)")
	}
	pRow("Other Income/(Expense)", 0.0, defaults.OtherNonOpPercent, "% of Revenue") // AAPL is 0 here

	fmt.Println("----------------------------------------------------------------------------------------------------")
	pRow("Income Before Tax", *report.IncomeStatement.NonOperatingSection.IncomeBeforeTax.Value, (*report.IncomeStatement.NonOperatingSection.IncomeBeforeTax.Value)/rev, "Calculated")
	pRow("Income Tax Provision", *report.IncomeStatement.TaxAdjustments.IncomeTaxExpense.Value, defaults.TaxRate, "% of Pre-Tax Income")
	pRow("Net Income", *report.IncomeStatement.NetIncomeSection.NetIncomeToCommon.Value, defaults.NetIncomeMargin, "Calculated")

	fmt.Println("====================================================================================================")
	fmt.Println("                            CASH FLOW & MEMO DRIVERS")
	fmt.Println("====================================================================================================")
	pRow("Stock-Based Compensation", *report.CashFlowStatement.OperatingActivities.StockBasedCompensation.Value, defaults.StockBasedCompPercent, "% of Revenue")
	pRow("Depreciation & Amortization", *report.CashFlowStatement.OperatingActivities.DepreciationAmortization.Value, defaults.DAPercent, "% of Revenue")
	pRow("Capital Expenditures", -(*report.CashFlowStatement.InvestingActivities.Capex.Value), defaults.CapExPercent, "% of Revenue") // Show positive for driver
	pRow("Dividends Paid", -(*report.CashFlowStatement.FinancingActivities.DividendsPaid.Value), defaults.DividendsPercent, "% of Revenue")
	pRow("Share Repurchases", -(*report.CashFlowStatement.FinancingActivities.ShareRepurchases.Value), defaults.RepurchasePercent, "% of Revenue")

	fmt.Println("====================================================================================================")
	fmt.Println("                            BALANCE SHEET WORKING CAPITAL")
	fmt.Println("====================================================================================================")
	if report.BalanceSheet.CurrentAssets.AccountsReceivableNet != nil {
		pRow("Accounts Receivable (Net)", *report.BalanceSheet.CurrentAssets.AccountsReceivableNet.Value, defaults.ReceivablesPercent, "% of Revenue")
	}
	if report.BalanceSheet.CurrentAssets.Inventories != nil {
		pRow("Inventories", *report.BalanceSheet.CurrentAssets.Inventories.Value, defaults.InventoryPercent, "% of Revenue")
	}
	if report.BalanceSheet.CurrentLiabilities.AccountsPayable != nil {
		pRow("Accounts Payable", *report.BalanceSheet.CurrentLiabilities.AccountsPayable.Value, defaults.APPercent, "% of Revenue")
	}
	if report.BalanceSheet.CurrentLiabilities.DeferredRevenueCurrent != nil {
		pRow("Deferred Revenue (Current)", *report.BalanceSheet.CurrentLiabilities.DeferredRevenueCurrent.Value, defaults.DeferredRevPercent, "% of Revenue")
	}

	// Print Dynamic Items in BS
	for k, v := range defaults.CustomItems {
		if len(k) > 9 && (k[len(k)-10:] == "(BS-Asset)" || k[len(k)-9:] == "(BS-Liab)") {
			pRow(k, v*rev, v, "% of Revenue (Dynamic)")
		}
	}
	fmt.Println("====================================================================================================")
}

func loadRealData(path string) (*edgar.FSAPDataResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var report edgar.FSAPDataResponse
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

func scaleToAbsoluteUnits(report *edgar.FSAPDataResponse) {
	// Helper to scale a single FSAPValue
	scale := func(v *edgar.FSAPValue, multiplier float64) {
		if v != nil && v.Value != nil {
			*v.Value = *v.Value * multiplier
			// Also scale historical years in map if present
			if v.Years != nil {
				for y, val := range v.Years {
					v.Years[y] = val * multiplier
				}
			}
		}
	}

	const M = 1000000.0 // Millions
	const K = 1000.0    // Thousands

	// Income Statement
	is := &report.IncomeStatement
	scale(is.Revenues, M)
	scale(is.CostOfGoodsSold, M)
	scale(is.SGAExpenses, M)
	scale(is.RDExpenses, M)
	scale(is.ReportedForValidation.GrossProfit, M)
	scale(is.ReportedForValidation.OperatingIncome, M)
	scale(is.ReportedForValidation.NetIncome, M)
	scale(is.ReportedForValidation.IncomeBeforeTax, M)
	scale(is.IncomeTaxExpense, M)

	// Balance Sheet
	bs := &report.BalanceSheet
	scale(bs.CurrentAssets.CashAndEquivalents, M)
	scale(bs.NoncurrentLiabilities.LongTermDebt, M)

	// Supplemental
	supp := &report.SupplementalData
	scale(supp.SharesOutstandingBasic, K)

	// Cash Flow (Legacy Flat Fields)
	cf := &report.CashFlowStatement
	scale(cf.DepreciationAmortization, M)
	scale(cf.StockBasedCompensation, M)
	scale(cf.Capex, M)
	scale(cf.Dividends, M)
	scale(cf.ShareRepurchases, M)

	fmt.Printf("DEBUG: Scaled SBC: %v\n", getValue(cf.StockBasedCompensation))
}

func NormalizeReport(report *edgar.FSAPDataResponse) {
	is := &report.IncomeStatement
	// 1. IS Normalization
	if is.GrossProfitSection == nil {
		is.GrossProfitSection = &edgar.GrossProfitSection{
			Revenues:        is.Revenues,
			CostOfGoodsSold: is.CostOfGoodsSold,
			GrossProfit:     is.ReportedForValidation.GrossProfit,
		}
	}
	if is.OperatingCostSection == nil {
		is.OperatingCostSection = &edgar.OperatingCostSection{
			SGAExpenses:     is.SGAExpenses,
			RDExpenses:      is.RDExpenses,
			OperatingIncome: is.ReportedForValidation.OperatingIncome,
		}
	} else {
		if is.OperatingCostSection.RDExpenses == nil {
			is.OperatingCostSection.RDExpenses = is.RDExpenses
		}
		if is.OperatingCostSection.SGAExpenses == nil {
			is.OperatingCostSection.SGAExpenses = is.SGAExpenses
		}
	}
	if is.NonOperatingSection == nil {
		is.NonOperatingSection = &edgar.NonOperatingSection{
			InterestExpense: is.InterestExpense,
			IncomeBeforeTax: is.ReportedForValidation.IncomeBeforeTax,
		}
	}
	if is.TaxAdjustments == nil {
		is.TaxAdjustments = &edgar.TaxAdjustmentsSection{
			IncomeTaxExpense: is.IncomeTaxExpense,
		}
	}

	// 2. CF Normalization (Legacy -> Nested)
	cf := &report.CashFlowStatement
	if cf.OperatingActivities == nil {
		cf.OperatingActivities = &edgar.CFOperatingSection{
			StockBasedCompensation:   cf.StockBasedCompensation,
			DepreciationAmortization: cf.DepreciationAmortization,
		}
	} else {
		if cf.OperatingActivities.StockBasedCompensation == nil {
			cf.OperatingActivities.StockBasedCompensation = cf.StockBasedCompensation
		}
		if cf.OperatingActivities.DepreciationAmortization == nil {
			cf.OperatingActivities.DepreciationAmortization = cf.DepreciationAmortization
		}
	}

	if cf.InvestingActivities == nil {
		cf.InvestingActivities = &edgar.CFInvestingSection{
			Capex: cf.Capex,
		}
	} else {
		if cf.InvestingActivities.Capex == nil {
			cf.InvestingActivities.Capex = cf.Capex
		}
	}

	if cf.FinancingActivities == nil {
		cf.FinancingActivities = &edgar.CFFinancingSection{
			DividendsPaid:    cf.Dividends,
			ShareRepurchases: cf.ShareRepurchases,
		}
	} else {
		if cf.FinancingActivities.DividendsPaid == nil {
			cf.FinancingActivities.DividendsPaid = cf.Dividends
		}
		if cf.FinancingActivities.ShareRepurchases == nil {
			cf.FinancingActivities.ShareRepurchases = cf.ShareRepurchases
		}
	}

	// 3. Net Income Section
	if is.NetIncomeSection == nil {
		is.NetIncomeSection = &edgar.NetIncomeSection{
			NetIncomeToCommon: is.ReportedForValidation.NetIncome,
		}
	}
}

func printStats(report *edgar.FSAPDataResponse) {
	is := &report.IncomeStatement
	fmt.Printf("IS.Revenues: %v\n", getValue(is.Revenues))
	fmt.Printf("IS.COGS:     %v\n", getValue(is.CostOfGoodsSold))
	fmt.Printf("IS.SGA:      %v\n", getValue(is.SGAExpenses))
	fmt.Printf("IS.RD:       %v\n", getValue(is.RDExpenses))
}

func printNestedStats(report *edgar.FSAPDataResponse) {
	is := &report.IncomeStatement
	if is.GrossProfitSection != nil {
		fmt.Printf("Nested.Rev:  %v\n", getValue(is.GrossProfitSection.Revenues))
		fmt.Printf("Nested.COGS: %v\n", getValue(is.GrossProfitSection.CostOfGoodsSold))
	}
	if is.OperatingCostSection != nil {
		fmt.Printf("Nested.SGA:  %v\n", getValue(is.OperatingCostSection.SGAExpenses))
	}
}

func getValue(v *edgar.FSAPValue) float64 {
	if v != nil && v.Value != nil {
		return *v.Value
	}
	return 0
}

func savePatchedData(path string, report *edgar.FSAPDataResponse) {
	data, err := json.MarshalIndent(report, "", "    ")
	if err != nil {
		fmt.Printf("Error marshaling patched data: %v\n", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Printf("Error saving patched data: %v\n", err)
		return
	}
	fmt.Println(" [Patch] Saved patched data to", path)
}
