---
name: fsap_data
description: High-fidelity FSAP skill. Separates inputs (Green), summations (Grey), and calculations (White).
---

# FSAP Data Skill (Standardized Template)

## 0. Filings Context & Rolling Update
- **10-K (Master Baseline)**: When processing a 10-K, the agent must perform a full "Initiation." This includes extracting 3-5 years of history and establishing the definitive cost structure, segment definitions, and accounting policies.
- **10-Q (Maintenance/Delta)**: When processing a 10-Q, the agent must treat it as an incremental update. 
    - **TTM Calculation**: Automatically compute Trailing Twelve Months (TTM) for P&L items: `Current 9M/6M/3M + (Previous FY - Previous 9M/6M/3M)`.
    - **Note Fallback**: If a 10-Q note is summarized/incomplete, the agent must reference the most recent 10-K for full accounting policy definitions.
    - **Maintenance**: Update rolling indicators (AR turns, inventory turns) based on TTM data.

## 1. Balance Sheet Data

### Assets
| Category | Variable | Description | Source/Type |
| :--- | :--- | :--- | :--- |
| **GREEN** | `cash_and_equivalents` | Cash and cash equivalents | Data Input |
| **GREEN** | `short_term_investments` | Short-term investments | Data Input |
| **GREEN** | `accounts_receivable_net` | Accounts and notes receivable - net | Data Input |
| **GREEN** | `inventories` | Inventories | Data Input |
| **GREEN** | `finance_div_loans_leases_st` | Finance Div: Loans and Leases, ST | Data Input |
| **GREEN** | `finance_div_other_curr_assets` | Finance Div: Other Curr Assets | Data Input |
| **GREEN** | `other_assets` | Other assets| Data Input |
| **GREEN** | `other_current_assets_2` | Other current assets (2) | Data Input |
| **GREY** | `total_current_assets` | Total Current Assets | **Skill: fsap_calculations** (Sum) |
| **GREEN** | `long_term_investments` | Long-term investments | Data Input |
| **GREEN** | `deferred_charges_lt` | Deferred Charges, LT | Data Input |
| **GREEN** | `ppe_at_cost` | Property, plant, and equipment - at cost | Data Input |
| **GREEN** | `accumulated_depreciation` | <Accumulated depreciation> | Data Input |
| **WHITE** | `ppe_net` | Property, plant, and equipment - net | **Skill: fsap_calculations** (Calc) |
| **GREEN** | `intangibles` | Intangibles | Data Input |
| **GREEN** | `finance_div_loans_leases_lt` | Finance Div: Loans and Leases, LT | Data Input |
| **GREEN** | `finance_div_other_lt_assets` | Finance Div: Other LT Assets | Data Input |
| **GREEN** | `deferred_tax_assets_lt` | Deferred Tax Assets, LT | Data Input |
| **GREEN** | `other_noncurrent_asset_3` | Other noncurrent asset (3) | Data Input |
| **GREEN** | `other_long_term_assets_4` | Other long-term assets (4) | Data Input |
| **GREEN** | `restricted_cash` | Restricted Cash | Data Input |
| **GREY** | `total_assets` | Total Assets | **Skill: fsap_calculations** (Sum) |

### Liabilities and Equities
| Category | Variable | Description | Source/Type |
| :--- | :--- | :--- | :--- |
| **GREEN** | `accounts_payable` | Accounts payable | Data Input |
| **GREEN** | `accrued_liabilities` | Accrued liabilities | Data Input |
| **GREEN** | `notes_payable_short_term_debt` | Notes payable and short-term debt | Data Input |
| **GREEN** | `current_maturities_long_term_debt` | Current maturities of long-term debt | Data Input |
| **GREEN** | `current_operating_lease_liabilities` | Current operating lease liabilities | Data Input |
| **GREEN** | `finance_div_curr` | Finance Div: Curr | Data Input |
| **GREEN** | `other_current_liabilities_1` | Other Current Liabilities | Data Input |
| **GREEN** | `other_current_liabilities_2` | Other current liabilities (2) | Data Input |
| **GREY** | `total_current_liabilities` | Total Current Liabilities | **Skill: fsap_calculations** (Sum) |
| **GREEN** | `long_term_debt` | Long-term debt | Data Input |
| **GREEN** | `long_term_operating_lease_liabilities` | Long-term operating lease liabilities | Data Input |
| **GREEN** | `deferred_tax_liabilities` | Deferred tax liabilities | Data Input |
| **GREEN** | `finance_div_non_curr` | Finance Div: Non-Curr | Data Input |
| **GREEN** | `other_noncurrent_liabilities_2` | Other noncurrent liabilities (2) | Data Input |
| **GREY** | `total_liabilities` | Total Liabilities | **Skill: fsap_calculations** (Sum) |
| **GREEN** | `preferred_stock` | Preferred stock | Data Input |
| **GREEN** | `common_stock_apic` | Common stock + Additional paid-in capital | Data Input |
| **GREEN** | `retained_earnings_deficit` | Retained earnings <deficit> | Data Input |
| **GREEN** | `accum_other_comprehensive_income` | Accum. other comprehensive income <loss> | Data Input |
| **GREEN** | `treasury_stock_adjustments` | <Treasury stock> and other equity adjustments | Data Input |
| **GREY** | `total_common_shareholders_equity` | Total Common Shareholders' Equity | **Skill: fsap_calculations** (Sum) |
| **GREEN** | `noncontrolling_interests` | Noncontrolling interests | Data Input |
| **GREY** | `total_equity` | Total Equity | **Skill: fsap_calculations** (Sum) |
| **GREY** | `total_liabilities_and_equities` | Total Liabilities and Equities | **Skill: fsap_calculations** (Sum) |

## 2. Income Statement Data
| Category | Variable | Description | Source/Type |
| :--- | :--- | :--- | :--- |
| **GREEN** | `revenues` | Revenues | Data Input |
| **GREEN** | `cost_of_goods_sold` | <Cost of goods sold> | Data Input |
| **WHITE** | `gross_profit` | Gross Profit | **Skill: fsap_calculations** (Calc) |
| **GREEN** | `sga_expenses` | <Selling, general, and administrative expenses> | Data Input |
| **WHITE** | `operating_income` | Operating Income | **Skill: fsap_calculations** (Calc) |
| **GREEN** | `interest_expense` | <Interest expense> | Data Input |
| **WHITE** | `income_before_tax` | Income before tax | **Skill: fsap_calculations** (Calc) |
| **GREEN** | `income_tax_expense` | <Income tax expense> | Data Input |
| **WHITE** | `net_income` | Net Income | **Skill: fsap_calculations** (Calc) |

## 3. Cash Flow Statement Data
| Category | Variable | Description | Source/Type |
| :--- | :--- | :--- | :--- |
| **GREEN** | `net_income_start` | Net Income (as reported in CFS start) | Data Input |
| **GREEN** | `depreciation_amortization` | Depreciation & Amortization | Data Input |
| **GREEN** | `deferred_taxes` | Deferred Income Taxes | Data Input |
| **GREEN** | `stock_based_compensation` | Stock-based Compensation | Data Input |
| **GREEN** | `changes_in_working_capital` | Changes in Working Capital | Data Input |
| **GREEN** | `other_operating_items` | Other operating activities | Data Input |
| **GREY** | `net_cash_operating` | Net Cash from Operating Activities | **Skill: fsap_calculations** (Sum) |
| **GREEN** | `capex` | Capital Expenditures | Data Input |
| **GREEN** | `investments_acquired_sold_net` | Investments Acquired/Sold (Net) | Data Input |
| **GREEN** | `other_investing_items` | Other investing activities | Data Input |
| **GREY** | `net_cash_investing` | Net Cash from Investing Activities | **Skill: fsap_calculations** (Sum) |
| **GREEN** | `debt_issuance_retirement_net` | Debt Issuance/Retirement (Net) | Data Input |
| **GREEN** | `share_repurchases` | Share Repurchases | Data Input |
| **GREEN** | `dividends` | Dividends Paid | Data Input |
| **GREEN** | `other_financing_items` | Other financing activities | Data Input |
| **GREY** | `net_cash_financing` | Net Cash from Financing Activities | **Skill: fsap_calculations** (Sum) |
| **GREEN** | `effect_exchange_rate` | Effect of Exchange Rate on Cash | Data Input |
| **WHITE** | `net_change_in_cash` | Net Change in Cash | **Skill: fsap_calculations** (Calc) |

## 4. Supplemental Data
| Category | Variable | Description | Source/Type |
| :--- | :--- | :--- | :--- |
| **GREEN** | `eps_basic` | Earnings Per Share (Basic) | Data Input |
| **GREEN** | `eps_diluted` | Earnings Per Share (Diluted) | Data Input |
| **GREEN** | `shares_outstanding_basic` | Weighted Avg Shares (Basic) | Data Input |
| **GREEN** | `shares_outstanding_diluted` | Weighted Avg Shares (Diluted) | Data Input |
| **GREEN** | `effective_tax_rate` | Effective Tax Rate (%) | Data Input |

## 5. Financial Data Checks
All checks are performed by **Skill: fsap_data_checks**.
- `check_assets_liab_equity`: Assets - Liabilities - Equities
- `check_net_income_comp_rep`: Net Income (computed) - Net Income (reported)
- `check_cash_changes`: Net Change in Cash (Calc) - Reported CF Change

## 6. Analysis Graphs (Go-based)
Calculated by **Skill: fsap_calculations** calling Go code.
- **Red Flags Analysis**: Op CF / Net Income
- **Sales vs A/R**: Sales Growth, A/R Growth, CAGR

---

## 7. FSAP Data JSON Schema

> **Scope**: GREEN variables (direct input). GREY/WHITE calculated by `fsap_calculations`.
> **Data Source Restriction → Enforced at Backend API**: Only from Item 8 Financial Statements + Notes.

### Full Response Structure
```json
{
  "company": "Ford Motor Co",
  "cik": "0000037996",
  "fiscal_year": 2023,
  "fiscal_period": "FY",
  "source_document": "10-K",
  
  "balance_sheet": {
    "current_assets": {
      "cash_and_equivalents": {"value": 25165, "xbrl_tag": "CashAndCashEquivalentsAtCarryingValue"},
      "short_term_investments": {"value": 22789, "xbrl_tag": "ShortTermInvestments"},
      "accounts_receivable_net": {"value": 11992, "xbrl_tag": "AccountsReceivableNetCurrent"},
      "inventories": {"value": 16579, "xbrl_tag": "InventoryNet"},
      "finance_div_loans_leases_st": {"value": null, "xbrl_tag": null},
      "other_current_assets_1": {"value": 3500, "xbrl_tag": "PrepaidExpense", "label": "Prepaid Expenses"},
      "other_current_assets_2": {"value": 1200, "xbrl_tag": "OtherAssetsCurrent", "label": "Other Current Assets"}
    },
    "noncurrent_assets": {
      "ppe_at_cost": {"value": 65000, "xbrl_tag": "PropertyPlantAndEquipmentGross"},
      "accumulated_depreciation": {"value": -22314, "xbrl_tag": "AccumulatedDepreciationDepletionAndAmortization"},
      "intangibles": {"value": null, "xbrl_tag": null},
      "goodwill": {"value": 614, "xbrl_tag": "Goodwill"},
      "long_term_investments": {"value": null, "xbrl_tag": null},
      "deferred_tax_assets_lt": {"value": null, "xbrl_tag": null},
      "other_noncurrent_assets_1": {"value": 5000, "xbrl_tag": "EquityMethodInvestments", "label": "Equity Method Investments"},
      "other_noncurrent_assets_2": {"value": null, "xbrl_tag": null, "label": null}
    },
    "current_liabilities": {
      "accounts_payable": {"value": 24890, "xbrl_tag": "AccountsPayableCurrent"},
      "accrued_liabilities": {"value": 24587, "xbrl_tag": "AccruedLiabilitiesCurrent"},
      "notes_payable_short_term_debt": {"value": null, "xbrl_tag": null},
      "current_maturities_long_term_debt": {"value": null, "xbrl_tag": null},
      "other_current_liabilities_1": {"value": 2000, "xbrl_tag": "DeferredRevenueCurrent", "label": "Deferred Revenue"},
      "other_current_liabilities_2": {"value": null, "xbrl_tag": null, "label": null}
    },
    "noncurrent_liabilities": {
      "long_term_debt": {"value": 92455, "xbrl_tag": "LongTermDebtNoncurrent"},
      "deferred_tax_liabilities": {"value": null, "xbrl_tag": null},
      "other_noncurrent_liabilities_1": {"value": 8000, "xbrl_tag": "PensionAndOtherPostretirementBenefitPlans", "label": "Pension Obligations"},
      "other_noncurrent_liabilities_2": {"value": null, "xbrl_tag": null, "label": null}
    },
    "equity": {
      "common_stock_apic": {"value": 24102, "xbrl_tag": "CommonStockValue"},
      "retained_earnings_deficit": {"value": 27879, "xbrl_tag": "RetainedEarningsAccumulatedDeficit"},
      "treasury_stock": {"value": -2152, "xbrl_tag": "TreasuryStockValue"},
      "accum_other_comprehensive_income": {"value": null, "xbrl_tag": null},
      "noncontrolling_interests": {"value": null, "xbrl_tag": null}
    },
    
    "_reported_for_validation": {
      "_comment": "These totals are reported in SEC filings - used to verify our calculations",
      "total_current_assets": {"value": 114093, "xbrl_tag": "AssetsCurrent"},
      "total_assets": {"value": 273344, "xbrl_tag": "Assets"},
      "total_current_liabilities": {"value": 105999, "xbrl_tag": "LiabilitiesCurrent"},
      "total_liabilities": {"value": 228584, "xbrl_tag": "Liabilities"},
      "total_equity": {"value": 44760, "xbrl_tag": "StockholdersEquity"}
    }
  },
  
  "income_statement": {
    "revenues": {"value": 176191, "xbrl_tag": "Revenues"},
    "cost_of_goods_sold": {"value": -151443, "xbrl_tag": "CostOfGoodsSold"},
    "sga_expenses": {"value": -11442, "xbrl_tag": "SellingGeneralAndAdministrativeExpense"},
    "interest_expense": {"value": -1534, "xbrl_tag": "InterestExpense"},
    "income_tax_expense": {"value": -904, "xbrl_tag": "IncomeTaxExpenseBenefit"},
    
    "_reported_for_validation": {
      "_comment": "These totals are reported in SEC filings - used to verify our calculations",
      "gross_profit": {"value": 24748, "xbrl_tag": "GrossProfit"},
      "operating_income": {"value": 4343, "xbrl_tag": "OperatingIncomeLoss"},
      "income_before_tax": {"value": 5272, "xbrl_tag": "IncomeLossFromContinuingOperationsBeforeIncomeTaxes"},
      "net_income": {"value": 4347, "xbrl_tag": "NetIncomeLoss"}
    }
  },

  "cash_flow_statement": {
    "net_income_start": {"value": 4347, "xbrl_tag": "NetIncomeLoss"},
    "depreciation_amortization": {"value": 8000, "xbrl_tag": "DepreciationDepletionAndAmortization"},
    "stock_based_compensation": {"value": 500, "xbrl_tag": "ShareBasedCompensation"},
    "deferred_taxes": {"value": 200, "xbrl_tag": "DeferredIncomeTaxExpenseBenefit"},
    "changes_in_working_capital": {"value": -2000, "xbrl_tag": "IncreaseDecreaseInOperatingCapital"},
    "capex": {"value": -7000, "xbrl_tag": "PaymentsToAcquirePropertyPlantAndEquipment"},
    "dividends": {"value": -1500, "xbrl_tag": "PaymentsOfDividendsCommonStock"},
    "share_repurchases": {"value": -500, "xbrl_tag": "PaymentsForRepurchaseOfCommonStock"},
    "_reported_for_validation": {
      "net_cash_operating": {"value": 14000, "xbrl_tag": "NetCashProvidedByUsedInOperatingActivities"},
      "net_cash_investing": {"value": -8000, "xbrl_tag": "NetCashProvidedByUsedInInvestingActivities"},
      "net_cash_financing": {"value": -2000, "xbrl_tag": "NetCashProvidedByUsedInFinancingActivities"},
      "net_change_in_cash": {"value": 4000, "xbrl_tag": "CashAndCashEquivalentsPeriodIncreaseDecrease"}
    }
  },

  "supplemental_data": {
    "eps_basic": {"value": 1.08, "xbrl_tag": "EarningsPerShareBasic"},
    "eps_diluted": {"value": 1.07, "xbrl_tag": "EarningsPerShareDiluted"},
    "shares_outstanding_basic": {"value": 4000, "xbrl_tag": "WeightedAverageNumberOfSharesOutstandingBasic"},
    "shares_outstanding_diluted": {"value": 4100, "xbrl_tag": "WeightedAverageNumberOfSharesOutstandingDiluted"},
    "effective_tax_rate": {"value": 21.0, "xbrl_tag": "EffectiveIncomeTaxRateContinuingOperations"}
  },
  
  "reclassifications": [
    {
      "fsap_variable": "other_current_assets_1",
      "reclassification_type": "COMBINED",
      "source_tags": [
        {"tag": "PrepaidExpense", "value": 500},
        {"tag": "OtherAssetsCurrent", "value": 300}
      ],
      "value": 800,
      "note_evidence": {
        "note_number": "Note 10",
        "note_title": "Other Assets",
        "source_document": "10-K FY2023",
        "page_reference": "F-22",
        "quote": "Other current assets include prepaid expenses and...",
        "extracted_detail": {"prepaid": 500, "other": 300}
      },
      "reasoning": "Combined per Note 10 breakdown"
    }
  ],
  
  "metadata": {
    "llm_provider": "deepseek",
    "variables_mapped": 18,
    "variables_unmapped": 12,
    "processing_time_ms": 1500
  }
}
```

### Value Object Schema
```json
{
  "value": 25165,              // Value in millions USD, null if not found
  "xbrl_tag": "CashAndCash...", // Mapped XBRL tag (null if not found)
  "source_path": "Item 8 > Consolidated Balance Sheet",  // Data source path
  "label": "Prepaid Expenses", // Optional: Dynamic label for flexible slots
  "confidence": 0.95,          // Optional: LLM confidence (0-1)
  "mapping_type": "DIRECT"     // DIRECT | FALLBACK | RECLASSIFIED
}
```

### Source Path Values
| source_path | Description |
|:---|:---|
| `Item 8 > Consolidated Balance Sheet` | Balance Sheet line items |
| `Item 8 > Consolidated Income Statement` | Income Statement line items |
| `Item 8 > Consolidated Cash Flow Statement` | Cash Flow items |
| `Notes > Note X: [Title]` | Reclassification evidence from Notes |
```

### Reclassification Object Schema
```json
{
  "fsap_variable": "other_assets_1",
  "reclassification_type": "SPLIT | COMBINED | SIGN_ADJUSTED | PROXY",
  "source_tags": [{"tag": "...", "value": 100}],
  "value": 800,
  "note_evidence": {
    "note_number": "Note X",
    "note_title": "...",
    "source_document": "10-K FY2023",
    "page_reference": "F-XX",
    "quote": "...",
    "extracted_detail": {...}
  },
  "reasoning": "..."
}
```

### Note
- `_calculated` fields (totals, gross_profit, operating_income, etc.) → Computed by `fsap_calculations` skill
- `integrity_check` → Validated by `fsap_data_checks` skill



