package analysis

import (
	"agentic_valuation/pkg/core/calc"
	"time"
)

// CompanyAnalysis represents the complete financial analysis profile for a company
// derived from its synthesized Golden Record.
type CompanyAnalysis struct {
	Ticker       string                  `json:"ticker"`
	CIK          string                  `json:"cik"`
	LastAnalyzed time.Time               `json:"last_analyzed"`
	Timeline     map[int]*YearlyAnalysis `json:"timeline"` // Key: Fiscal Year
}

// YearlyAnalysis contains the computed metrics for a specific fiscal year.
type YearlyAnalysis struct {
	FiscalYear int `json:"fiscal_year"`

	// 1. Common-Size Analysis (Percentage of Revenue/Assets)
	CommonSize *calc.CommonSizeAnalysis `json:"common_size"`

	// 2. Efficiency & Return Ratios (Penman, DuPont)
	Ratios *calc.ThreeLevelAnalysis `json:"ratios"`

	// 3. Implied Metrics (Tax Rate, Useful Life)
	Implied calc.ImpliedMetrics `json:"implied"`

	// 4. Growth Rates (calculated using current vs prior year)
	Growth GrowthMetrics `json:"growth"`

	// 5. Benford's Law Analysis
	Benford *calc.BenfordResult `json:"benford"`
}

// GrowthMetrics captures Year-over-Year growth rates for key items.
type GrowthMetrics struct {
	RevenueGrowth     float64 `json:"revenue_growth"`
	OpIncomeGrowth    float64 `json:"op_income_growth"`
	NetIncomeGrowth   float64 `json:"net_income_growth"`
	TotalAssetsGrowth float64 `json:"total_assets_growth"`
	EquityGrowth      float64 `json:"equity_growth"`
	FCFGrowth         float64 `json:"fcf_growth"`
}
