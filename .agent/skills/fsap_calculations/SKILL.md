---
name: fsap_calculations
description: Skill to handle section summations (Grey) and ratio calculations (White) using the Go calculation engine.
---

# FSAP Calculations Skill

This skill handles all arithmetic summations and complex financial ratio calculations (Growth, CAGR, RoE, etc.) using high-performance Go code.

## Scopes
1.  **Section Summations (Grey Area)**: Aggregating current assets, total liabilities, etc.
2.  **Calculated Items (White Cells)**: Gross Profit, EBITDA, Net Income (Computed).
3.  **Analysis Ratios**: Sales Growth, A/R Growth, Op CF / Net Income, CAGR.

## Execution
// turbo
Run `go run services/calc-engine/main.go --mode=calculate --data=<json_data>`
