package models

import (
	"time"
)

type FinancialPeriod struct {
	ID         int       `json:"id"`
	Ticker     string    `json:"ticker"`
	FiscalYear int       `json:"fiscal_year"`
	PeriodType string    `json:"period_type"` // 'FY', 'Q1', etc.
	EndDate    time.Time `json:"end_date"`
}

type BSData struct {
	// Assets (Green)
	CashAndEquivalents        float64 `json:"cash_and_equivalents"`
	ShortTermInvestments      float64 `json:"short_term_investments"`
	AccountsReceivableNet     float64 `json:"accounts_receivable_net"`
	Inventories               float64 `json:"inventories"`
	FinanceDivLoansLeasesST   float64 `json:"finance_div_loans_leases_st"`
	FinanceDivOtherCurrAssets float64 `json:"finance_div_other_curr_assets"`
	OtherAssets1              float64 `json:"other_assets_1"`
	OtherCurrentAssets2       float64 `json:"other_current_assets_2"`

	// Assets (Grey/White)
	TotalCurrentAssets float64 `json:"total_current_assets"`
	PPENet             float64 `json:"ppe_net"`
	TotalAssets        float64 `json:"total_assets"`

	// Liabilities & Equity (Green)
	AccountsPayable    float64 `json:"accounts_payable"`
	AccruedLiabilities float64 `json:"accrued_liabilities"`
	// ... (Other fields can be added as needed following the same pattern)
}

type FSAPData struct {
	Period     FinancialPeriod `json:"period"`
	SnapshotID int             `json:"snapshot_id"`

	BalanceSheet    BSData             `json:"balance_sheet"`
	IncomeStatement map[string]float64 `json:"income_statement"`
	CashFlow        map[string]float64 `json:"cash_flow"`

	AuditTrail map[string]interface{} `json:"audit_trail"`
}
