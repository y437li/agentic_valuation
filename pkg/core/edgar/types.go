// Package edgar provides functionality for fetching and parsing SEC EDGAR filings.
package edgar

import "time"

// XBRLFact represents a single XBRL tagged value from inline XBRL
type XBRLFact struct {
	Tag        string  `json:"tag"`         // e.g., "us-gaap:Assets"
	Value      string  `json:"value"`       // Raw string value
	NumericVal float64 `json:"numeric_val"` // Parsed numeric value
	ContextRef string  `json:"context_ref"` // XBRL context reference
	Decimals   string  `json:"decimals"`    // Decimals attribute
	UnitRef    string  `json:"unit_ref"`    // Unit reference (e.g., "usd")
}

// FilingMetadata contains metadata about a SEC filing
type FilingMetadata struct {
	CIK             string    `json:"cik"`
	CompanyName     string    `json:"company_name"`
	Tickers         []string  `json:"tickers"`
	AccessionNumber string    `json:"accession_number"`
	FilingDate      string    `json:"filing_date"`
	Form            string    `json:"form"`       // "10-K", "10-Q", "8-K"
	IsAmended       bool      `json:"is_amended"` // True if this is a 10-K/A amendment
	FiscalYear      int       `json:"fiscal_year"`
	FiscalPeriod    string    `json:"fiscal_period"` // "FY", "Q1", "Q2", "Q3"
	PrimaryDocument string    `json:"primary_document"`
	FilingURL       string    `json:"filing_url"`
	ParsedAt        time.Time `json:"parsed_at"`
}

// =============================================================================
// V2.0 ARCHITECTURE: Data Source Types & Citation Support
// =============================================================================

// DataSourceType defines the origin and priority of a data point
// Priority order: MANUAL > LOCAL_FILE > INTERNAL_DB > WEB_SEARCH
type DataSourceType string

const (
	SourceManual     DataSourceType = "MANUAL"      // User override (highest priority)
	SourceLocal      DataSourceType = "LOCAL_FILE"  // Uploaded research/expert call
	SourceInternalDB DataSourceType = "INTERNAL_DB" // SEC 10-K/Q filings
	SourceWeb        DataSourceType = "WEB_SEARCH"  // Web scraping (lowest priority)
)

// Citation represents a single source reference with jump-to-source capability
// Used to support multi-source aggregation and audit trails
type Citation struct {
	AssetID string `json:"asset_id,omitempty"` // References KnowledgeAsset.ID
	ChunkID string `json:"chunk_id,omitempty"` // References specific chunk within asset
	Snippet string `json:"snippet,omitempty"`  // Quoted text from source (for display)
	Link    string `json:"link,omitempty"`     // URL or local file path (for navigation)
	PageRef string `json:"page_ref,omitempty"` // e.g., "F-22" or "p.45"
	LineNum int    `json:"line_num,omitempty"` // Line number in source document
}

// =============================================================================
// END V2.0 ARCHITECTURE EXTENSIONS
// =============================================================================

// FSAPValue represents a single FSAP variable value with source evidence
// Label = Original 10-K line item name (e.g., "Vendor non-trade receivables")
// Years = All fiscal year values in millions USD
// Value = Primary/latest year value (for backward compatibility)
type FSAPValue struct {
	// --- Core value fields (unchanged for backward compatibility) ---
	Value        *float64           `json:"value"`                   // Primary year value in millions USD
	Years        map[string]float64 `json:"years,omitempty"`         // Multi-year values: {"2024": 25000, "2023": 23000}
	Label        string             `json:"label,omitempty"`         // Original 10-K item name (source of truth)
	XBRLTag      string             `json:"xbrl_tag,omitempty"`      // Mapped XBRL tag (if available)
	SourcePath   string             `json:"source_path,omitempty"`   // Path in source doc (omitempty)
	MappingType  string             `json:"mapping_type,omitempty"`  // DIRECT, FALLBACK, RECLASSIFIED, UNMAPPED
	Confidence   float64            `json:"confidence,omitempty"`    // LLM confidence (0-1)
	Provenance   *SourceTrace       `json:"provenance,omitempty"`    // Full extraction provenance
	FSAPVariable string             `json:"fsap_variable,omitempty"` // Mapped FSAP variable key (e.g. "cash_and_equivalents")

	// --- V2.0 Architecture: Multi-source support ---
	SourceType DataSourceType `json:"source_type,omitempty"` // Origin priority indicator
	Citations  []Citation     `json:"citations,omitempty"`   // Multi-source references for audit trail
}

// SourceTrace provides complete provenance for a extracted value
type SourceTrace struct {
	SectionTitle     string `json:"section_title"`
	ParentSection    string `json:"parent_section,omitempty"` // V2: Section classification (e.g. "current_assets_section")
	TableID          string `json:"table_id,omitempty"`
	PageRef          string `json:"page_ref,omitempty"`
	RowIndex         int    `json:"row_index,omitempty"`
	RowLabel         string `json:"row_label"`
	ColumnIndex      int    `json:"column_index,omitempty"`
	ColumnLabel      string `json:"column_label,omitempty"`
	Scale            string `json:"scale,omitempty"` // e.g. "millions", "thousands"
	RawValue         string `json:"raw_value,omitempty"`
	Currency         string `json:"currency,omitempty"`
	MarkdownPosition int    `json:"markdown_position,omitempty"`
	MarkdownLine     int    `json:"markdown_line,omitempty"`
	ExtractedBy      string `json:"extracted_by"`
	ExtractedAt      string `json:"extracted_at,omitempty"`
}

// ReportedForValidation contains SEC-reported totals for verification
type ReportedForValidation struct {
	TotalCurrentAssets      *FSAPValue `json:"total_current_assets,omitempty"`
	TotalAssets             *FSAPValue `json:"total_assets,omitempty"`
	TotalCurrentLiabilities *FSAPValue `json:"total_current_liabilities,omitempty"`
	TotalLiabilities        *FSAPValue `json:"total_liabilities,omitempty"`
	TotalEquity             *FSAPValue `json:"total_equity,omitempty"`
	GrossProfit             *FSAPValue `json:"gross_profit,omitempty"`
	OperatingIncome         *FSAPValue `json:"operating_income,omitempty"`
	IncomeBeforeTax         *FSAPValue `json:"income_before_tax,omitempty"`
	IncomeTaxExpense        *FSAPValue `json:"income_tax_expense,omitempty"`
	NetIncome               *FSAPValue `json:"net_income,omitempty"`
	NetCashOperating        *FSAPValue `json:"net_cash_operating,omitempty"`
	NetCashInvesting        *FSAPValue `json:"net_cash_investing,omitempty"`
	NetCashFinancing        *FSAPValue `json:"net_cash_financing,omitempty"`
	NetChangeInCash         *FSAPValue `json:"net_change_in_cash,omitempty"`
}

// CurrentAssets contains current asset line items
type CurrentAssets struct {
	CashAndEquivalents       *FSAPValue  `json:"cash_and_equivalents"`
	ShortTermInvestments     *FSAPValue  `json:"short_term_investments"`
	AccountsReceivableNet    *FSAPValue  `json:"accounts_receivable_net"`
	Inventories              *FSAPValue  `json:"inventories"`
	FinanceDivLoansST        *FSAPValue  `json:"finance_div_loans_leases_st,omitempty"`
	FinanceDivOtherCurrAsset *FSAPValue  `json:"finance_div_other_curr_assets,omitempty"`
	OtherAssets              *FSAPValue  `json:"other_assets,omitempty"`
	OtherCurrentAssets       *FSAPValue  `json:"other_current_assets,omitempty"`
	AdditionalItems          []FSAPValue `json:"additional_items,omitempty"`
	CalculatedTotal          *float64    `json:"calculated_total,omitempty"`
}

// NoncurrentAssets contains non-current asset line items
type NoncurrentAssets struct {
	LongTermInvestments     *FSAPValue  `json:"long_term_investments"`
	DeferredChargesLT       *FSAPValue  `json:"deferred_charges_lt,omitempty"`
	PPEAtCost               *FSAPValue  `json:"ppe_at_cost"`
	AccumulatedDepreciation *FSAPValue  `json:"accumulated_depreciation"`
	PPENet                  *FSAPValue  `json:"ppe_net,omitempty"`
	Intangibles             *FSAPValue  `json:"intangibles"`
	Goodwill                *FSAPValue  `json:"goodwill"`
	FinanceDivLoansLT       *FSAPValue  `json:"finance_div_loans_leases_lt,omitempty"`
	FinanceDivOtherLTAssets *FSAPValue  `json:"finance_div_other_lt_assets,omitempty"`
	DeferredTaxAssetsLT     *FSAPValue  `json:"deferred_tax_assets_lt"`
	RestrictedCash          *FSAPValue  `json:"restricted_cash,omitempty"`
	OtherNoncurrentAssets   *FSAPValue  `json:"other_noncurrent_assets,omitempty"`
	AdditionalItems         []FSAPValue `json:"additional_items,omitempty"`
	CalculatedTotal         *float64    `json:"calculated_total,omitempty"`
}

// CurrentLiabilities contains current liability line items
type CurrentLiabilities struct {
	AccountsPayable           *FSAPValue  `json:"accounts_payable"`
	AccruedLiabilities        *FSAPValue  `json:"accrued_liabilities"`
	NotesPayableShortTermDebt *FSAPValue  `json:"notes_payable_short_term_debt"`
	CurrentMaturitiesLTD      *FSAPValue  `json:"current_maturities_long_term_debt"`
	CurrentOperatingLeaseLiab *FSAPValue  `json:"current_operating_lease_liabilities,omitempty"`
	DeferredRevenueCurrent    *FSAPValue  `json:"deferred_revenue_current,omitempty"`
	FinanceDivCurr            *FSAPValue  `json:"finance_div_curr,omitempty"`
	OtherCurrentLiabilities   *FSAPValue  `json:"other_current_liabilities,omitempty"`
	AdditionalItems           []FSAPValue `json:"additional_items,omitempty"`
	CalculatedTotal           *float64    `json:"calculated_total,omitempty"`
}

// NoncurrentLiabilities contains non-current liability line items
type NoncurrentLiabilities struct {
	LongTermDebt               *FSAPValue  `json:"long_term_debt"`
	LongTermOperatingLeaseLiab *FSAPValue  `json:"long_term_operating_lease_liabilities,omitempty"`
	DeferredTaxLiabilities     *FSAPValue  `json:"deferred_tax_liabilities"`
	PensionObligations         *FSAPValue  `json:"pension_obligations,omitempty"`
	FinanceDivNoncurr          *FSAPValue  `json:"finance_div_noncurr,omitempty"`
	OtherNoncurrentLiabilities *FSAPValue  `json:"other_noncurrent_liabilities,omitempty"`
	AdditionalItems            []FSAPValue `json:"additional_items,omitempty"`
	CalculatedTotal            *float64    `json:"calculated_total,omitempty"`
}

// Equity contains stockholders' equity line items
type Equity struct {
	PreferredStock                *FSAPValue  `json:"preferred_stock,omitempty"`
	CommonStockAPIC               *FSAPValue  `json:"common_stock_apic"`
	RetainedEarningsDeficit       *FSAPValue  `json:"retained_earnings_deficit"`
	TreasuryStock                 *FSAPValue  `json:"treasury_stock"`
	AccumOtherComprehensiveIncome *FSAPValue  `json:"accum_other_comprehensive_income"`
	NoncontrollingInterests       *FSAPValue  `json:"noncontrolling_interests"`
	AdditionalItems               []FSAPValue `json:"additional_items,omitempty"`
	CalculatedTotal               *float64    `json:"calculated_total,omitempty"`
}

// BalanceSheet contains the full balance sheet structure
type BalanceSheet struct {
	CurrentAssets         CurrentAssets         `json:"current_assets"`
	NoncurrentAssets      NoncurrentAssets      `json:"noncurrent_assets"`
	CurrentLiabilities    CurrentLiabilities    `json:"current_liabilities"`
	NoncurrentLiabilities NoncurrentLiabilities `json:"noncurrent_liabilities"`
	Equity                Equity                `json:"equity"`
	ReportedForValidation ReportedForValidation `json:"_reported_for_validation"`
}

// IncomeStatement contains income statement line items
type IncomeStatement struct {
	// FSAP 6-Section Structure
	GrossProfitSection   *GrossProfitSection    `json:"gross_profit_section,omitempty"`
	OperatingCostSection *OperatingCostSection  `json:"operating_cost_section,omitempty"`
	NonOperatingSection  *NonOperatingSection   `json:"non_operating_section,omitempty"`
	TaxAdjustments       *TaxAdjustmentsSection `json:"tax_adjustments_section,omitempty"`
	NetIncomeSection     *NetIncomeSection      `json:"net_income_section,omitempty"`
	OCISection           *OCISection            `json:"oci_section,omitempty"`

	// Special section for Supplemental Data analysis (NOT in IS flow-through)
	NonRecurringSection *NonRecurringSection `json:"nonrecurring_section,omitempty"`

	AdditionalItems []AdditionalItem `json:"additional_items,omitempty"`
}

// GrossProfitSection represents Section 1 of Income Statement
type GrossProfitSection struct {
	Revenues        *FSAPValue       `json:"revenues,omitempty"`
	CostOfGoodsSold *FSAPValue       `json:"cost_of_goods_sold,omitempty"`
	GrossProfit     *FSAPValue       `json:"gross_profit,omitempty"`
	AdditionalItems []AdditionalItem `json:"additional_items,omitempty"`
}

// OperatingCostSection represents Section 2 of Income Statement
type OperatingCostSection struct {
	SGAExpenses            *FSAPValue       `json:"sga_expenses,omitempty"`
	SellingMarketing       *FSAPValue       `json:"selling_marketing,omitempty"`
	GeneralAdmin           *FSAPValue       `json:"general_admin,omitempty"`
	RDExpenses             *FSAPValue       `json:"rd_expenses,omitempty"`
	AdvertisingExpenses    *FSAPValue       `json:"advertising_expenses,omitempty"`
	OtherOperatingExpenses *FSAPValue       `json:"other_operating_expenses,omitempty"`
	OperatingIncome        *FSAPValue       `json:"operating_income,omitempty"`
	AdditionalItems        []AdditionalItem `json:"additional_items,omitempty"`
}

// NonOperatingSection represents Section 3 of Income Statement
type NonOperatingSection struct {
	InterestExpense              *FSAPValue       `json:"interest_expense,omitempty"`
	OtherIncomeExpense           *FSAPValue       `json:"other_income_expense,omitempty"`
	EquityAffiliatesNonOperating *FSAPValue       `json:"equity_affiliates_non_operating,omitempty"`
	IncomeBeforeTax              *FSAPValue       `json:"income_before_tax,omitempty"`
	AdditionalItems              []AdditionalItem `json:"additional_items,omitempty"`
}

// TaxAdjustmentsSection represents Section 4 of Income Statement
type TaxAdjustmentsSection struct {
	IncomeTaxExpense       *FSAPValue       `json:"income_tax_expense,omitempty"`
	DiscontinuedOperations *FSAPValue       `json:"discontinued_operations,omitempty"`
	ExtraordinaryItems     *FSAPValue       `json:"extraordinary_items,omitempty"`
	AdditionalItems        []AdditionalItem `json:"additional_items,omitempty"`
}

// NetIncomeSection represents Section 5 of Income Statement
type NetIncomeSection struct {
	NetIncomeToCommon     *FSAPValue       `json:"net_income_to_common,omitempty"`
	NetIncomeToNCI        *FSAPValue       `json:"net_income_to_nci,omitempty"`
	EPSBasic              *FSAPValue       `json:"eps_basic,omitempty"`
	EPSDiluted            *FSAPValue       `json:"eps_diluted,omitempty"`
	WeightedAverageShares *FSAPValue       `json:"weighted_average_shares,omitempty"`
	AdditionalItems       []AdditionalItem `json:"additional_items,omitempty"`
}

// OCISection represents Section 6 of Income Statement
type OCISection struct {
	OCIForeignCurrency       *FSAPValue       `json:"oci_foreign_currency,omitempty"`
	OCISecurities            *FSAPValue       `json:"oci_securities,omitempty"`
	OCIPension               *FSAPValue       `json:"oci_pension,omitempty"`
	OCIHedges                *FSAPValue       `json:"oci_hedges,omitempty"`
	OtherComprehensiveIncome *FSAPValue       `json:"other_comprehensive_income,omitempty"`
	AdditionalItems          []AdditionalItem `json:"additional_items,omitempty"`
}

// NonRecurringSection represents special items for Supplemental Data analysis
type NonRecurringSection struct {
	ImpairmentCharges    *FSAPValue       `json:"impairment_charges,omitempty"`
	RestructuringCharges *FSAPValue       `json:"restructuring_charges,omitempty"`
	GainLossAssetSales   *FSAPValue       `json:"gain_loss_asset_sales,omitempty"`
	SettlementCosts      *FSAPValue       `json:"settlement_costs,omitempty"`
	WriteOffs            *FSAPValue       `json:"write_offs,omitempty"`
	OtherNonRecurring    *FSAPValue       `json:"other_nonrecurring,omitempty"`
	AdditionalItems      []AdditionalItem `json:"additional_items,omitempty"`
}

// CashFlowStatement contains cash flow line items
type CashFlowStatement struct {
	OperatingActivities *CFOperatingSection `json:"operating_activities,omitempty"`
	InvestingActivities *CFInvestingSection `json:"investing_activities,omitempty"`
	FinancingActivities *CFFinancingSection `json:"financing_activities,omitempty"`
	SupplementalInfo    *CFSupplementalInfo `json:"supplemental_info,omitempty"`
	CashSummary         *CashSummarySection `json:"cash_summary,omitempty"`

	// Legacy flat fields
	NetIncomeStart             *FSAPValue            `json:"net_income_start,omitempty"`
	DepreciationAmortization   *FSAPValue            `json:"depreciation_amortization,omitempty"`
	DeferredTaxes              *FSAPValue            `json:"deferred_taxes,omitempty"`
	StockBasedCompensation     *FSAPValue            `json:"stock_based_compensation,omitempty"`
	ChangesInWorkingCapital    *FSAPValue            `json:"changes_in_working_capital,omitempty"`
	OtherOperatingItems        *FSAPValue            `json:"other_operating_items,omitempty"`
	Capex                      *FSAPValue            `json:"capex,omitempty"`
	InvestmentsAcquiredSoldNet *FSAPValue            `json:"investments_acquired_sold_net,omitempty"`
	OtherInvestingItems        *FSAPValue            `json:"other_investing_items,omitempty"`
	DebtIssuanceRetirementNet  *FSAPValue            `json:"debt_issuance_retirement_net,omitempty"`
	ShareRepurchases           *FSAPValue            `json:"share_repurchases,omitempty"`
	Dividends                  *FSAPValue            `json:"dividends,omitempty"`
	OtherFinancingItems        *FSAPValue            `json:"other_financing_items,omitempty"`
	EffectExchangeRate         *FSAPValue            `json:"effect_exchange_rate,omitempty"`
	ReportedForValidation      ReportedForValidation `json:"_reported_for_validation"`
}

// CFOperatingSection represents Operating Activities section
type CFOperatingSection struct {
	NetIncomeStart           *FSAPValue       `json:"net_income_start,omitempty"`
	DepreciationAmortization *FSAPValue       `json:"depreciation_amortization,omitempty"`
	AmortizationIntangibles  *FSAPValue       `json:"amortization_intangibles,omitempty"`
	DeferredTaxes            *FSAPValue       `json:"deferred_taxes,omitempty"`
	StockBasedCompensation   *FSAPValue       `json:"stock_based_compensation,omitempty"`
	ImpairmentCharges        *FSAPValue       `json:"impairment_charges,omitempty"`
	GainLossAssetSales       *FSAPValue       `json:"gain_loss_asset_sales,omitempty"`
	ChangeReceivables        *FSAPValue       `json:"change_receivables,omitempty"`
	ChangeInventory          *FSAPValue       `json:"change_inventory,omitempty"`
	ChangePayables           *FSAPValue       `json:"change_payables,omitempty"`
	ChangeAccruedExpenses    *FSAPValue       `json:"change_accrued_expenses,omitempty"`
	ChangeDeferredRevenue    *FSAPValue       `json:"change_deferred_revenue,omitempty"`
	OtherWorkingCapital      *FSAPValue       `json:"other_working_capital,omitempty"`
	OtherNonCashItems        *FSAPValue       `json:"other_non_cash_items,omitempty"`
	AdditionalItems          []AdditionalItem `json:"additional_items,omitempty"`
}

// CFInvestingSection represents Investing Activities section
type CFInvestingSection struct {
	Capex                *FSAPValue       `json:"capex,omitempty"`
	AcquisitionsNet      *FSAPValue       `json:"acquisitions_net,omitempty"`
	PurchasesSecurities  *FSAPValue       `json:"purchases_securities,omitempty"`
	MaturitiesSecurities *FSAPValue       `json:"maturities_securities,omitempty"`
	SalesSecurities      *FSAPValue       `json:"sales_securities,omitempty"`
	ProceedsAssetSales   *FSAPValue       `json:"proceeds_asset_sales,omitempty"`
	OtherInvesting       *FSAPValue       `json:"other_investing,omitempty"`
	AdditionalItems      []AdditionalItem `json:"additional_items,omitempty"`
}

// CFFinancingSection represents Financing Activities section
type CFFinancingSection struct {
	DebtProceeds           *FSAPValue       `json:"debt_proceeds,omitempty"`
	DebtRepayments         *FSAPValue       `json:"debt_repayments,omitempty"`
	StockIssuanceProceeds  *FSAPValue       `json:"stock_issuance_proceeds,omitempty"`
	ShareRepurchases       *FSAPValue       `json:"share_repurchases,omitempty"`
	DividendsPaid          *FSAPValue       `json:"dividends_paid,omitempty"`
	TaxWithholdingPayments *FSAPValue       `json:"tax_withholding_payments,omitempty"`
	OtherFinancing         *FSAPValue       `json:"other_financing,omitempty"`
	AdditionalItems        []AdditionalItem `json:"additional_items,omitempty"`
}

// CFSupplementalInfo represents Supplemental Cash Flow Information
type CFSupplementalInfo struct {
	CashInterestPaid *FSAPValue `json:"cash_interest_paid,omitempty"`
	CashTaxesPaid    *FSAPValue `json:"cash_taxes_paid,omitempty"`
	NonCashInvesting *FSAPValue `json:"non_cash_investing,omitempty"`
	NonCashFinancing *FSAPValue `json:"non_cash_financing,omitempty"`
}

// CashSummarySection represents Cash Summary/Reconciliation
type CashSummarySection struct {
	NetCashOperating *FSAPValue `json:"net_cash_operating,omitempty"`
	NetCashInvesting *FSAPValue `json:"net_cash_investing,omitempty"`
	NetCashFinancing *FSAPValue `json:"net_cash_financing,omitempty"`
	FXEffect         *FSAPValue `json:"fx_effect,omitempty"`
	NetChangeInCash  *FSAPValue `json:"net_change_in_cash,omitempty"`
	CashBeginning    *FSAPValue `json:"cash_beginning,omitempty"`
	CashEnding       *FSAPValue `json:"cash_ending,omitempty"`
}

// AdditionalItem represents a catch-all item not in standard mapping
// LLM may output either {label, years} or {label, value: {years}} format
type AdditionalItem struct {
	Key      string             `json:"key"`
	Label    string             `json:"label"`
	Category string             `json:"category,omitempty"`
	Value    *FSAPValue         `json:"value,omitempty"`
	Years    map[string]float64 `json:"years,omitempty"` // Direct years map from LLM output
}

// SupplementalData contains key financial ratios and metrics
type SupplementalData struct {
	StatutoryTaxRate         *FSAPValue `json:"statutory_tax_rate,omitempty"`
	EPSBasic                 *FSAPValue `json:"eps_basic"`
	EPSDiluted               *FSAPValue `json:"eps_diluted"`
	SharesOutstandingBasic   *FSAPValue `json:"shares_outstanding_basic"`
	SharesOutstandingDiluted *FSAPValue `json:"shares_outstanding_diluted"`
	PreferredDividends       *FSAPValue `json:"preferred_dividends,omitempty"`
	DepreciationExpense      *FSAPValue `json:"depreciation_expense,omitempty"`
	EffectiveTaxRate         *float64   `json:"effective_tax_rate,omitempty"`
	AfterTaxNonRecurring     *float64   `json:"after_tax_nonrecurring,omitempty"`
	CommonDividendsPerShare  *float64   `json:"common_dividends_per_share,omitempty"`
	SharePriceYearEnd        *float64   `json:"share_price_year_end,omitempty"`
}

// NoteEvidence provides evidence from Notes for reclassifications
type NoteEvidence struct {
	NoteNumber      string             `json:"note_number"`
	NoteTitle       string             `json:"note_title"`
	SourceDocument  string             `json:"source_document"`
	PageReference   string             `json:"page_reference,omitempty"`
	Quote           string             `json:"quote"`
	ExtractedDetail map[string]float64 `json:"extracted_detail,omitempty"`
}

// Reclassification represents a reclassified line item
type Reclassification struct {
	FSAPVariable         string        `json:"fsap_variable"`
	ReclassificationType string        `json:"reclassification_type"`
	SourceTags           []SourceTag   `json:"source_tags"`
	Value                float64       `json:"value"`
	NoteEvidence         *NoteEvidence `json:"note_evidence,omitempty"`
	Reasoning            string        `json:"reasoning"`
}

// SourceTag represents a source XBRL tag for reclassification
type SourceTag struct {
	Tag   string  `json:"tag"`
	Value float64 `json:"value"`
}

// Metadata contains processing metadata
type Metadata struct {
	LLMProvider       string `json:"llm_provider,omitempty"`
	VariablesMapped   int    `json:"variables_mapped"`
	VariablesUnmapped int    `json:"variables_unmapped"`
	ProcessingTimeMs  int64  `json:"processing_time_ms"`
}

// DebugStep represents a single step in the extraction pipeline
type DebugStep struct {
	Name     string      `json:"name"`
	Status   string      `json:"status"`
	TimingMs int64       `json:"timing_ms"`
	Detail   string      `json:"detail"`
	Data     interface{} `json:"data,omitempty"`
}

// DebugSteps contains all pipeline step results for debugging
type DebugSteps struct {
	Markdown   *DebugStep `json:"markdown,omitempty"`
	Tables     *DebugStep `json:"tables,omitempty"`
	Mapping    *DebugStep `json:"mapping,omitempty"`
	Validation *DebugStep `json:"validation,omitempty"`
	Final      *DebugStep `json:"final,omitempty"`
}

// FSAPDataResponse is the full response structure
type FSAPDataResponse struct {
	Company           string                 `json:"company"`
	CIK               string                 `json:"cik"`
	FiscalYear        int                    `json:"fiscal_year"`
	FiscalYears       []int                  `json:"fiscal_years"`
	FiscalPeriod      string                 `json:"fiscal_period"`
	IsAmended         bool                   `json:"is_amended"`
	SourceDocument    string                 `json:"source_document"`
	FilingURL         string                 `json:"filing_url,omitempty"`
	FullMarkdown      string                 `json:"full_markdown,omitempty"`
	BalanceSheet      BalanceSheet           `json:"balance_sheet"`
	IncomeStatement   IncomeStatement        `json:"income_statement"`
	CashFlowStatement CashFlowStatement      `json:"cash_flow_statement"`
	SupplementalData  SupplementalData       `json:"supplemental_data"`
	HistoricalData    map[int]YearData       `json:"historical_data,omitempty"`
	Qualitative       *QualitativeInsights   `json:"qualitative,omitempty"`
	Reclassifications []Reclassification     `json:"reclassifications,omitempty"`
	Metadata          Metadata               `json:"metadata"`
	DebugSteps        *DebugSteps            `json:"debug_steps,omitempty"`
	RawJSON           map[string]interface{} `json:"raw_json,omitempty"`
}

// YearData contains financial data for a single fiscal year
type YearData struct {
	BalanceSheet      BalanceSheet      `json:"balance_sheet"`
	IncomeStatement   IncomeStatement   `json:"income_statement"`
	CashFlowStatement CashFlowStatement `json:"cash_flow_statement"`
	SupplementalData  SupplementalData  `json:"supplemental_data"`
}
