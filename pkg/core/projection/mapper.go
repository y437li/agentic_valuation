package projection

import "agentic_valuation/pkg/core/calc"

// MapFromCommonSizeDefaults converts calc.CommonSizeDefaults to ProjectionAssumptions.
// This bridges the calc layer (historical analysis) to the projection layer (forecasting).
func MapFromCommonSizeDefaults(defaults calc.CommonSizeDefaults, revenueGrowth float64) ProjectionAssumptions {
	return ProjectionAssumptions{
		// Revenue Growth (passed in, not derived from common-size)
		RevenueGrowth: revenueGrowth,

		// Income Statement Drivers
		COGSPercent: defaults.COGSPercent,
		SGAPercent:  defaults.SGAPercent,
		RDPercent:   defaults.RDPercent,
		TaxRate:     defaults.TaxRate,

		// Cash Flow Drivers
		StockBasedCompPercent: defaults.StockBasedCompPercent,
		CapexPercent:          defaults.CapExPercent,

		// Financing Drivers
		DebtInterestRate:    defaults.DebtInterestRate,
		DividendPayoutRatio: defaults.DividendsPercent, // % of Revenue or NI depending on source

		// Balance Sheet Drivers (Working Capital)
		ReceivablesPercent:     defaults.ReceivablesPercent,
		InventoryPercent:       defaults.InventoryPercent,
		AccountsPayablePercent: defaults.APPercent,
		DeferredRevenuePercent: defaults.DeferredRevPercent,

		// Dynamic Item-Level Drivers
		NodeDrivers: defaults.CustomItems,
	}
}
