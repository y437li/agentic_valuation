package projection

import (
	"agentic_valuation/pkg/core/debate"
	"regexp"
	"strconv"
	"strings"
)

// ConvertDebateReportToAssumptions parses the debate outcome (Markdown Table) into engine inputs.
// It bridges the Qualitative Debate (Synthesizer) and the Quantitative Engine.
func ConvertDebateReportToAssumptions(report *debate.FinalDebateReport) ProjectionAssumptions {
	// 1. Default Fallbacks (Safe Defaults)
	ass := ProjectionAssumptions{
		RevenueGrowth:           0.05,
		COGSPercent:             0.40, // Gross Margin 60%
		SGAPercent:              0.20,
		SellingMarketingPercent: 0.10,
		GeneralAdminPercent:     0.10,
		RDPercent:               0.05,
		TaxRate:                 0.21,
		// WACC: 0.08, // Removed
		UnleveredBeta:     1.0,
		RiskFreeRate:      0.04,
		MarketRiskPremium: 0.05,
		PreTaxCostOfDebt:  0.05,
		TargetDebtEquity:  0.5,
		TerminalGrowth:    0.02,
		CapexPercent:      0.05,
	}

	if report == nil {
		return ass
	}

	// 2. Parse Markdown Table from Executive Summary (or raw content)
	// We look for rows like: | rev_growth | Revenue Growth | 12.5 | ...
	// Regex matches: | key | label | value | ...

	// Normalize content
	content := report.ExecutiveSummary
	lines := strings.Split(content, "\n")

	// Regex to find table rows
	// Matches: | key | ... | value | ...
	// Group 1: Key
	// Group 2: Label (ignored)
	// Group 3: Value
	rowRegex := regexp.MustCompile(`\|\s*([a-z_]+)\s*\|\s*[^|]+\|\s*([\d\.]+)\s*\|`)

	for _, line := range lines {
		matches := rowRegex.FindStringSubmatch(line)
		if len(matches) >= 3 {
			key := strings.TrimSpace(matches[1])
			valStr := strings.TrimSpace(matches[2])
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				continue
			}

			// Map keys to Struct Fields
			// Input from table is typically in Percent (e.g. 12.5 for 12.5%).
			// Engine expects Decimals (0.125)?
			// Check Prompt: "Value: X, Unit: %". If Agent writes "12.5", it implies %.
			// Standardize: if val > 1.0 (like 12.5), treat as %. If < 1.0 (0.125), treat as decimal?
			// Safer: Assume Input is % for specific keys.

			isPercent := true // Most are % in the prompt

			switch key {
			case "rev_growth":
				if isPercent {
					ass.RevenueGrowth = val / 100.0
				}
			case "cogs_percent":
				if isPercent {
					ass.COGSPercent = val / 100.0
				}
			case "sga_percent":
				if isPercent {
					ass.SGAPercent = val / 100.0
				}
			case "selling_marketing_percent":
				if isPercent {
					ass.SellingMarketingPercent = val / 100.0
				}
			case "general_admin_percent":
				if isPercent {
					ass.GeneralAdminPercent = val / 100.0
				}
			case "rd_percent":
				if isPercent {
					ass.RDPercent = val / 100.0
				}
			case "tax_rate":
				if isPercent {
					ass.TaxRate = val / 100.0
				}

			case "terminal_growth":
				if isPercent {
					ass.TerminalGrowth = val / 100.0
				}
			case "capex_percent", "capex_ratio":
				if isPercent {
					ass.CapexPercent = val / 100.0
				}
			// Dynamic WACC Components
			case "beta", "unlevered_beta":
				ass.UnleveredBeta = val // Not percent
			case "risk_free_rate":
				if isPercent {
					ass.RiskFreeRate = val / 100.0
				}
			case "market_risk_premium", "equity_risk_premium":
				if isPercent {
					ass.MarketRiskPremium = val / 100.0
				}
			case "cost_of_debt":
				if isPercent {
					ass.PreTaxCostOfDebt = val / 100.0
				}
			case "target_debt_equity", "target_leverage":
				ass.TargetDebtEquity = val // Ratio, not percent usually, though might be 0.5
			}
		}
	}

	return ass
}
