---
name: tenk_section_parser
description: Skill to parse 10-K filings section-by-section (Item 1 through 15) for incremental agent processing.
---

# 10-K Section Parser Skill

This skill handles parsing and processing of 10-K annual reports by **individual sections**, avoiding sending the entire document to the agent at once.

## 10-K Section Structure (SEC Form)

| Item | Section Name | FSAP Relevance |
|------|--------------|----------------|
| **1** | Business | Industry context, segments |
| **1A** | Risk Factors | Valuation risks |
| **1B** | Unresolved Staff Comments | Red flags |
| **1C** | Cybersecurity | Risk assessment |
| **2** | Properties | Fixed assets context |
| **3** | Legal Proceedings | Contingent liabilities |
| **4** | Mine Safety Disclosures | Industry-specific |
| **5** | Market for Common Equity | Share data, dividends |
| **6** | Selected Financial Data | ⚠️ Discontinued Feb 2021 |
| **7** | MD&A | 🔥 Key for forecasting |
| **7A** | Market Risk Disclosures | Interest/FX exposure |
| **8** | Financial Statements | 🔥 Core data + Notes |
| **9** | Accounting Disagreements | Audit flags |
| **9A** | Controls and Procedures | Internal controls |
| **9B** | Other Information | Misc disclosures |
| **10** | Directors & Governance | Management info |
| **11** | Executive Compensation | Comp structure |
| **12** | Security Ownership | Insider holdings |
| **13** | Related Transactions | Related party |
| **14** | Accountant Fees | Audit fees |
| **15** | Exhibits | Schedules, exhibits |

## Processing Priority

### Tier 1 (Core Financial Data)
1. **Item 8** - Financial Statements and Notes
2. **Item 7** - MD&A (forward guidance, segment analysis)

### Tier 2 (Valuation Context)
3. **Item 1** - Business description, segments
4. **Item 1A** - Risk factors
5. **Item 5** - Share data, buybacks

### Tier 3 (Due Diligence)
6. **Item 3** - Legal proceedings
7. **Item 7A** - Market risk
8. **Item 9-9B** - Controls, audit

### Tier 4 (Optional)
9. Items 10-15 (Governance, compensation, etc.)

## Workflow

```
┌──────────────────┐
│  User Upload     │──┐
│  (PDF/HTML)      │  │
└──────────────────┘  │
                      ▼
┌──────────────────┐  ┌─────────────────┐
│  SEC EDGAR API   │─▶│  Section Parser │
│  (Auto-fetch)    │  │  (Go/Python)    │
└──────────────────┘  └────────┬────────┘
                               │
           ┌───────────────────┼───────────────────┐
           ▼                   ▼                   ▼
    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
    │  Item 8     │    │  Item 7     │    │  Item 1     │
    │  (JSON)     │    │  (JSON)     │    │  (JSON)     │
    └──────┬──────┘    └──────┬──────┘    └──────┬──────┘
           │                  │                  │
           ▼                  ▼                  ▼
    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
    │ Data Agent  │    │ Analysis    │    │ Context     │
    │ (Extract $) │    │ Agent       │    │ Agent       │
    └─────────────┘    └─────────────┘    └─────────────┘
```

## Section Extraction Patterns

### HTML Pattern (SEC Filing)
```regex
<a name="item1".*?>(.*?)<a name="item1a"
```

### Common Markers
```
ITEM 1. BUSINESS
ITEM 1A. RISK FACTORS
Part I
Item 7. Management's Discussion...
```

## Data Output Format

Each section outputs to:
```
uploaded_files/{file_id}/sections/
├── item_1_business.json
├── item_1a_risk_factors.json
├── item_7_mda.json
├── item_8_financials.json
│   ├── balance_sheet.json
│   ├── income_statement.json
│   ├── cash_flow.json
│   └── notes/
│       ├── note_1_accounting_policies.json
│       ├── note_2_revenue.json
│       └── ...
└── metadata.json
```

## Agent Prompts (Section-Specific)

### Item 8 Prompt (Financial Statements)
```
You are processing Item 8 of a 10-K filing.
Extract the following INTO STRUCTURED JSON:
1. Consolidated Balance Sheet (all years shown)
2. Consolidated Income Statement (all years shown)
3. Consolidated Cash Flow Statement (all years shown)
4. List of Notes with their titles

DO NOT summarize. Extract exact numbers with their units.
```

### Item 7 Prompt (MD&A)
```
You are processing Item 7 (MD&A) of a 10-K filing.
Extract:
1. Revenue discussion by segment
2. Cost structure changes
3. Forward-looking statements (guidance)
4. Key metrics management highlights

Focus on QUANTITATIVE statements only.
```

## Execution

### Step 1: Parse Sections
// turbo
```bash
go run cmd/parser/main.go --file=<path> --mode=split
```

### Step 2: Process Priority Sections
// turbo
```bash
go run cmd/agent/main.go --section=item8 --file-id=<id>
go run cmd/agent/main.go --section=item7 --file-id=<id>
```

### Step 3: Validate
// turbo
```bash
go run cmd/validator/main.go --file-id=<id> --check=balance
```

## Database Tables Used

- `uploaded_files` - File metadata and status
- `file_sections` - Individual section content (new table needed)
- `section_extractions` - Agent extraction results

## Notes

1. **Never send full 10-K** - Files are 200+ pages, will exceed context
2. **Process Item 8 first** - Core financial data
3. **Cache sections** - Avoid re-parsing
4. **Track extraction status** - Show progress to user
