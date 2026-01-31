// Package calc provides deterministic financial calculations for the FSAP model.
// This file implements valuation methods: WACC, CAPM, DCF models.
package calc

import (
	"math"
)

// =============================================================================
// VALUATION PARAMETERS
// These should be stored in database (Supabase) and loaded at runtime.
// =============================================================================

// ValuationParams holds core inputs for valuation calculations.
// All values should be retrieved from database.
type ValuationParams struct {
	RiskFreeRate      float64 // r_f: e.g., 0.04318 (4.318%)
	MarketBeta        float64 // β: e.g., 1.64
	MarketRiskPremium float64 // MRP: e.g., 0.0418 (4.18%)
	LongRunGrowth     float64 // g: e.g., 0.0445 (4.45%)
	TaxRate           float64 // T: Corporate tax rate
	SharesOutstanding float64 // For per-share calculations
}

// =============================================================================
// COST OF CAPITAL
// Based on: V2_FSAP_Ford_2023.xlsx → Valuation Sheet
// =============================================================================

// CostOfEquityCAPM calculates required return on equity using CAPM.
//
// FORMULA: r_e = r_f + β × MRP
//
// Excel Reference: Valuation!B37
// =B34 + B33*B35
//
// Where:
//   - r_f = Risk-free rate (10-year Treasury)
//   - β = Equity beta (market sensitivity)
//   - MRP = Market Risk Premium (expected market return - risk-free rate)
func CostOfEquityCAPM(riskFreeRate, beta, marketRiskPremium float64) float64 {
	return riskFreeRate + beta*marketRiskPremium
}

// WACC calculates Weighted Average Cost of Capital.
//
// FORMULA: WACC = r_d × (1 - T) × (D/V) + r_e × (E/V)
//
// Excel Reference: Valuation!B61
//
// Where:
//   - r_d = Cost of debt (yield on debt)
//   - T = Corporate tax rate
//   - D/V = Debt weight in capital structure
//   - r_e = Cost of equity (from CAPM)
//   - E/V = Equity weight in capital structure
func WACC(costOfDebt, taxRate, debtWeight, costOfEquity, equityWeight float64) float64 {
	afterTaxDebtCost := costOfDebt * (1 - taxRate) * debtWeight
	equityCost := costOfEquity * equityWeight
	return afterTaxDebtCost + equityCost
}

// =============================================================================
// DCF VALUATION MODELS
// =============================================================================

// TerminalValueGordonGrowth calculates terminal value using Gordon Growth Model.
//
// FORMULA: TV = CF_{t+1} / (r - g)
//
// Excel Reference: Valuation!G81/(B36-B29) for dividends
//
// Where:
//   - CF_{t+1} = Next period's cash flow (after forecast horizon)
//   - r = Discount rate (WACC or cost of equity)
//   - g = Long-run growth rate (must be < r)
func TerminalValueGordonGrowth(nextPeriodCF, discountRate, growthRate float64) float64 {
	if discountRate <= growthRate {
		return 0 // Invalid: growth must be less than discount rate
	}
	return nextPeriodCF / (discountRate - growthRate)
}

// PresentValue calculates PV of a single cash flow.
//
// FORMULA: PV = CF / (1 + r)^t
func PresentValue(cashFlow, discountRate float64, periods int) float64 {
	if periods < 0 {
		return 0
	}
	return cashFlow / math.Pow(1+discountRate, float64(periods))
}

// PresentValueOfCashFlows calculates PV of a series of cash flows.
//
// FORMULA: PV = Σ [ CF_t / (1 + r)^t ]
//
// Cash flows are assumed to be at end of each period (ordinary annuity).
func PresentValueOfCashFlows(cashFlows []float64, discountRate float64) float64 {
	var pv float64
	for t, cf := range cashFlows {
		pv += cf / math.Pow(1+discountRate, float64(t+1))
	}
	return pv
}

// =============================================================================
// VALUATION METHODS
// Based on: V2_FSAP_Ford_2023.xlsx → Valuation Sheet methods
// =============================================================================

// ValuationResult contains per-share values from different methods.
type ValuationResult struct {
	DividendModel  float64 // DDM-based value
	FCFEModel      float64 // Free Cash Flow to Equity
	ResidualIncome float64 // Residual Income Valuation
	FCFFModel      float64 // Free Cash Flow to Firm (Enterprise)
}

// DividendModelValue calculates equity value using Dividend Discount Model.
//
// FORMULA: Value = PV(Dividends) + PV(Terminal Value)
//
// Excel Reference: Valuation!B89 / B56 (per share)
func DividendModelValue(dividends []float64, terminalValue, costOfEquity float64, sharesOutstanding float64) float64 {
	pvDividends := PresentValueOfCashFlows(dividends, costOfEquity)
	pvTerminal := PresentValue(terminalValue, costOfEquity, len(dividends))
	equityValue := pvDividends + pvTerminal

	if sharesOutstanding == 0 {
		return 0
	}
	return equityValue / sharesOutstanding
}

// FCFEModelValue calculates equity value using Free Cash Flow to Equity.
//
// FORMULA: Value = PV(FCFE) + PV(Terminal Value)
//
// Excel Reference: Valuation!B119 / B56 (per share)
//
// FCFE = Net Income - Net CapEx - Change in Working Capital + Net Borrowing
func FCFEModelValue(fcfeFlows []float64, terminalValue, costOfEquity float64, sharesOutstanding float64) float64 {
	pvFCFE := PresentValueOfCashFlows(fcfeFlows, costOfEquity)
	pvTerminal := PresentValue(terminalValue, costOfEquity, len(fcfeFlows))
	equityValue := pvFCFE + pvTerminal

	if sharesOutstanding == 0 {
		return 0
	}
	return equityValue / sharesOutstanding
}

// ResidualIncomeValue calculates equity value using Residual Income Model.
//
// FORMULA: Value = Book Value + PV(Residual Income)
//
// Excel Reference: Valuation!B176 / B56 (per share)
//
// Residual Income = Net Income - (Cost of Equity × Beginning Book Value)
func ResidualIncomeValue(bookValue float64, residualIncomes []float64, costOfEquity float64, sharesOutstanding float64) float64 {
	pvRI := PresentValueOfCashFlows(residualIncomes, costOfEquity)
	equityValue := bookValue + pvRI

	if sharesOutstanding == 0 {
		return 0
	}
	return equityValue / sharesOutstanding
}

// FCFFModelValue calculates equity value using Free Cash Flow to Firm (Enterprise).
//
// FORMULA: Equity Value = Enterprise Value - Net Debt
//
//	Enterprise Value = PV(FCFF) + PV(Terminal Value)
//
// Excel Reference: Valuation!B274 / B56 (per share)
//
// FCFF = EBIT × (1 - T) + Depreciation - CapEx - Change in WC
func FCFFModelValue(fcffFlows []float64, terminalValue, wacc, netDebt float64, sharesOutstanding float64) float64 {
	pvFCFF := PresentValueOfCashFlows(fcffFlows, wacc)
	pvTerminal := PresentValue(terminalValue, wacc, len(fcffFlows))
	enterpriseValue := pvFCFF + pvTerminal
	equityValue := enterpriseValue - netDebt

	if sharesOutstanding == 0 {
		return 0
	}
	return equityValue / sharesOutstanding
}

// =============================================================================
// FORECAST PROJECTIONS
// Based on: V2_FSAP_Ford_2023.xlsx → Forecast Development Sheet
// =============================================================================

// ProjectRevenue calculates projected revenue based on growth assumption.
//
// FORMULA: Sales_t = Sales_{t-1} × (1 + Growth_t)
//
// Excel Reference: Forecast Development rows 34, 45, 56, etc.
func ProjectRevenue(priorRevenue, growthRate float64) float64 {
	return priorRevenue * (1 + growthRate)
}

// ProjectFromRatio calculates projected amount from revenue ratio.
//
// FORMULA: Amount = Revenue × Ratio
//
// Used for: COGS, SG&A, etc. (where ratio is the "common size" percentage)
// Example: COGS = Revenue × (-0.90) for 90% COGS ratio
func ProjectFromRatio(revenue, ratio float64) float64 {
	return revenue * ratio
}

// ProjectFromGrowth calculates projected amount from prior period growth.
//
// FORMULA: Amount_t = Amount_{t-1} × (1 + Growth_t)
//
// Used for: Working capital, PP&E, etc.
func ProjectFromGrowth(priorAmount, growthRate float64) float64 {
	return priorAmount * (1 + growthRate)
}
