package debate

import (
	"context"
	"fmt"
	"time"

	"agentic_valuation/pkg/core/calc"
)

// RoleQuant is a specialized agent that generates baseline assumptions
// based strictly on quantitative historical data.
type RoleQuant struct {
	BaseAgent
}

// Ensure RoleQuant implements DebateAgent interface
var _ DebateAgent = &RoleQuant{}

func NewRoleQuant(id string) *RoleQuant {
	return &RoleQuant{
		BaseAgent: BaseAgent{
			// utilizing Synthesizer role or adding a new one?
			// Let's add RoleQuant to AgentRole enum in debate_types.go if strict.
			// For now, let's use RoleFundamental but with a specific ID/SystemPrompt, or cast string.
			// Actually, let's use a new constant in debate_types.go if possible, or just reusing one.
			// Let's cast:
			role: AgentRole("quant"),
			systemPrompt: `You are a Quantitative Analyst. Your goal is to strictly calculate historical averages and common-size ratios 
to serve as a neutral baseline for financial projections. You do not speculate.`,
		},
	}
}

func (a *RoleQuant) Name() string {
	return "Quantitative Analyst"
}

func (a *RoleQuant) Role() AgentRole {
	return AgentRole("quant")
}

// Generate produces a debate message with the calculated baselines
func (a *RoleQuant) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	// 1. Extract Latest Historical Data
	if shared.MaterialPool == nil {
		return DebateMessage{}, fmt.Errorf("material pool is empty")
	}

	latestYear := 0
	for year := range shared.MaterialPool.FinancialHistory[0].HistoricalData {
		if year > latestYear {
			latestYear = year
		}
	}

	// 2. Calculate Defaults using pkg/core/calc
	var defaults calc.CommonSizeDefaults
	// We need to find the YearData for the latest year.
	// shared.MaterialPool.FinancialHistory is []*edgar.FSAPDataResponse
	// We assume index 0 is the primary ticker data.
	if len(shared.MaterialPool.FinancialHistory) > 0 {
		histDataMap := shared.MaterialPool.FinancialHistory[0].HistoricalData
		if data, ok := histDataMap[latestYear]; ok {
			defaults = calc.CalculateCommonSizeDefaults(&data)
		} else {
			defaults = calc.CalculateCommonSizeDefaults(nil)
		}
	} else {
		defaults = calc.CalculateCommonSizeDefaults(nil)
	}

	// 3. Format message
	content := fmt.Sprintf(
		"I have calculated the baseline common-size assumptions based on FY%d actuals:\n"+
			"- Revenue Growth Action: %.1f%%\n"+
			"- COGS: %.1f%% of Revenue\n"+
			"- SG&A: %.1f%% of Revenue\n"+
			"- R&D: %.1f%% of Revenue\n"+
			"- Tax Rate: %.1f%%\n"+
			"- Stock-Based Comp: %.1f%% of Revenue\n"+
			"- Implied Debt Interest: %.1f%%\n\n"+
			"These should serve as the starting point for our debate.",
		latestYear,
		defaults.RevenueAction*100,
		defaults.COGSPercent*100,
		defaults.SGAPercent*100,
		defaults.RDPercent*100,
		defaults.TaxRate*100,
		defaults.StockBasedCompPercent*100,
		defaults.DebtInterestRate*100,
	)

	return DebateMessage{
		AgentRole: a.Role(),
		AgentName: a.Name(),
		Content:   content,
		Timestamp: time.Now(),
	}, nil
}

// GenerateBaselineAssumptions returns the raw map for the Orchestrator to seed the context
func (a *RoleQuant) GenerateBaselineAssumptions(pool *MaterialPool) map[string]float64 {
	if pool == nil || len(pool.FinancialHistory) == 0 {
		return nil
	}

	histDocs := pool.FinancialHistory[0].HistoricalData
	latestYear := 0
	for year := range histDocs {
		if year > latestYear {
			latestYear = year
		}
	}

	var defaults calc.CommonSizeDefaults
	if data, ok := histDocs[latestYear]; ok {
		defaults = calc.CalculateCommonSizeDefaults(&data)
	} else {
		defaults = calc.CalculateCommonSizeDefaults(nil)
	}

	assumptions := make(map[string]float64)
	assumptions["rev_growth"] = defaults.RevenueAction
	assumptions["cogs_pct"] = defaults.COGSPercent
	assumptions["sga_pct"] = defaults.SGAPercent
	assumptions["rd_pct"] = defaults.RDPercent
	assumptions["tax_rate"] = defaults.TaxRate
	assumptions["stock_based_comp_percent"] = defaults.StockBasedCompPercent
	assumptions["debt_interest_rate"] = defaults.DebtInterestRate

	return assumptions
}
