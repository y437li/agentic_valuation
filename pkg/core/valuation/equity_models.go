package valuation

import (
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/projection"
	"math"
)

// EquityModelInput holds inputs shared across equity-based valuation models (DDM, RIM, FCFE)
type EquityModelInput struct {
	Projections       []*projection.ProjectedFinancials
	CostOfEquity      float64 // Ke
	TerminalGrowth    float64 // g
	SharesOutstanding float64
	CurrentBookValue  float64 // B_0 (Initial Book Value of Equity)
}

// EquityValuationResult holds the valuation outputs for a specific model
type EquityValuationResult struct {
	ModelName     string
	EquityValue   float64
	SharePrice    float64
	PV_Stream     float64 // PV of Dividends / RI / FCFE
	PV_Terminal   float64
	TerminalValue float64
}

// Helper for safe pointer dereference
func getValSafe(v *edgar.FSAPValue) float64 {
	if v != nil && v.Value != nil {
		return *v.Value
	}
	return 0.0
}

// 1. Dividend Discount Model (Dividend Based Valuation)
// Value = Sum(PV of Divs) + PV(Terminal Price based on Div growth)
func CalculateDDM(input EquityModelInput) EquityValuationResult {
	var pvDivs float64
	var lastDiv float64

	for i, proj := range input.Projections {
		// Extract Dividends
		div := 0.0
		if proj.CashFlow != nil && proj.CashFlow.FinancingActivities != nil {
			// DividendsPaid is usually negative inflow (outflow)
			val := getValSafe(proj.CashFlow.FinancingActivities.DividendsPaid)
			if val < 0 {
				div = -val
			} else {
				div = val
			}
		} else if proj.CashFlow != nil && proj.CashFlow.Dividends != nil {
			val := getValSafe(proj.CashFlow.Dividends)
			if val < 0 {
				div = -val
			} else {
				div = val
			}
		}

		discountFactor := 1.0 / math.Pow(1.0+input.CostOfEquity, float64(i+1))
		pvDivs += div * discountFactor

		if i == len(input.Projections)-1 {
			lastDiv = div
		}
	}

	// Terminal Value
	terminalVal := 0.0
	if input.CostOfEquity > input.TerminalGrowth {
		terminalVal = lastDiv * (1 + input.TerminalGrowth) / (input.CostOfEquity - input.TerminalGrowth)
	}

	pvTerminal := terminalVal / math.Pow(1.0+input.CostOfEquity, float64(len(input.Projections)))

	totalEquityVal := pvDivs + pvTerminal
	sharePrice := totalEquityVal / input.SharesOutstanding

	return EquityValuationResult{
		ModelName:     "Dividend Based Valuation",
		EquityValue:   totalEquityVal,
		SharePrice:    sharePrice,
		PV_Stream:     pvDivs,
		PV_Terminal:   pvTerminal,
		TerminalValue: terminalVal,
	}
}

// 2. Residual Income Valuation (RIM)
// Value = BookValue_0 + Sum(PV of Residual Income) + PV(Terminal RI)
// RI_t = NetIncome_t - (Ke * BookValue_{t-1})
func CalculateResidualIncome(input EquityModelInput) EquityValuationResult {
	var pvRI float64
	prevBookValue := input.CurrentBookValue
	var lastRI float64

	for i, proj := range input.Projections {
		// Extract Net Income
		ni := 0.0
		if proj.IncomeStatement != nil {
			if proj.IncomeStatement.NetIncomeSection != nil {
				ni = getValSafe(proj.IncomeStatement.NetIncomeSection.NetIncomeToCommon)
			} else {
				// Fallback or explicit NetIncome field check if exists (it doesn't in legacy list so verify)
				// Assuming standard Engine populates NetIncomeSection
			}
		}

		// Calculate Capital Charge
		capitalCharge := prevBookValue * input.CostOfEquity

		// Residual Income
		ri := ni - capitalCharge
		lastRI = ri

		// Discount
		discountFactor := 1.0 / math.Pow(1.0+input.CostOfEquity, float64(i+1))
		pvRI += ri * discountFactor

		// Update Book Value: B_t = B_{t-1} + NI - Div
		div := 0.0
		if proj.CashFlow != nil && proj.CashFlow.FinancingActivities != nil {
			val := getValSafe(proj.CashFlow.FinancingActivities.DividendsPaid)
			if val < 0 {
				div = -val
			} else {
				div = val
			}
		}

		prevBookValue = prevBookValue + ni - div
	}

	// Terminal Value of Residual Income
	terminalRIVal := 0.0
	if input.CostOfEquity > input.TerminalGrowth {
		terminalRIVal = lastRI * (1 + input.TerminalGrowth) / (input.CostOfEquity - input.TerminalGrowth)
	}

	pvTerminalRI := terminalRIVal / math.Pow(1.0+input.CostOfEquity, float64(len(input.Projections)))

	totalEquityVal := input.CurrentBookValue + pvRI + pvTerminalRI
	sharePrice := totalEquityVal / input.SharesOutstanding

	return EquityValuationResult{
		ModelName:     "Residual Income Valuation",
		EquityValue:   totalEquityVal,
		SharePrice:    sharePrice,
		PV_Stream:     pvRI,
		PV_Terminal:   pvTerminalRI,
		TerminalValue: terminalRIVal,
	}
}

// 3. Free Cash Flow to Equity (FCFE) Valuation
// FCFE = CFO - CapEx + NetBorrowing
func CalculateFCFE(input EquityModelInput) EquityValuationResult {
	var pvFCFE float64
	var lastFCFE float64

	for i, proj := range input.Projections {
		// CFO
		cfo := 0.0
		if proj.CashFlow != nil {
			if proj.CashFlow.CashSummary != nil {
				cfo = getValSafe(proj.CashFlow.CashSummary.NetCashOperating)
			} else if proj.CashFlow.OperatingActivities != nil {
				// Fallback to recalculate? Not needed if Engine works.
				// cfo = ...
			}
		}

		// CapEx (Subtract)
		capex := 0.0
		if proj.CashFlow != nil {
			if proj.CashFlow.InvestingActivities != nil {
				capex = getValSafe(proj.CashFlow.InvestingActivities.Capex)
			} else {
				capex = getValSafe(proj.CashFlow.Capex)
			}
		}

		// Net Borrowing
		netBorrowing := 0.0
		if proj.CashFlow != nil && proj.CashFlow.FinancingActivities != nil {
			proceeds := getValSafe(proj.CashFlow.FinancingActivities.DebtProceeds)
			repayments := getValSafe(proj.CashFlow.FinancingActivities.DebtRepayments)
			netBorrowing = proceeds + repayments // Assuming repayments are negative magnitude
		}

		// FCFE Logic: CFO + CapEx(Neg) + NetBorrowing
		fcfe := cfo + capex + netBorrowing
		lastFCFE = fcfe

		discountFactor := 1.0 / math.Pow(1.0+input.CostOfEquity, float64(i+1))
		pvFCFE += fcfe * discountFactor
	}

	// Terminal Value
	terminalVal := 0.0
	if input.CostOfEquity > input.TerminalGrowth {
		terminalVal = lastFCFE * (1 + input.TerminalGrowth) / (input.CostOfEquity - input.TerminalGrowth)
	}

	pvTerminal := terminalVal / math.Pow(1.0+input.CostOfEquity, float64(len(input.Projections)))

	totalEquityVal := pvFCFE + pvTerminal
	sharePrice := totalEquityVal / input.SharesOutstanding

	return EquityValuationResult{
		ModelName:     "Free Cash Flow Valuation (FCFE)",
		EquityValue:   totalEquityVal,
		SharePrice:    sharePrice,
		PV_Stream:     pvFCFE,
		PV_Terminal:   pvTerminal,
		TerminalValue: terminalVal,
	}
}

func CalculateMarketToBookRI(input EquityModelInput) EquityValuationResult {
	res := CalculateResidualIncome(input)
	res.ModelName = "Residual Income Market-to-Book Valuation"
	return res
}
