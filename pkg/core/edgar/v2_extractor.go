package edgar

import (
	"context"
	"fmt"
	"strings"
)

// V2Extractor is the v2.0 Decoupled Extraction Pipeline entry point.
// Architecture: Navigator -> Mapper -> GoExtractor
// - Navigator: LLM parses TOC to find statement locations
// - Mapper: LLM identifies row semantics (LineItemMapping)
// - GoExtractor: Deterministic Go code extracts numeric values as []*FSAPValue
type V2Extractor struct {
	navigator *NavigatorAgent
	mapper    *TableMapperAgent
	extractor *GoExtractor
}

// NewV2Extractor creates a new v2.0 extraction pipeline
func NewV2Extractor(provider AIProvider) *V2Extractor {
	return &V2Extractor{
		navigator: NewNavigatorAgent(provider),
		mapper:    NewTableMapperAgent(provider),
		extractor: NewGoExtractor(),
	}
}

// statementConfig defines extraction parameters for each statement type
type statementConfig struct {
	name      string
	tableType string
	patterns  []string
}

// Extract performs full v2.0 extraction on markdown content
func (e *V2Extractor) Extract(ctx context.Context, markdown string, meta *FilingMetadata) (*FSAPDataResponse, error) {
	// Step 1: Parse TOC using NavigatorAgent
	toc, err := e.navigator.ParseTOC(ctx, markdown)
	if err != nil {
		fmt.Printf("Warning: NavigatorAgent.ParseTOC failed: %v (continuing with fallback patterns)\n", err)
	}

	// Statement configurations with fallback patterns
	statements := []statementConfig{
		{
			name:      "Income_Statement",
			tableType: "income_statement",
			patterns:  []string{"CONSOLIDATED STATEMENTS OF INCOME", "CONSOLIDATED STATEMENTS OF OPERATIONS", "STATEMENTS OF INCOME"},
		},
		{
			name:      "Balance_Sheet",
			tableType: "balance_sheet",
			patterns:  []string{"CONSOLIDATED BALANCE SHEETS", "CONSOLIDATED BALANCE SHEET"},
		},
		{
			name:      "Cash_Flow",
			tableType: "cash_flow",
			patterns:  []string{"CONSOLIDATED STATEMENTS OF CASH FLOWS", "STATEMENTS OF CASH FLOWS"},
		},
	}

	// Add LLM-discovered titles from TOC (priority)
	if toc != nil {
		if toc.IncomeStatement != nil && toc.IncomeStatement.Title != "" {
			statements[0].patterns = append([]string{toc.IncomeStatement.Title}, statements[0].patterns...)
		}
		if toc.BalanceSheet != nil && toc.BalanceSheet.Title != "" {
			statements[1].patterns = append([]string{toc.BalanceSheet.Title}, statements[1].patterns...)
		}
		if toc.CashFlow != nil && toc.CashFlow.Title != "" {
			statements[2].patterns = append([]string{toc.CashFlow.Title}, statements[2].patterns...)
		}
	}

	// Result container
	result := &FSAPDataResponse{
		FiscalYear: meta.FiscalYear,
		Company:    meta.CompanyName,
		CIK:        meta.CIK,
	}

	// Step 2: Extract each statement
	for _, stmt := range statements {
		values, err := e.extractStatement(ctx, markdown, stmt)
		if err != nil {
			fmt.Printf("Warning: %s extraction failed: %v\n", stmt.name, err)
			continue
		}
		fmt.Printf("  Extracted %d values for %s\n", len(values), stmt.name)

		// Map values to result structure using FSAPVariable
		MapFSAPValuesToResult(result, stmt.tableType, values)
	}

	// Populate Value fields from Years map for backwards compatibility
	populateValuesFromYears(result)

	return result, nil
}

// extractStatement extracts a single financial statement using v2.0 pattern
func (e *V2Extractor) extractStatement(ctx context.Context, markdown string, stmt statementConfig) ([]*FSAPValue, error) {
	// Find table position
	startLine := findTableLineV2(markdown, stmt.patterns)
	if startLine == 0 {
		return nil, fmt.Errorf("%s not found in document", stmt.name)
	}

	// Slice table content (60 lines should cover most tables)
	tableMarkdown := sliceLinesV2(markdown, startLine, startLine+60)

	// Step 2a: TableMapperAgent - LLM identifies row semantics
	mapping, err := e.mapper.MapTable(ctx, stmt.tableType, tableMarkdown)
	if err != nil {
		return nil, fmt.Errorf("MapTable failed: %w", err)
	}

	// Step 2b: GoExtractor - Parse table and extract values
	parsedTable := e.extractor.ParseMarkdownTableWithOffset(tableMarkdown, stmt.tableType, startLine)
	values := e.extractor.ExtractValues(parsedTable, mapping)

	return values, nil
}

// MapFSAPValuesToResult maps extracted FSAPValues to FSAPDataResponse structure
// Uses the FSAPVariable field to determine where each value belongs
func MapFSAPValuesToResult(result *FSAPDataResponse, tableType string, values []*FSAPValue) {
	for _, v := range values {
		if v == nil || v.FSAPVariable == "" || v.FSAPVariable == "UNIQUE" {
			continue // Skip unmapped items for now
		}

		switch tableType {
		case "income_statement":
			mapToIncomeStatementV2(&result.IncomeStatement, v)
		case "balance_sheet":
			mapToBalanceSheetV2(&result.BalanceSheet, v)
		case "cash_flow":
			mapToCashFlowV2(&result.CashFlowStatement, v)
		}
	}
}

// mapToIncomeStatementV2 maps an FSAPValue to the IncomeStatement structure
func mapToIncomeStatementV2(is *IncomeStatement, v *FSAPValue) {
	// Ensure sections are initialized
	if is.GrossProfitSection == nil {
		is.GrossProfitSection = &GrossProfitSection{}
	}
	if is.OperatingCostSection == nil {
		is.OperatingCostSection = &OperatingCostSection{}
	}
	if is.NonOperatingSection == nil {
		is.NonOperatingSection = &NonOperatingSection{}
	}
	if is.TaxAdjustments == nil {
		is.TaxAdjustments = &TaxAdjustmentsSection{}
	}
	if is.NetIncomeSection == nil {
		is.NetIncomeSection = &NetIncomeSection{}
	}

	// Map based on FSAPVariable key
	switch v.FSAPVariable {
	case "revenues", "net_sales", "total_revenue":
		is.GrossProfitSection.Revenues = v
	case "cost_of_goods_sold", "cost_of_revenue", "cost_of_sales":
		is.GrossProfitSection.CostOfGoodsSold = v
	case "gross_profit":
		is.GrossProfitSection.GrossProfit = v
	case "sga_expenses":
		is.OperatingCostSection.SGAExpenses = v
	case "rd_expenses", "research_and_development":
		is.OperatingCostSection.RDExpenses = v
	case "operating_income", "income_from_operations":
		is.OperatingCostSection.OperatingIncome = v
	case "interest_expense":
		is.NonOperatingSection.InterestExpense = v
	case "income_tax_expense", "provision_for_income_taxes":
		is.TaxAdjustments.IncomeTaxExpense = v // Note: adjust if needed
	case "net_income", "net_income_to_common":
		is.NetIncomeSection.NetIncomeToCommon = v
	}
}

// mapToBalanceSheetV2 maps an FSAPValue to the BalanceSheet structure
// Note: BalanceSheet uses non-pointer struct members
func mapToBalanceSheetV2(bs *BalanceSheet, v *FSAPValue) {
	switch v.FSAPVariable {
	// Current Assets
	case "cash_and_equivalents", "cash":
		bs.CurrentAssets.CashAndEquivalents = v
	case "short_term_investments":
		bs.CurrentAssets.ShortTermInvestments = v
	case "accounts_receivable", "accounts_receivable_net":
		bs.CurrentAssets.AccountsReceivableNet = v
	case "inventory", "inventories":
		bs.CurrentAssets.Inventories = v

	// Noncurrent Assets
	case "ppe_net", "property_plant_equipment":
		bs.NoncurrentAssets.PPENet = v
	case "intangible_assets", "intangibles":
		bs.NoncurrentAssets.Intangibles = v
	case "goodwill":
		bs.NoncurrentAssets.Goodwill = v

	// Current Liabilities
	case "accounts_payable":
		bs.CurrentLiabilities.AccountsPayable = v
	case "short_term_debt", "notes_payable":
		bs.CurrentLiabilities.NotesPayableShortTermDebt = v

	// Noncurrent Liabilities
	case "long_term_debt":
		bs.NoncurrentLiabilities.LongTermDebt = v

	// Equity
	case "common_stock", "common_stock_apic":
		bs.Equity.CommonStockAPIC = v
	case "retained_earnings", "retained_earnings_deficit":
		bs.Equity.RetainedEarningsDeficit = v

	// Validation Totals
	case "total_assets":
		bs.ReportedForValidation.TotalAssets = v
	case "total_liabilities":
		bs.ReportedForValidation.TotalLiabilities = v

	}
}

// mapToCashFlowV2 maps an FSAPValue to the CashFlowStatement structure
func mapToCashFlowV2(cf *CashFlowStatement, v *FSAPValue) {
	// Ensure sections are initialized
	if cf.OperatingActivities == nil {
		cf.OperatingActivities = &CFOperatingSection{}
	}
	if cf.InvestingActivities == nil {
		cf.InvestingActivities = &CFInvestingSection{}
	}
	if cf.FinancingActivities == nil {
		cf.FinancingActivities = &CFFinancingSection{}
	}
	if cf.CashSummary == nil {
		cf.CashSummary = &CashSummarySection{}
	}

	switch v.FSAPVariable {
	// Operating Section
	case "net_income", "net_income_start":
		cf.OperatingActivities.NetIncomeStart = v
	case "depreciation_amortization":
		cf.OperatingActivities.DepreciationAmortization = v
	case "stock_based_compensation":
		cf.OperatingActivities.StockBasedCompensation = v
	case "net_cash_from_operations", "net_cash_operating":
		cf.CashSummary.NetCashOperating = v

	// Investing Section
	case "capital_expenditures", "capex":
		cf.InvestingActivities.Capex = v
	case "net_cash_from_investing", "net_cash_investing":
		cf.CashSummary.NetCashInvesting = v

	// Financing Section
	case "dividends_paid":
		cf.FinancingActivities.DividendsPaid = v
	case "share_repurchases":
		cf.FinancingActivities.ShareRepurchases = v
	case "net_cash_from_financing", "net_cash_financing":
		cf.CashSummary.NetCashFinancing = v

	// Net Change
	case "net_change_in_cash":
		cf.CashSummary.NetChangeInCash = v
	}
}

// findTableLineV2 finds the line number where a table starts based on patterns
func findTableLineV2(markdown string, patterns []string) int {
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		upperLine := strings.ToUpper(line)
		for _, pattern := range patterns {
			if strings.Contains(upperLine, strings.ToUpper(pattern)) {
				return i + 1 // 1-indexed
			}
		}
	}
	return 0
}

// sliceLinesV2 extracts a range of lines from markdown
func sliceLinesV2(markdown string, start, end int) string {
	lines := strings.Split(markdown, "\n")
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		return ""
	}
	return strings.Join(lines[start-1:end-1], "\n")
}
