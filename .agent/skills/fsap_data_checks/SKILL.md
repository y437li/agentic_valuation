---
name: fsap_data_checks
description: Skill to perform financial integrity checks (Assets = L + E, etc.) using the Go calculation engine.
---

# Financial Data Checks Skill

This skill triggers the Go-based calculation engine to validate the integrity of the mapped FSAP data.

## Logic
1.  **Identity Guard**: Assets must equal Total Liabilities + Equity.
2.  **Clean Surplus**: Net Income must align with changes in Retained Earnings and Dividends.
3.  **Cash Flow Tie-in**: Net Change in Cash (CF Statement) must match Change in Cash (Balance Sheet).

## Execution
// turbo
Run `go run services/calc-engine/main.go --mode=check --data=<json_data>`
