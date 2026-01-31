# FSAP Projection Engine V2 Upgrade Plan
**Target**: Align `StandardSkeleton` and `ProjectionEngine` with institutional-grade financial modeling standards (based on Ford Motor Co. reference).

## Phase 1: Skeleton Expansion (The "Wall Street" Standard)
**Goal**: Expand the L1 Fixed Nodes to capture the granularity required for complex valuations, ensuring no line item is "orphaned".

### 1. Balance Sheet Expansion
We will refactor `StandardSkeleton` in `skeleton.go` to include these discrete nodes:

| Section | New Nodes to Add | Rationale (from Reference) |
| :--- | :--- | :--- |
| **Current Assets** | `short_term_investments`<br>`other_current_assets` | Distinguish cash from restricted cash/investments.<br>Catch-all for prepaids/receivables. |
| **Non-Current Assets** | `goodwill_and_intangibles`<br>`long_term_investments`<br>`deferred_tax_assets`<br>`other_non_current_assets` | **Critical**: Goodwill is often massive.<br>DTAs need specific modeling (or % of Tax). |
| **Current Liabs** | `accrued_liabilities`<br>`other_current_liabilities` | Separate "Trade AP" (DPO driven) from "Accrued" (Sales driven). |
| **Non-Current Liabs** | `deferred_tax_liabilities`<br>`other_non_current_liabilities` | DTLs are a major non-cash funding source. |
| **Equity** | `preferred_stock`<br>`minority_interest`<br>`aoci`<br>`treasury_stock` | **Critical**: Treasury Stock reflects buybacks.<br>Minority Interest affects valuation bridge. |

### 2. Income Statement Expansion
| Section | New Nodes to Add | Rationale |
| :--- | :--- | :--- |
| **OpEx Breakdown** | `selling_marketing`<br>`general_admin`<br>`other_operating_expense` | Replace generic `SGA` with `SM` and `GA` for finer benchmarking. |
| **Non-Op** | `other_non_operating_income`<br>`minority_interest_expense` | Capture FX gains/losses and minority attribution. |

---

## Phase 2: Engine Logic Refinement (The "Brain" Upgrade)
**Goal**: Make the `ProjectYear` function smarter about capital allocation and roll-forwards.

### 1. Flexible Financial Account (Capital Allocation)
We will upgrade the "Plug" logic to a **Waterfall Priority System**:
1.  **Excess Cash Priority**:
    *   Step 1: Pay down Revolver (if any).
    *   Step 2: Pay Scheduled Debt (if modeling prepayments).
    *   Step 3: **Share Buybacks** (flows into `treasury_stock`).
    *   Step 4: Accumulate Cash.
2.  **Deficit Funding Priority**:
    *   Step 1: Draw on Revolver.
    *   Step 2: Issue Debt/Equity (Manual override).

### 2. Segment Forecast Integration
Using the newly implemented `SumStrategy`, we will enable:
*   **Revenue Configuration**: `Parent: Revenue` (SumStrategy) <- `Child: Ford Blue`, `Child: Ford Pro`.
*   **Validation**: Ensure `Engine` automatically calculates Parent = Sum(Children) *before* running the income statement flow.

---

## Phase 3: Immediate Execution Steps
1.  **Modify `skeleton.go`**: Add all definition lines for the new nodes listed in Phase 1.
2.  **Update `engine.go`**:
    *   Map new nodes to the `BalanceSheet` and `IncomeStatement` structs.
    *   Ensure the `CalculateBalanceSheetTotals` helper includes these new buckets.
3.  **Test**: Run a generated test case populated with dummy data for these new fields to verify the balance sheet still balances.

---
**Decision**:
Do you want me to proceed with **Step 1 (Modify skeleton.go)** immediately?
