---
name: fsap_business_essence
description: Skill to identify key performance drivers and perform cost structure analysis based on different business models.
---

# Business Essence & Cost Structure Analysis Skill

This skill enables the agent to act as a "Business Strategist" by identifying critical financial drivers based on the company's industry and business model.

## 1. Business Model Identification
The agent must categorize the company into one or more frameworks:
- **Capital Intensive (Manufacturing/Auto)**: Focus on PPE, Depreciation, R&D, and Gross Margin.
- **Cloud/Software (SaaS)**: Focus on Sales & Marketing, R&D, Deferred Revenue, and Net Retention.
- **Consumer/Retail**: Focus on Inventory Turnover, Accounts Payable, and Operating Margin.
- **Resource/Energy**: Focus on Capex, Commodity Prices, and Exploration costs.

## 2. Granular Segment Analysis (As per Reference)
The agent must drill down into the business segments to understand the "Sales Mix":
- **Dynamic Segment Discovery**: Do NOT rely on a fixed list. Read the "Segment Reporting" note and identify the current active reporting units.
- **New Feature Detection**: Watch for "First-time independent disclosure" or "Segment reclassification." If a new business line appears (e.g., Ford Model e), automatically add it to the FSAP structure.
- **Segment Identification**: Extract revenue by identified product/service line.
- **Growth Attribution**: Calculate growth rates per segment and identify the "Primary Growth Engine."
- **CAGR Calculation**: Perform 3-5 year historical CAGR analysis for each segment to contrast with user/analyst "Sales Forecasts."

## 3. Capex & Depreciation Alignment
- **Capex intensity**: Analyze "Net CAPEX as a percent of Revenues" and "Gross PP&E."
- **Useful Life Estimation**: Back-calculate "Implied Average Useful Life in Years" from the Depreciation expense vs. Depreciable PP&E base.
- **Reinvestment Rate**: Compare Net Capex with Depreciation to determine if the company is in a growth or maintenance phase.

## 4. Key Driver Extraction Logic
1.  **Identify Primary Costs**: Look into `other_operating_expenses_x` placeholders...
2.  **Profitability Drivers**: Identify items contributing most to the change in Operating Income.
3.  **Cross-Reference Notes**: Link extracted drivers to specific management discussion and analysis (MD&A) or Note sections.

## 5. Cost Structure Table Generation (Common Size)
Generate a structured report containing:
- **Common-Sized Income Statement**: Every line as a % of Revenue.
- **Common-Sized Segment Analysis**: Segment revenue as % of Total Net Sales.
- **Forecast vs. History**: Compare "Implied Revenue Growth Rates" in forecasts with historical CAGRs.
- **Reinvestment metrics**: Capex/Sales, Capex/Depreciation.

## 6. Analytical Reasoning
- **Anomaly Detection**: Flag cost items growing significantly faster than revenue.
- **Essence Validation**: Check if the financial structure matches the claimed business model.
