# System Module Map & Architecture Reference

This document maps the conceptual stages of the **Agentic Valuation** pipeline to the specific Go packages and entry points in the codebase.

## 1. High-Level Directory Structure

### `cmd/` (Entry Points)
Executable binaries and runners for the system.
- **`cmd/pipeline/`**: The standard production entry point.
- **`cmd/pipeline_demo/`**: The end-to-end demonstration runner (CLI) that visualizes the entire flow.
- **`cmd/api/`**: (Future) REST/gRPC API server for frontend integration.
- **`cmd/tools/`**: Helper utilities (e.g., `batch_extract` for bulk data processing).

### `pkg/core/` (Core Logic)
The heart of the application, organized by domain.

---

## 2. Module Mapping by Pipeline Stage

### Stage 1: Data Ingestion & Extraction (The "Senses")
*Responsibility: Fetching raw data (10-K, Transcripts) and structuring it.*

| Conceptual Component | Go Package | Description |
| :--- | :--- | :--- |
| **SEC Extraction** | `pkg/core/edgar` | `integration_all_companies_test.go` defines the golden standard:<br>1. **Navigator Agent** (`navigator_agent.go`): Finds TOC and Statement Links.<br>2. **Table Mapper** (`table_mapper_agent.go`): Identifies columns (Current vs Prior Year).<br>3. **Parallel Extract** (`statement_agents.go`): Runs the extraction. |
| **Segment Extraction** | `pkg/core/edgar` | `segment_agent.go`: Specialized agent used in integration tests to parse "Note 25" (Segments) for Sum-of-the-Parts. |
| **Data Types** | `pkg/core/edgar/types.go` | Defines the standard `FSAPDataResponse` (IncomeStatement, BalanceSheet, CashFlow). |
| **Ingestion** | `pkg/core/ingest` | Utilities for loading local files or streaming data into the system. |

### Stage 2: Data Aggregation & Logic (The "Zipper")
*Responsibility: Cleaning, aligning, and validating historical data.*

| Conceptual Component | Go Package | Description |
| :--- | :--- | :--- |
| **The Zipper** | `pkg/core/synthesis` | `zipper.go`: Implements the "Golden Record" logic. Stitches multi-year filings (10-K/A > 10-K) into a unified timeline. |
| **Validation** | `pkg/core/calc` | `aggregation.go`: Checks accounting identities (Assets = L+E) and computes calculated totals per year. |
| **Analysis** | `pkg/core/calc` | `analysis.go`: Calculates margins, growth rates, and RNOA (Penman decomposition). |

### Stage 3: Cognitive Roundtable (The "Brain")
*Responsibility: Multi-agent debate, qualitative research, and assumption generation.*

| Conceptual Component | Go Package | Description |
| :--- | :--- | :--- |
| **Orchestrator** | `pkg/core/debate` | `orchestrator.go`: Manages the **Challenge Loop** (Research -> Debate -> Synthesis). Coordinates Skeptic vs. Optimist rounds. |
| **Material Pool** | `pkg/core/debate` | `material_pool.go`: The "Shared Memory". Thread-safe container for all "Facts" (Financials, Transcripts, Research) that Agents must cite. |
| **System Prompts** | `pkg/core/debate` | `prompts.go`: Defines the strict **Persona Guidelines** and the *Synthesis Output Schema* (Markdown Table) required by the Adapter. |
| **Agents** | `pkg/core/debate` | `agents.go`: Implements the distinct behaviors for Skeptic (Bear Case), Optimist (Bull Case), and Synthesizer (CIO). |

### Stage 4: Quantitative Projection (The "Builder")
*Responsibility: Translating qualitative assumptions into 3-statement models.*

| Conceptual Component | Go Package | Description |
| :--- | :--- | :--- |
| **The Adapter** | `pkg/core/projection` | `adapter.go`: The "Bridge". Parses strict Markdown tables from Debate into the `ProjectionAssumptions` Go struct for the engine. |
| **Projection Engine** | `pkg/core/projection` | `engine.go`: The recursive core. Projects IS -> BS -> CF. Handles **Plug Logic** (Debt Paydown vs. Cash Accumulation) to balance the Balance Sheet. |
| **Standard Skeleton** | `pkg/core/projection` | `skeleton.go`: Defines the canonical accounting structure (Line Items) for the projected future state. |

### Stage 5: Valuation & Dynamic Rates (The "Judge")
*Responsibility: Discounting cash flows and determining fair value.*

| Conceptual Component | Go Package | Description |
| :--- | :--- | :--- |
| **Master Runner** | `pkg/core/valuation` | `summary.go`: Orchestrates the 5-Model Suite. Collects outputs from DCF, DDM, RIM, etc. |
| **Dynamic WACC** | `pkg/core/valuation` | `wacc_series.go`: **Iterative Feedback Loop**. Scans the *projected* Balance Sheet to re-calculate WACC (via Hamada Equation) for each forecast year based on changing D/E ratios. |
| **DCF Engine** | `pkg/core/valuation` | `dcf.go`: Implements FCFF (Enterprise Value) and FCFE (Equity Value) using cumulative discount factors. |
| **Equity Models** | `pkg/core/valuation` | `equity_models.go`: Implements Dividend Discount (DDM) and Residual Income (RIM) models for cross-verification. |

### Infrastructure & Utils

| Component | Go Package | Description |
| :--- | :--- | :--- |
| **AI Providers** | `pkg/core/llm` | Wrappers for DeepSeek, Gemini, etc. |
| **Persistence** | `pkg/core/store` | (Optional) Database interfaces for saving debate results. |
| **Utils** | `pkg/core/utils` | Shared JSON validators and helper functions. |

---

## 3. Key Workflows (How code connects)

### End-to-End Pipeline (`tests/e2e/full_pipeline_test.go`)
This is the **Gold Standard** verification suite. It proves the "Integration Capability" of the system by chaining:
1.  **Extraction**: Extracting raw data from 20+ diverse companies (Apple, JPM, Exxon, etc.) using `edgar` agents.
2.  **Synthesis**: Feeding these extractions into the `synthesis.ZipperEngine` to produce a validated `GoldenRecord`.

### Demo Runner (`cmd/pipeline_demo/main.go`)
A lightweight CLI visualizer that simulates the flow for a single company (Apple) to demonstrate:
1.  **Load**: `edgar` + `calc` (Get History)
2.  **Think**: `debate` (Generate Assumptions)
3.  **Build**: `projection` (Generate Future 3-Statements)
4.  **Rate**: `valuation` (Calculate WACC Curve)
5.  **Value**: `valuation` (Run DCF/DDM/RIM)

### Assumption Flow
1. **Debate** outputs Markdown Table.
2. **Adapter** reads Markdown -> `ProjectionAssumptions` (Go Struct).
3. **Engine** reads `ProjectionAssumptions` + `Historical Data` -> `ProjectedFinancials` (Go Struct).
