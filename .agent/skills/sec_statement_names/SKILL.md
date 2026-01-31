---
description: Reference collection of financial statement names used in SEC 10-K and 10-Q filings
---

# Financial Statement Names Reference

This skill provides a comprehensive reference of all possible names companies may use for financial statements in SEC filings. Use this when parsing 10-K/10-Q documents or configuring LLM extraction prompts.

## Balance Sheet Names

### Consolidated Versions (Most Common)
- Consolidated Balance Sheets
- CONSOLIDATED BALANCE SHEETS
- Consolidated Balance Sheet
- Consolidated Statements of Financial Position
- CONSOLIDATED STATEMENTS OF FINANCIAL POSITION
- Consolidated Statements of Financial Condition (Banks/Financial)

### Standalone Versions
- Balance Sheets
- Balance Sheet
- BALANCE SHEETS
- Statements of Financial Position
- Statement of Financial Position
- Statements of Financial Condition

### Condensed Versions (Quarterly/Interim)
- Condensed Consolidated Balance Sheets
- Condensed Balance Sheets
- Unaudited Condensed Consolidated Balance Sheets
- Condensed Statements of Financial Position

### Avoid These (Parent Company Only)
- Parent Company Only Balance Sheet
- Registrant Only Balance Sheet
- Schedule I - Condensed Financial Information of Registrant
- Condensed Balance Sheet of Parent Company

---

## Income Statement Names

### Consolidated Versions
- Consolidated Statements of Operations
- CONSOLIDATED STATEMENTS OF OPERATIONS
- Consolidated Statements of Income
- CONSOLIDATED STATEMENTS OF INCOME
- Consolidated Statements of Earnings
- Consolidated Statements of Income (Loss)
- Consolidated Statements of Operations and Comprehensive Income

### Standalone Versions
- Statements of Operations
- Statements of Income
- Statements of Earnings
- Income Statements
- Statement of Operations
- Statement of Income

### Condensed Versions
- Condensed Consolidated Statements of Operations
- Condensed Statements of Operations
- Unaudited Condensed Consolidated Statements of Operations

### Avoid These
- Parent Company Only Statement of Operations
- Schedule I - Condensed Parent Company Statements of Operations

---

## Cash Flow Statement Names

### Consolidated Versions
- Consolidated Statements of Cash Flows
- CONSOLIDATED STATEMENTS OF CASH FLOWS
- Consolidated Statement of Cash Flows
- Consolidated Statements of Cash Flow

### Standalone Versions
- Statements of Cash Flows
- Statement of Cash Flows
- Cash Flow Statements
- STATEMENTS OF CASH FLOWS

### Condensed Versions
- Condensed Consolidated Statements of Cash Flows
- Condensed Statements of Cash Flows
- Unaudited Condensed Consolidated Statements of Cash Flows

### Avoid These
- Parent Company Only Statement of Cash Flows
- Schedule I - Condensed Parent Company Cash Flows

---

## Comprehensive Income Statement Names

### Consolidated Versions
- Consolidated Statements of Comprehensive Income
- CONSOLIDATED STATEMENTS OF COMPREHENSIVE INCOME
- Consolidated Statements of Comprehensive Income (Loss)
- Consolidated Statements of Operations and Comprehensive Income

### Standalone Versions
- Statements of Comprehensive Income
- Statement of Comprehensive Income
- Statements of Comprehensive Income (Loss)

---

## Stockholders' Equity Statement Names

### Consolidated Versions
- Consolidated Statements of Stockholders' Equity
- CONSOLIDATED STATEMENTS OF STOCKHOLDERS' EQUITY
- Consolidated Statements of Changes in Stockholders' Equity
- Consolidated Statements of Shareholders' Equity
- Consolidated Statements of Equity

### Standalone Versions
- Statements of Stockholders' Equity
- Statements of Changes in Stockholders' Equity
- Statement of Stockholders' Equity

---

## Industry-Specific Variations

### Banks and Financial Institutions
- Consolidated Statements of Condition (instead of Balance Sheet)
- Consolidated Statements of Financial Condition
- Statement of Condition

### Insurance Companies
- Consolidated Balance Sheets - Statutory Basis
- Consolidated Statements of Operations - Statutory Basis

### Real Estate (REITs)
- Consolidated Balance Sheets
- Consolidated Statements of Operations (same as others)

---

## Section Headers to Skip (Not Financial Statements)

These appear in 10-K but are NOT the main financial statements:
- Schedule I - Condensed Financial Information of Registrant
- Schedule II - Valuation and Qualifying Accounts
- Supplemental Consolidating Data
- Segment Information
- Selected Financial Data
- Financial Statement Schedules

---

## Parser Pattern Recommendations

When building regex patterns, account for:
1. **Case variations**: Mix of uppercase, lowercase, title case
2. **Whitespace**: Variable spacing between words
3. **Punctuation**: Hyphens, en-dashes, em-dashes
4. **Parentheses**: "(Continued)", "(Unaudited)"
5. **Year references**: "December 31, 2024" in titles

### Recommended Regex Patterns

```go
// Balance Sheet patterns (order of priority)
balanceSheetPatterns := []string{
    `(?i)(Consolidated\s+Balance\s+Sheets?)`,
    `(?i)(Consolidated\s+Statements?\s+of\s+(Financial\s+)?(Position|Condition))`,
    `(?i)(Balance\s+Sheets?)\s*[\n<]`,
    `(?i)(Statements?\s+of\s+Financial\s+(Position|Condition))`,
    `(?i)(Condensed\s+(Consolidated\s+)?Balance\s+Sheets?)`,
}

// Income Statement patterns
incomeStatementPatterns := []string{
    `(?i)(Consolidated\s+Statements?\s+of\s+(Operations|Income|Earnings))`,
    `(?i)(Statements?\s+of\s+(Operations|Income|Earnings))`,
    `(?i)(Income\s+Statements?)`,
    `(?i)(Condensed\s+(Consolidated\s+)?Statements?\s+of\s+(Operations|Income))`,
}

// Cash Flow patterns
cashFlowPatterns := []string{
    `(?i)(Consolidated\s+Statements?\s+of\s+Cash\s+Flows?)`,
    `(?i)(Statements?\s+of\s+Cash\s+Flows?)`,
    `(?i)(Cash\s+Flow\s+Statements?)`,
    `(?i)(Condensed\s+(Consolidated\s+)?Statements?\s+of\s+Cash\s+Flows?)`,
}
```

---

## LLM Prompt Integration

When instructing an LLM to extract financial data, include these clarifications:

```
IMPORTANT - Financial Statement Selection:
1. Use the FIRST/MAIN set of financial statements (company-wide)
2. AVOID these sections (they appear AFTER main statements):
   - "Parent Company Only"
   - "Registrant Only"
   - "Schedule I - Condensed Financial Information of Registrant"
   - "Supplemental Consolidating Data"
3. Correct table names to look for:
   - Balance Sheet: "Consolidated Balance Sheets" OR "Balance Sheets"
   - Income: "Consolidated Statements of Operations" OR "Statements of Income"
   - Cash Flow: "Consolidated Statements of Cash Flows" OR "Statements of Cash Flows"
4. If multiple statements exist, use the FIRST one
5. Use the most recent fiscal year column (rightmost)
```
