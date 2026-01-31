package projection

import (
	"agentic_valuation/pkg/core/edgar"
)

// ProjectedFinancials holds the articulated statements for a projected year
type ProjectedFinancials struct {
	Year            int
	IncomeStatement *edgar.IncomeStatement
	BalanceSheet    *edgar.BalanceSheet
	CashFlow        *edgar.CashFlowStatement
	Segments        []edgar.StandardizedSegment // Granular support
}

// ProjectionAssumptions defines the drivers for a specific year
type ProjectionAssumptions struct {
	RevenueGrowth float64 // %
	COGSPercent   float64 // % of Revenue

	// SG&A Breakdown (Priority over SGAPercent)
	SellingMarketingPercent float64 // % of Revenue
	GeneralAdminPercent     float64 // % of Revenue

	// Fallback/Aggregate
	SGAPercent float64 // % of Revenue

	RDPercent float64 // % of Revenue
	TaxRate   float64 // % of EBT

	// Working Capital Drivers
	DSO float64 // Days
	DSI float64 // Days
	DPO float64 // Days

	// Capex
	CapexPercent float64 // % of Revenue

	// Depreciation Drivers
	UsefulLifeForecast  float64 // Years (Gross PPE / Depn)
	DepreciationPercent float64 // % of Gross PPE

	// Valuation Drivers (Carried through for DCF)

	TerminalGrowth float64 // %

	// Dynamic WACC Components
	UnleveredBeta     float64
	RiskFreeRate      float64
	MarketRiskPremium float64
	PreTaxCostOfDebt  float64
	TargetDebtEquity  float64 // D/E ratio

	// Dynamic Linkage (V3 Feature)
	NodeDrivers map[string]float64

	// Segment Drivers (Sum-of-Parts)
	// Key: Segment Name (normalized), Value: Growth Rate (decimal)
	SegmentGrowth map[string]float64

	// Common Size Granularity (New)
	StockBasedCompPercent float64 // % of Revenue (Add-back to CF)
	DividendPayoutRatio   float64 // % of Net Income
	CashInterestRate      float64 // % on Cash Balance
	DebtInterestRate      float64 // % on Debt Balance

	// Working Capital (Percentage Method)
	ReceivablesPercent     float64 // % of Revenue
	InventoryPercent       float64 // % of Revenue
	AccountsPayablePercent float64 // % of Revenue
	DeferredRevenuePercent float64 // % of Revenue

	// Capital Structure
	SharesOutstanding float64 // Millions
}
