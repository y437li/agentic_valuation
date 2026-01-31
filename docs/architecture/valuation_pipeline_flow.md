# End-to-End Valuation Pipeline Architecture

```mermaid
graph TD
    subgraph Data_Layer [Step 1: Data Ingestion & Grounding]
        A[SEC EDGAR 10-K/10-Q] -->|Extract| B(Material Pool)
        C[Earnings Transcripts] -->|Parse| B
        D[Market Data (Rf, ERP)] -->|Feed| B
    end

    subgraph Cognitive_Layer [Step 2: Roundtable Debate]
        B --> E[Orchestrator FSM]
        E -->|Phase 1| F[Research Agents]
        F -->|Macro, Fundamental, Sentiment| G[Initial Thesis]
        
        G -->|Phase 2| H[Challenge Loop]
        H -->|Critique| I[Skeptic Agent]
        H -->|Defend| J[Optimist Agent]
        
        I & J -->|Phase 3| K[Synthesizer (CIO)]
        K -->|Decide| L[Final Debate Report]
        L -- Contains --> M[Markdown Table Layout]
    end

    subgraph Bridge_Layer [Step 3: Structural Adapter]
        M -->|Parse text| N[Adapter Module]
        N -->|Map to Struct| O[ProjectionAssumptions]
        O -- Includes --> P[Drivers: Growth, Margins]
        O -- Includes --> Q[Rates: Beta, Rf, CostDebt]
    end

    subgraph Quantitative_Layer [Step 4: Projection Engine]
        R[Standard Skeleton] --> S[Projection Engine]
        O --> S
        S -->|Articulate| T[Projected Financials]
        T -- Contains --> U[Income Statement]
        T -- Contains --> V[Balance Sheet]
        T -- Contains --> W[Cash Flow]
    end

    subgraph Dynamic_Rates [Step 5: Dynamic WACC Loop]
        V -->|Extract Debt & Equity| X[Capital Structure Audit]
        X -->|Combine with (Q)| Y[GenerateDynamicWACCSeries]
        Y -->|Output| Z[Period-Specific WACC Array]
    end

    subgraph Valuation_Layer [Step 6: Master Valuation Suite]
        T & Z --> AA[Standard DCF (FCFF)]
        T & Q --> AB[Dividend Discount (DDM)]
        T & Q --> AC[Residual Income (RIM)]
        T & Q --> AD[Adjusted Present Value (APV)]
        T & Q --> AE[Relative Valuation (Comps)]
        
        AA & AB & AC & AD & AE --> AF[Summary Output]
        AF -->|Final Price| AG((Target Price $))
    end

    style K fill:#f9f,stroke:#333,stroke-width:2px
    style S fill:#ccf,stroke:#333,stroke-width:2px
    style AF fill:#cfc,stroke:#333,stroke-width:2px
```

## System Component Breakdown

### 1. Data Layer (Grounding)
*   **Material Pool**: The single source of truth. Prevents hallucinations by forcing agents to cite specific lines from 10-Ks or transcripts.

### 2. Cognitive Layer (The "Roundtable")
*   **Orchestrator**: Manages the state machine (Research -> Debate -> Synthesis).
*   **Agents**: 
    *   *Research*: Gather raw data.
    *   *Skeptic/Optimist*: Stress-test assumptions (e.g., "Margins are too high given inflation").
    *   *Synthesizer*: The decision maker. It forces the qualitative debate into a **Quantitative Markdown Table**.

### 3. Bridge Layer (The Adapter)
*   **Adapter Module**: Regex-based parser that reads the debate output and fills the Go `ProjectionAssumptions` struct. It handles unit conversions (% vs decimal) and fallback values.

### 4. Quantitative Layer (The Engine)
*   **Projection Engine**: The "Calculator". It takes the assumptions and the `StandardSkeleton` (accounting logic) to produce mathematically balanced financial statements (`ProjectedFinancials`) for N years.
*   **Key Feature**: Assets = Liabilities + Equity is guaranteed here via the Plug (Cash/Revolver).

### 5. Dynamic Rates (The Feedback Loop)
*   **Capital Structure Audit**: Looks at the *Projected* Balance Sheet for each future year.
*   **WACC Series**: Re-calculates WACC for Year 1, Year 2, etc., based on the changing Debt/Equity ratio found in the projection.

### 6. Valuation Layer (The Models)
*   **Multi-Model Suite**: logical convergence check using 5 distinct methodologies.
    *   *DCF*: WACC-based.
    *   *DDM*: Dividend-based.
    *   *RIM*: Book Value + Excess Return.
    *   *FCFE*: Direct equity cash flow.
*   **Summary**: Aggregates all share prices into a final consensus view.
