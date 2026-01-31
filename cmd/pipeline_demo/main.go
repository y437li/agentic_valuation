package main

import (
	"agentic_valuation/pkg/core/agent"
	"agentic_valuation/pkg/core/debate"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/projection"
	"agentic_valuation/pkg/core/prompt"
	"agentic_valuation/pkg/core/valuation"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Logger helper
func logStep(step string, details string) {
	fmt.Printf("\n[STEP] %s\n", step)
	fmt.Println("---------------------------------------------------------")
	fmt.Println(details)
	fmt.Println("---------------------------------------------------------")
}

func main() {
	logStep("0. Initialization", "Starting End-to-End Valuation Pipeline Demo...")

	// Initialize Prompt Library
	if err := prompt.LoadFromDirectory("resources"); err != nil {
		fmt.Printf("Warning: Failed to load prompts from 'resources': %v\n", err)
	} else {
		fmt.Println("✅ Prompt Library Loaded")
	}

	// =========================================================================
	// STEP 1: LOAD HISTORICAL DATA (T-0) - REAL DATA MODE
	// =========================================================================
	dataPath := filepath.Join("batch_data", "TSLA", "TSLA_FY2025.json")
	report, err := loadRealData(dataPath)
	if err != nil {
		fmt.Printf("Error loading data from %s: %v\n", dataPath, err)
		return
	}

	scaleToAbsoluteUnits(report)
	NormalizeReport(report) // Prepare data for Projection Engine

	// --- 1.1 SELF-SEED HISTORICAL DATA (Required for Quant Agent) ---
	// Since we loaded a single-year JSON without the "historical_data" map,
	// we must populate it so the Quant Agent can find the "latest year".
	// Self-seed HistoricalData for the single year (Required for Quant Agent)
	// NOTE: We must clone the data structure to ensure independent references if needed,
	// but here we just need the map populated for the Quant Agent to "see" the history.
	// Since we scaled the data above, this historical entry will also use Absolute Units.
	if report.HistoricalData == nil {
		report.HistoricalData = make(map[int]edgar.YearData)
	}
	report.HistoricalData[report.FiscalYear] = edgar.YearData{
		BalanceSheet:      report.BalanceSheet,
		IncomeStatement:   report.IncomeStatement,
		CashFlowStatement: report.CashFlowStatement,
		SupplementalData:  report.SupplementalData,
	}

	// 1.2 LOAD ADDITIONAL HISTORY (FY2022, FY2023)
	// This fixes the "Single Year" limitation by injecting real past data.
	for year := 2024; year < 2025; year++ {
		histPath := filepath.Join("batch_data", "TSLA", fmt.Sprintf("TSLA_FY%d.json", year))
		histReport, err := loadRealData(histPath)
		if err == nil {
			scaleToAbsoluteUnits(histReport)
			report.HistoricalData[year] = edgar.YearData{
				BalanceSheet:      histReport.BalanceSheet,
				IncomeStatement:   histReport.IncomeStatement,
				CashFlowStatement: histReport.CashFlowStatement,
				SupplementalData:  histReport.SupplementalData,
			}
			fmt.Printf(" [Data] Loaded Historical Context: FY%d\n", year)
		} else {
			fmt.Printf(" [Data] Warning: Could not load history for FY%d: %v\n", year, err)
		}
	}
	fmt.Printf(" [Data] Self-seeded HistoricalData for FY%d\n", report.FiscalYear)

	// Extract Previous Year Statements
	prevIS := &report.IncomeStatement
	prevBS := &report.BalanceSheet

	// Print Key Stats from Real Data - nil-safe accessors for Tesla compatibility
	getVal := func(v *edgar.FSAPValue) float64 {
		if v != nil && v.Value != nil {
			return *v.Value
		}
		return 0
	}

	rev := getVal(prevIS.GrossProfitSection.Revenues)
	netIncome := getVal(prevIS.NetIncomeSection.NetIncomeToCommon)

	// Nil-safe totalAssets - use ReportedForValidation directly
	totalAssets := 0.0
	if prevBS.ReportedForValidation.TotalAssets != nil && prevBS.ReportedForValidation.TotalAssets.Value != nil {
		totalAssets = *prevBS.ReportedForValidation.TotalAssets.Value
	}

	debt := getVal(prevBS.NoncurrentLiabilities.LongTermDebt) + getVal(prevBS.CurrentLiabilities.NotesPayableShortTermDebt)

	equity := 0.0
	if prevBS.ReportedForValidation.TotalEquity != nil && prevBS.ReportedForValidation.TotalEquity.Value != nil {
		equity = *prevBS.ReportedForValidation.TotalEquity.Value
	}

	cash := getVal(prevBS.CurrentAssets.CashAndEquivalents)

	logStep("1. Historical Data Loaded (Real Data)", fmt.Sprintf(
		"Source: %s\nRevenue: $%.2fM\nNet Income: $%.2fM\nTotal Assets: $%.2fM\nDebt: $%.2fM\nEquity: $%.2fM\nCash: $%.2fM",
		dataPath, rev, netIncome, totalAssets, debt, equity, cash))

	// =========================================================================
	// STEP 2: COGNITIVE LAYER (REAL DATA + AGENTS)
	// =========================================================================
	fmt.Println("\n--- Step 2: Cognitive Layer (Debate Agents) ---")

	// Initialize Agent Manager (Mock/Simulation Mode for safety)
	// [MODIFIED] Using Qwen for Real Debate
	os.Setenv("QWEN_API_KEY", "sk-8a84fa93d8934b46a825c1e0943ebe5b")
	agentConfig := agent.Config{
		ActiveProvider: "qwen",
		Agents:         make(map[string]agent.AgentConfig),
	}
	mgr := agent.NewManager(agentConfig)

	// Initialize Debate Orchestrator with Real Data Context
	// [MODIFIED] IsSimulation = false to trigger real LLM calls
	ticker := "TSLA" // NOTE: Extracted ticker from path
	orc := debate.NewOrchestrator("demo-session", ticker, report.Company, fmt.Sprintf("%d", report.FiscalYear), false, "automatic", mgr, nil)

	// Build Material Pool from Real Data
	// This makes the loaded AAPL data available to the agents
	poolBuilder := debate.NewMaterialPoolBuilder(report, nil)
	pool, err := poolBuilder.Build()

	// Variable to hold real extracted baselines
	var realBaselines map[string]float64
	var debateOutput string

	if err != nil {
		fmt.Printf("Error building Material Pool: %v\n", err)
		debateOutput = "Error: Material Pool failed."
	} else {
		orc.SharedContext.MaterialPool = pool
		fmt.Printf(" [Agents] Material Pool populated with %s %d Data\n", ticker, report.FiscalYear)

		// RUN THE DEBATE (Triggers Phase 0 Quant Agent + Phase 1/2 LLM Debate)
		fmt.Println(" [Agents] Running Debate Orchestrator (REAL LLM - Qwen)...")
		ctx := context.Background()

		// [MODIFIED] Subscribe to Real-Time Updates so the user can see the debate
		msgChan, _ := orc.Subscribe()

		// Launch background printer
		go func() {
			for msg := range msgChan {
				// Format based on message type
				prefix := "  "
				if msg.AgentRole == "moderator" {
					prefix = "\n[SYSTEM] "
				} else {
					prefix = fmt.Sprintf("\n[%s] %s:\n  ", msg.AgentRole, msg.AgentName)
				}
				fmt.Printf("%s%s\n", prefix, msg.Content)
			}
		}()

		orc.Run(ctx)

		// Extract REAL baseline assumptions generated by the Quant Agent
		realBaselines = orc.SharedContext.BaselineAssumptions
	}

	var quantOutput string
	if len(realBaselines) > 0 {
		quantOutput = fmt.Sprintf("Quant Agent Calculated Baselines:\n"+
			"- Revenue Growth: %.1f%%\n"+
			"- COGS Margin:    %.1f%%\n"+
			"- R&D Intensity:  %.1f%%",
			realBaselines["rev_growth"]*100,
			realBaselines["cogs_pct"]*100,
			realBaselines["rd_pct"]*100)
	} else {
		quantOutput = "Quant Agent failed to generate baselines."
	}

	// Create a hybrid report context for logging
	if orc.FinalReport != nil && orc.FinalReport.ExecutiveSummary != "" {
		debateOutput = fmt.Sprintf(
			"## Executive Summary (Powered by Real Qwen Debate)\n\n%s\n\n"+
				"**Financial Baselines (Actuals)**:\n%s",
			orc.FinalReport.ExecutiveSummary,
			quantOutput,
		)
	} else {
		// Fallback if LLM fails
		debateOutput = fmt.Sprintf(
			"## Executive Summary (Fallback)\n\n"+
				"**Strategic Outlook**:\n"+
				"Apple continues to dominate. (Simulation Fallback - LLM Output Empty)\n\n"+
				"**Financial Baselines (Actuals)**:\n%s",
			quantOutput,
		)
	}

	logStep("2. Roundtable Debate (Cognitive)", "Synthesizer produced Final Report:\n"+debateOutput)

	// =========================================================================
	// STEP 3: BRIDGE LAYER (ADAPTER)
	// =========================================================================
	mockReport := &debate.FinalDebateReport{ExecutiveSummary: debateOutput}
	assumptions := projection.ConvertDebateReportToAssumptions(mockReport)

	if len(realBaselines) > 0 {
		fmt.Println(" [Bridge] Overriding defaults with REAL Quant Agent assumptions")
		assumptions.RevenueGrowth = realBaselines["rev_growth"]
		assumptions.COGSPercent = realBaselines["cogs_pct"]

		// Map Aggregate OpEx (Critical for Valuation Integrity)
		assumptions.SGAPercent = realBaselines["sga_pct"]
		assumptions.RDPercent = realBaselines["rd_pct"]

		// ZERO OUT granular drivers to prevent Engine from preferring them over Aggregate SGA.
		// If these are non-zero (from Debate Hallucination), the Engine ignores SGAPercent.
		assumptions.SellingMarketingPercent = 0
		assumptions.GeneralAdminPercent = 0

		assumptions.TaxRate = realBaselines["tax_rate"]
		if v, ok := realBaselines["stock_based_comp_percent"]; ok {
			// Assumptions struct might check distinct field or map it later,
			// for now we stick to the main drivers.
			_ = v
		}
	} else {
		fmt.Println(" [Bridge] Using default mock assumptions (Quant Agent failed)")
	}

	if report.SupplementalData.SharesOutstandingBasic != nil && report.SupplementalData.SharesOutstandingBasic.Value != nil {
		assumptions.SharesOutstanding = *report.SupplementalData.SharesOutstandingBasic.Value
	} else {
		fmt.Println("Warning: Shares Outstanding not found in real data, using default.")
		assumptions.SharesOutstanding = 100.0
	}

	logStep("3. Adapter Conversion (Bridge)", fmt.Sprintf(
		"Extracted Driver Code Values:\n- Rev Growth: %.2f%%\n- COGS: %.2f%%\n- Unlevered Beta: %.2f\n- Target D/E: %.2f",
		assumptions.RevenueGrowth*100, assumptions.COGSPercent*100, assumptions.UnleveredBeta, assumptions.TargetDebtEquity))

	// =========================================================================
	// STEP 4: PROJECTION ENGINE (QUANTITATIVE)
	// =========================================================================
	engine := projection.NewProjectionEngine(&projection.StandardSkeleton{})

	projections := make([]*projection.ProjectedFinancials, 5)
	currIS := prevIS
	currBS := prevBS

	fmt.Println("\n[STEP] 4. Detailed Financial Statement Articulation")
	fmt.Println("---------------------------------------------------------")

	for i := 0; i < 5; i++ {
		proj := engine.ProjectYear(currIS, currBS, nil, assumptions, 2025+i)
		projections[i] = proj

		fmt.Printf("\n--- Year %d ---\n", 2025+i)

		// Income Statement detail
		is := proj.IncomeStatement
		revVal := *is.GrossProfitSection.Revenues.Value
		cogsVal := *is.GrossProfitSection.CostOfGoodsSold.Value
		gpVal := *is.GrossProfitSection.GrossProfit.Value
		opIncVal := *is.OperatingCostSection.OperatingIncome.Value
		netIncVal := *is.NetIncomeSection.NetIncomeToCommon.Value
		epsVal := *is.NetIncomeSection.EPSBasic.Value

		fmt.Printf("IS: Rev $%.1f -> GP $%.1f (Margin %.1f%%) [COGS: $%.1f] -> OpInc $%.1f -> NI $%.1f (EPS: $%.2f)\n",
			revVal, gpVal, (gpVal/revVal)*100, cogsVal, opIncVal, netIncVal, epsVal)

		// Balance Sheet Check
		bs := proj.BalanceSheet
		assets := *bs.NoncurrentAssets.CalculatedTotal
		liab := *bs.NoncurrentLiabilities.LongTermDebt.Value // Simplified check
		equity := *bs.Equity.RetainedEarningsDeficit.Value + *bs.Equity.CommonStockAPIC.Value
		// Note: CalculatedTotal in engine usually aggregates everything.
		// For demo, let's trust the engine's internal checks but print the cash position which is the plug.
		cashPos := *bs.CurrentAssets.CashAndEquivalents.Value
		fmt.Printf("BS: Cash Plug $%.1f | Assets $%.1f | Debt (LT) $%.1f | Equity $%.1f\n",
			cashPos, assets, liab, equity)

		currIS = proj.IncomeStatement
		currBS = proj.BalanceSheet
	}

	// =========================================================================
	// STEP 5: DYNAMIC WACC RE-CALCULATION
	// =========================================================================
	waccInput := valuation.WACCInput{
		UnleveredBeta:     assumptions.UnleveredBeta,
		RiskFreeRate:      assumptions.RiskFreeRate,
		MarketRiskPremium: assumptions.MarketRiskPremium,
		PreTaxCostOfDebt:  assumptions.PreTaxCostOfDebt,
		TaxRate:           assumptions.TaxRate,
		DebtToEquityRatio: assumptions.TargetDebtEquity, // Base target
	}

	// Generate Series
	waccSeries := valuation.GenerateDynamicWACCSeries(waccInput, projections)

	fmt.Println("\n[STEP] 5. Dynamic WACC Calculation (Iterative Process)")
	fmt.Println("---------------------------------------------------------")
	fmt.Printf("%-6s | %-10s | %-10s | %-10s | %-10s\n", "Year", "Debt", "Equity", "D/E Ratio", "WACC")
	for i, w := range waccSeries {
		// Re-extract d/e for display
		p := projections[i]
		d := *p.BalanceSheet.NoncurrentLiabilities.LongTermDebt.Value
		e := *p.BalanceSheet.Equity.RetainedEarningsDeficit.Value + *p.BalanceSheet.Equity.CommonStockAPIC.Value
		fmt.Printf("%-6d | $%-9.1f | $%-9.1f | %-10.3f | %.2f%%\n",
			2025+i, d, e, d/e, w*100)
	}

	// =========================================================================
	// STEP 6: MASTER VALUATION SUITE
	// =========================================================================
	fmt.Println("\n[STEP] 6. Master Valuation Suite (Detailed Build-up)")
	fmt.Println("---------------------------------------------------------")

	// Extract final WACC for display
	finalWACC := waccSeries[len(waccSeries)-1]

	masterInput := valuation.MasterValuationInput{
		Projections:       projections,
		SharesOutstanding: assumptions.SharesOutstanding,
		// Net Debt = Debt - Cash
		NetDebt:          (debt - cash),
		CurrentBookValue: equity,
		WACC:             0.08, // Initial guess, overridden below
		PeriodWACCs:      waccSeries,
		CostOfEquity:     0.10, // Initial guess
		TerminalGrowth:   assumptions.TerminalGrowth,
		TaxRate:          assumptions.TaxRate,
	}

	initialWACCRes := valuation.CalculateWACC(waccInput)
	masterInput.CostOfEquity = initialWACCRes.CostOfEquity

	results := valuation.RunAllValuations(masterInput)

	// --- CLI VISUALIZATION (CLAUDE CODE STYLE) ---
	fmt.Println("\n🔮 VALUATION MODEL OUTPUT")
	fmt.Println("=========================================================")

	// Re-run DCF explicitly to get detailed struct (ValuationLineItem is too opaque)
	dcfInput := valuation.DCFInput{
		Projections:       projections,
		WACC:              finalWACC, // Use final WACC or average? Usually final for TV.
		TerminalGrowth:    assumptions.TerminalGrowth,
		SharesOutstanding: assumptions.SharesOutstanding,
		NetDebt:           masterInput.NetDebt,
		TaxRate:           assumptions.TaxRate,
		PeriodWACCs:       waccSeries, // Pass dynamic WACC series if supported
	}
	// Using simple DCF or period WACC? The engine handles it.
	dcfDetail := valuation.CalculateDCF(dcfInput)

	if dcfDetail.SharePrice > 0 {
		fmt.Printf("\n🔹 MODEL: DCF (Free Cash Flow Method)\n")
		fmt.Println("   ---------------------------------------------")

		// Terminal Value Calculation Display
		finalWACCPercent := waccSeries[len(waccSeries)-1]

		fmt.Printf("   📌 Terminal Value Calculation:\n")
		fmt.Printf("      Formula: [Final FCF * (1 + g)] / (WACC - g)\n")
		fmt.Printf("      Inputs:  g=%.2f%%, WACC(T5)=%.2f%%\n", assumptions.TerminalGrowth*100, finalWACCPercent*100)
		fmt.Printf("      --> TERMINAL VALUE: $%.2f M\n", dcfDetail.PV_Terminal)

		fmt.Printf("   ---------------------------------------------")
		fmt.Printf("\n   💵 Enterprise Value:    $%.2f M\n", dcfDetail.EnterpriseValue)
		fmt.Printf("   ➖ Net Debt:            $%.2f M\n", dcfInput.NetDebt)
		fmt.Printf("   =============================================\n")
		fmt.Printf("   💎 EQUITY VALUE:        $%.2f M\n", dcfDetail.EquityValue)
		fmt.Printf("   ÷  Shares Outstanding:  %.2f M\n", dcfInput.SharesOutstanding/1000000)
		fmt.Printf("   ---------------------------------------------\n")
		fmt.Printf("   🚀 IMPLIED SHARE PRICE: $%.2f\n", dcfDetail.SharePrice)
		fmt.Printf("   =============================================\n")

		fmt.Printf("\n   [Implied Exit Multiple: %.1fx EBITDA]\n", dcfDetail.ImpliedMultiple)
	}

	// Just list others concisely
	fmt.Println("\n📋 OTHER MODEL COMPARISONS")
	for _, res := range results {
		if res.ModelName != "Free Cash Flow for All Debt and Equity Valuation" {
			fmt.Printf("   - %-40s : $%.2f\n", res.ModelName, res.SharePrice)
		}
	}
}

// Helper: Load Real Data
func loadRealData(path string) (*edgar.FSAPDataResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Trim BOM if present
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	var report edgar.FSAPDataResponse
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// scaleToAbsoluteUnits multiplies financial figures (in Millions) by 1,000,000
// and Share Counts (in Thousands) by 1,000 to convert them to absolute units.
func scaleToAbsoluteUnits(report *edgar.FSAPDataResponse) {
	scale := func(v *edgar.FSAPValue, multiplier float64) {
		if v != nil && v.Value != nil {
			*v.Value = (*v.Value) * multiplier
			for y, val := range v.Years {
				v.Years[y] = val * multiplier
			}
		}
	}

	// 1. Income Statement (Millions -> Dollars)
	scale(report.IncomeStatement.Revenues, 1e6)
	scale(report.IncomeStatement.CostOfGoodsSold, 1e6)
	scale(report.IncomeStatement.SGAExpenses, 1e6)
	scale(report.IncomeStatement.RDExpenses, 1e6)
	scale(report.IncomeStatement.InterestExpense, 1e6)
	scale(report.IncomeStatement.IncomeTaxExpense, 1e6)

	scale(report.IncomeStatement.ReportedForValidation.GrossProfit, 1e6)
	scale(report.IncomeStatement.ReportedForValidation.OperatingIncome, 1e6)
	scale(report.IncomeStatement.ReportedForValidation.IncomeBeforeTax, 1e6)
	scale(report.IncomeStatement.ReportedForValidation.NetIncome, 1e6)

	// 2. Balance Sheet (Millions -> Dollars)
	bs := &report.BalanceSheet
	scale(bs.CurrentAssets.CashAndEquivalents, 1e6)
	scale(bs.CurrentAssets.ShortTermInvestments, 1e6)
	scale(bs.CurrentAssets.AccountsReceivableNet, 1e6)
	scale(bs.CurrentAssets.Inventories, 1e6)

	scale(bs.NoncurrentAssets.PPEAtCost, 1e6)
	scale(bs.NoncurrentAssets.AccumulatedDepreciation, 1e6)
	scale(bs.NoncurrentAssets.PPENet, 1e6)
	scale(bs.NoncurrentAssets.LongTermInvestments, 1e6)
	scale(bs.NoncurrentAssets.DeferredTaxAssetsLT, 1e6)

	scale(bs.CurrentLiabilities.AccountsPayable, 1e6)
	scale(bs.CurrentLiabilities.CurrentMaturitiesLTD, 1e6)

	scale(bs.NoncurrentLiabilities.LongTermDebt, 1e6)
	scale(bs.NoncurrentLiabilities.DeferredTaxLiabilities, 1e6)

	scale(bs.Equity.CommonStockAPIC, 1e6)
	scale(bs.Equity.RetainedEarningsDeficit, 1e6)
	scale(bs.Equity.AccumOtherComprehensiveIncome, 1e6)

	scale(bs.ReportedForValidation.TotalAssets, 1e6)
	scale(bs.ReportedForValidation.TotalEquity, 1e6)
	scale(bs.ReportedForValidation.TotalLiabilities, 1e6)

	// 3. Cash Flow (Millions -> Dollars)
	cf := &report.CashFlowStatement
	scale(cf.NetIncomeStart, 1e6)
	scale(cf.DepreciationAmortization, 1e6)
	scale(cf.StockBasedCompensation, 1e6)
	scale(cf.Capex, 1e6)
	scale(cf.Dividends, 1e6)
	scale(cf.ShareRepurchases, 1e6)
	scale(cf.InvestmentsAcquiredSoldNet, 1e6)
	scale(cf.DebtIssuanceRetirementNet, 1e6)

	// Validation fields (struct is embedded, so directly access fields)
	scale(cf.ReportedForValidation.NetCashOperating, 1e6)
	scale(cf.ReportedForValidation.NetCashInvesting, 1e6)
	scale(cf.ReportedForValidation.NetCashFinancing, 1e6)

	// 4. Shares (Thousands -> Units)
	scale(report.SupplementalData.SharesOutstandingBasic, 1000)
	scale(report.SupplementalData.SharesOutstandingDiluted, 1000)

	fmt.Println(" [Data] Scaled extracted financials to absolute units (Financials: x1M, Shares: x1k)")
}

// Helper: Normalize Legacy Flat JSON to Nested Sections (v2.0)
func NormalizeReport(report *edgar.FSAPDataResponse) {
	is := &report.IncomeStatement
	// bs := &report.BalanceSheet

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
		// Ensure fields are mapped if they exist in legacy flat structure but missed in nested
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

	if is.NetIncomeSection == nil {
		// Try to find EPS in Supplemental Data first
		epsBasic := report.SupplementalData.EPSBasic
		epsDiluted := report.SupplementalData.EPSDiluted
		shares := report.SupplementalData.SharesOutstandingBasic // Unused here but kept for context

		// If missing in Supplemental, try IS Reported
		if epsBasic == nil && is.ReportedForValidation.NetIncome != nil {
			// fallback logic could go here
		}

		is.NetIncomeSection = &edgar.NetIncomeSection{
			NetIncomeToCommon:     is.ReportedForValidation.NetIncome,
			EPSBasic:              epsBasic,
			EPSDiluted:            epsDiluted,
			WeightedAverageShares: shares, // Mapping Shares to NetIncomeSection for Calc
		}
	}
}
