// Package calc provides deterministic financial calculations for the FSAP model.
// This file defines core data types for the FSAP workflow.
package calc

// =============================================================================
// FSAP DATA STRUCTURES
// Matches: V2_FSAP_Ford_2023.xlsx → Data Sheet structure
// =============================================================================

// BalanceSheet represents balance sheet line items.
// Row references are from V2_FSAP_Ford_2023.xlsx Data sheet.
type BalanceSheet struct {
	// CURRENT ASSETS (Rows 16-24)
	Cash               float64 `json:"cash"`                 // Row 16
	ShortTermInvest    float64 `json:"short_term_invest"`    // Row 17
	AccountsReceivable float64 `json:"accounts_receivable"`  // Row 18
	Inventories        float64 `json:"inventories"`          // Row 19
	PrepaidExpenses    float64 `json:"prepaid_expenses"`     // Row 20
	OtherCurrentAssets float64 `json:"other_current_assets"` // Row 21-23
	TotalCurrentAssets float64 `json:"total_current_assets"` // Row 24 (Grey: SUM)

	// NON-CURRENT ASSETS (Rows 25-34)
	PPEGross              float64 `json:"ppe_gross"`               // Row 25
	AccumDepreciation     float64 `json:"accum_depreciation"`      // Row 26 (negative)
	PPENet                float64 `json:"ppe_net"`                 // Row 27 (computed)
	Goodwill              float64 `json:"goodwill"`                // Row 28
	IntangibleAssets      float64 `json:"intangible_assets"`       // Row 29
	OtherNonCurrentAssets float64 `json:"other_noncurrent_assets"` // Row 30-33
	TotalAssets           float64 `json:"total_assets"`            // Row 34 (Grey: SUM)

	// CURRENT LIABILITIES (Rows 36-45)
	AccountsPayable         float64 `json:"accounts_payable"`          // Row 36
	ShortTermDebt           float64 `json:"short_term_debt"`           // Row 37
	CurrentPortionLTDebt    float64 `json:"current_portion_lt_debt"`   // Row 38
	AccruedLiabilities      float64 `json:"accrued_liabilities"`       // Row 39-44
	TotalCurrentLiabilities float64 `json:"total_current_liabilities"` // Row 45 (Grey: SUM)

	// NON-CURRENT LIABILITIES (Rows 46-52)
	LongTermDebt         float64 `json:"long_term_debt"`               // Row 46
	DeferredTaxLiability float64 `json:"deferred_tax_liability"`       // Row 47
	OtherNonCurrentLiab  float64 `json:"other_noncurrent_liabilities"` // Row 48-52
	TotalLiabilities     float64 `json:"total_liabilities"`            // Row 53 (Grey: SUM)

	// EQUITY (Rows 54-61)
	CommonStock       float64 `json:"common_stock"`       // Row 54
	APIC              float64 `json:"apic"`               // Row 55
	RetainedEarnings  float64 `json:"retained_earnings"`  // Row 56
	TreasuryStock     float64 `json:"treasury_stock"`     // Row 57 (negative)
	AOCI              float64 `json:"aoci"`               // Row 58
	NoncontrollingInt float64 `json:"noncontrolling_int"` // Row 59
	TotalEquity       float64 `json:"total_equity"`       // Row 60-61 (Grey: SUM)
}

// IncomeStatement represents income statement line items.
// Row references are from V2_FSAP_Ford_2023.xlsx Data sheet (Rows 65-97).
type IncomeStatement struct {
	// REVENUE (Row 66)
	Revenue float64 `json:"revenue"` // Row 66

	// COSTS & EXPENSES (negative values per FSAP convention)
	COGS               float64 `json:"cogs"`                // Row 67 <Cost of goods sold>
	GrossProfit        float64 `json:"gross_profit"`        // Row 68 (White: computed)
	SGAExpense         float64 `json:"sga_expense"`         // Row 69 <SG&A>
	AdvertisingExpense float64 `json:"advertising_expense"` // Row 70
	RDExpense          float64 `json:"rd_expense"`          // Row 71 <R&D>
	OtherOperatingExp  float64 `json:"other_operating_exp"` // Row 72-74
	OperatingIncome    float64 `json:"operating_income"`    // Row 75 (Grey: SUM)

	// INTEREST & OTHER
	InterestIncome    float64 `json:"interest_income"`     // Row 80
	InterestExpense   float64 `json:"interest_expense"`    // Row 81 (negative)
	OtherNonOperating float64 `json:"other_non_operating"` // Row 82-84
	IncomeBeforeTax   float64 `json:"income_before_tax"`   // Row 85 (Grey: SUM)

	// TAXES & NET INCOME
	IncomeTaxExpense  float64 `json:"income_tax_expense"`  // Row 90 (negative)
	NetIncome         float64 `json:"net_income"`          // Row 92
	NetIncomeComputed float64 `json:"net_income_computed"` // Row 93 (White: computed)
}

// CashFlowStatement represents cash flow statement line items.
// Row references are from V2_FSAP_Ford_2023.xlsx Data sheet (Rows 99-143).
type CashFlowStatement struct {
	// OPERATING ACTIVITIES (Rows 100-120)
	NetIncome             float64 `json:"net_income"`
	DepreciationAmort     float64 `json:"depreciation_amort"`
	DeferredTaxes         float64 `json:"deferred_taxes"`
	StockCompensation     float64 `json:"stock_compensation"`
	WorkingCapitalChanges float64 `json:"wc_changes"`
	OtherOperating        float64 `json:"other_operating"`
	CashFromOperations    float64 `json:"cash_from_operations"` // Grey: SUM

	// INVESTING ACTIVITIES (Rows 121-130)
	CapEx               float64 `json:"capex"`        // (negative)
	Acquisitions        float64 `json:"acquisitions"` // (negative)
	InvestmentPurchases float64 `json:"investment_purchases"`
	InvestmentSales     float64 `json:"investment_sales"`
	OtherInvesting      float64 `json:"other_investing"`
	CashFromInvesting   float64 `json:"cash_from_investing"` // Grey: SUM

	// FINANCING ACTIVITIES (Rows 131-140)
	DebtIssuance      float64 `json:"debt_issuance"`
	DebtRepayment     float64 `json:"debt_repayment"`    // (negative)
	DividendsPaid     float64 `json:"dividends_paid"`    // (negative)
	ShareRepurchases  float64 `json:"share_repurchases"` // (negative)
	OtherFinancing    float64 `json:"other_financing"`
	CashFromFinancing float64 `json:"cash_from_financing"` // Grey: SUM

	// NET CHANGE
	NetChangeInCash float64 `json:"net_change_in_cash"` // Row 138
	BeginningCash   float64 `json:"beginning_cash"`     // Row 139
	EndingCash      float64 `json:"ending_cash"`        // Row 140
}

// SupplementalData holds additional inputs needed for analysis.
// Row references are from V2_FSAP_Ford_2023.xlsx Data sheet (Rows 145-156).
type SupplementalData struct {
	EffectiveTaxRate  float64 `json:"effective_tax_rate"`  // Row 147
	SharesOutstanding float64 `json:"shares_outstanding"`  // Row 148
	SharePrice        float64 `json:"share_price"`         // Current market price
	DividendsPerShare float64 `json:"dividends_per_share"` // Row 150
}

// =============================================================================
// FSAP MODEL AGGREGATE
// =============================================================================

// FSAPData represents a complete FSAP dataset for one period.
type FSAPData struct {
	Ticker          string            `json:"ticker"`
	FiscalYear      int               `json:"fiscal_year"`
	BalanceSheet    BalanceSheet      `json:"balance_sheet"`
	IncomeStatement IncomeStatement   `json:"income_statement"`
	CashFlow        CashFlowStatement `json:"cash_flow"`
	Supplemental    SupplementalData  `json:"supplemental"`
}

// FSAPTimeSeries represents multi-year historical data.
type FSAPTimeSeries struct {
	Ticker string     `json:"ticker"`
	Years  []FSAPData `json:"years"` // Ordered from oldest to newest
}

// =============================================================================
// ANALYSIS RESULT TYPES
// =============================================================================

// AnalysisResult represents a single analyzed line item
type AnalysisResult struct {
	Value float64 `json:"value"`
	// Add other fields as needed for analysis context
}

// CommonSizeAnalysis holds the common-size analysis of financial statements
type CommonSizeAnalysis struct {
	IncomeStatement map[string]*AnalysisResult `json:"income_statement"`
	BalanceSheet    map[string]*AnalysisResult `json:"balance_sheet"`
	CashFlow        map[string]*AnalysisResult `json:"cash_flow"`
}
