---
name: fsap_assumption_gen
description: 4-Step LLM-based assumption generation combining historical data, guidance, earnings calls, and macro research.
---

# LLM Assumption Generation Skill

This skill defines a **4-Step LLM Workflow** for generating valuation assumptions by synthesizing multiple data sources.

## Overview: The 4-Step Assumption Pipeline

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     LLM ASSUMPTION GENERATION                           │
├──────────────┬──────────────┬──────────────┬──────────────┬────────────┤
│   STEP 1     │   STEP 2     │   STEP 3     │   STEP 4     │   OUTPUT   │
│  Data Map    │  Guidance    │  8-K/Calls   │  Macro Web   │  Synthesize│
│  (Common %)  │  (Mgmt View) │  (Events)    │  (External)  │            │
└──────────────┴──────────────┴──────────────┴──────────────┴────────────┘
```

---

## Step 1: Data Mapping (Historical + Common Size)

### Purpose
Extract historical financial ratios and generate **Common Size Analysis** as baseline assumptions.

### Agent Prompt
```
Analyze the provided FSAP data for {company} over the past {n} fiscal years.

Extract:
1. Key financial ratios (Gross Margin, EBIT Margin, Net Margin, ROE, etc.)
2. Common Size ratios for each line item as % of Revenue
3. Calculate the AVERAGE of common size ratios over the period
4. Identify any significant TRENDS (improving/declining)

Output JSON:
{
  "baseline_assumptions": {
    "gross_margin": {"avg": 0.22, "trend": "stable", "range": [0.20, 0.24]},
    "sga_percent": {"avg": 0.08, "trend": "declining", "range": [0.07, 0.09]},
    "capex_percent": {"avg": 0.045, "trend": "stable", "range": [0.04, 0.05]}
  },
  "data_source": "FSAP Historical (FY2019-2023)"
}
```

### Data Source
- `fsap_data` table (historical financials)
- SEC 10-K Item 8 (Financial Statements)

---

## Step 2: Financial Statement Guidance (Management Guidance)

### Purpose
Extract **forward-looking statements** from official filings (MD&A, 10-K Item 7).

### Agent Prompt
```
Extract management's forward guidance from {company}'s 10-K/10-Q filings.

Look for:
1. Revenue growth expectations ("We expect revenue to grow by X%")
2. Margin targets ("We aim to achieve operating margin of X%")
3. Capex plans ("Planned capital expenditures of $X billion")
4. Segment-specific guidance (by product line or geography)

Output JSON:
{
  "management_guidance": {
    "revenue_growth": {"value": 0.05, "quote": "We expect 5% growth in FY2025", "source": "10-K MD&A p.42"},
    "ebit_margin_target": {"value": 0.10, "quote": "Target 10% operating margin", "source": "10-K Item 7"},
    "capex_plan": {"value": 8000000000, "quote": "$8B capex for EV transition", "source": "10-K Item 7"}
  },
  "confidence": "HIGH",
  "fiscal_year": "FY2025"
}
```

### Data Source
- SEC 10-K Item 7 (MD&A)
- SEC 10-Q (Quarterly Updates)

---

## Step 3: Earnings Calls & 8-K Events (Event-Driven)

### Purpose
Capture **real-time updates** and **soft signals** from earnings calls and 8-K filings.

### Agent Prompt
```
Analyze {company}'s recent earnings calls and 8-K filings for assumption updates.

Extract from Earnings Calls:
1. CEO/CFO forward statements
2. Q&A insights on margins, growth, challenges
3. Tone analysis (optimistic/cautious/defensive)

Extract from 8-K:
1. Material events (M&A, restructuring, executive changes)
2. Preliminary earnings data (before 10-Q)
3. Contract announcements, legal settlements

Output JSON:
{
  "earnings_call_signals": [
    {"date": "2024-01-25", "signal": "CFO expects 3% margin compression from tariffs", "impact": "negative", "parameter": "gross_margin"}
  ],
  "8k_events": [
    {"date": "2024-02-01", "type": "Contract", "description": "$2B fleet order from Hertz", "impact": "positive", "parameter": "revenue_growth"}
  ],
  "overall_sentiment": "cautiously_optimistic"
}
```

### Data Source
- SEC 8-K filings
- Earnings call transcripts (SeekingAlpha, company IR)
- Press releases

---

## Step 4: Macro Web Research (Macro Assumptions)

### Purpose
Incorporate **external macro factors** that affect the company's assumptions.

### Agent Prompt
```
Search for macro factors affecting {company}'s industry and geography.

Research categories:
1. **Interest Rates**: Fed policy impact on financing costs
2. **Commodity Prices**: Steel, lithium, oil (relevant to industry)
3. **Regulatory**: EV subsidies, emissions standards, tariffs
4. **Currency**: FX exposure for multinational operations
5. **GDP/Consumer**: Economic growth affecting demand

Output JSON:
{
  "macro_assumptions": {
    "interest_rate": {"value": 0.0525, "source": "Fed Funds Rate Dec 2024", "impact": "WACC +0.5%"},
    "lithium_price_trend": {"trend": "declining", "source": "Bloomberg Commodities", "impact": "battery cost -10%"},
    "ev_subsidy": {"status": "extended", "source": "IRA 2024 Update", "impact": "demand +5%"}
  },
  "search_sources": ["fed.gov", "bloomberg.com", "congress.gov"]
}
```

### Tools Used
- `search_web` tool
- LLM synthesis of search results

---

## Synthesis: Combining All 4 Steps

### Final Output Structure
```json
{
  "company": "Ford Motor Co",
  "fiscal_year": "FY2025E",
  "assumptions": {
    "revenue_growth": {
      "value": 0.04,
      "sources": {
        "step1_historical": {"value": 0.03, "weight": 0.2},
        "step2_guidance": {"value": 0.05, "weight": 0.4},
        "step3_8k_events": {"value": 0.05, "weight": 0.2},
        "step4_macro": {"value": 0.03, "weight": 0.2}
      },
      "calculation": "Weighted Average"
    },
    "gross_margin": {
      "value": 0.18,
      "sources": {...}
    }
  },
  "confidence_score": 0.82,
  "last_updated": "2024-01-18T12:00:00Z"
}
```

### Weight Logic
| Source | Default Weight | Condition |
|:---|:---|:---|
| Step 1 (Historical) | 20% | Baseline from proven track record |
| Step 2 (Guidance) | 40% | Management's direct projection |
| Step 3 (8-K/Calls) | 20% | Real-time adjustments |
| Step 4 (Macro) | 20% | External environment factors |

> **User Override**: Users can adjust weights or veto specific sources via HITL interface.

---

## Agent Execution Flow

```python
# Pseudo-code
def generate_assumptions(company, fiscal_year):
    # Step 1: Historical baseline
    step1 = llm.extract_common_size(fsap_data[company])
    
    # Step 2: Management guidance
    step2 = llm.extract_guidance(sec_10k[company])
    
    # Step 3: Events & Calls
    step3 = llm.extract_events(sec_8k[company], earnings_calls[company])
    
    # Step 4: Macro research
    step4 = llm.search_macro(industry=company.industry)
    
    # Synthesize
    final = llm.synthesize([step1, step2, step3, step4], weights=DEFAULT_WEIGHTS)
    
    return final
```

## Related Skills
- `fsap_data`: Provides historical financial data
- `fsap_ingest_logic`: Handles 10-K/10-Q/8-K hierarchy
- `tenk_section_parser`: Extracts Item 7 (MD&A) and Item 8 (Financials)
- `fsap_business_essence`: Segment analysis and cost structure
