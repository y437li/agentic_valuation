
# AI Agent Registry (Tiered Architecture)

> This registry documents the official hierarchy of agents within the TIED platform.
> Status: **Proposed / Partially Implemented**

## Layer 1: The Workers (Atomic Extraction)
*Responsible for interfacing with raw unstructured data (HTML/PDF) and converting it to structured JSON.*

| Agent ID | Name | Role | Go Package | Status |
| :--- | :--- | :--- | :--- | :--- |
| **A1** | **Filing Fetcher** | Downloads/Updates SEC filings; handles cache. | `pkg/core/edgar` | ✅ Active |
| **A2** | **Tabular Extractor** | Extracts BS/IS/CF tables using vision/text models. | `pkg/core/edgar` | ✅ Active |
| **A3** | **Segment Miner** | Extracts "Note 25" Segment Reporting (Rev/OpInc by Geo). | `pkg/core/segment` | 🚧 Planned (Phase 3) |
| **A4** | **Narrative Parser** | Extracts MD&A, Risk Factors, and Footnotes text blocks. | `pkg/core/text` | 🚧 Planned (Phase 3) |

## Layer 2: The Analysts (Synthesis & Computation)
*Responsible for cleaning, merging, and deriving insights from Worker output. No finding new data, only analyzing existing.*

| Agent ID | Name | Role | Go Package | Status |
| :--- | :--- | :--- | :--- | :--- |
| **B1** | **The Accountant** | **(Core)** Runs the "Rolling Merger" logic; handles Restatements; enforces Accounting Identities. | `pkg/core/synthesis` | 🚧 **In Progress** (Phase 1b) |
| **B2** | **Ratio Analyst** | Calculates RNOA, ROCE, Margins, Growth Rates. | `pkg/core/calc` | ✅ Active (Partial) |
| **B3** | **Forensic Detective**| Runs Benford's Law, Beneish M-Score; flags anomalies. | `pkg/core/forensics` | 🚧 Planned (Phase 2) |
| **B4** | **Time Traveler** | Analyzes MD&A narrative shifts over time (Sentiment/Keywords). | `pkg/core/text` | 🚧 Planned (Phase 3) |

## Layer 2.5: The Scouts (Research & Grounding)
*Responsible for fetching real-time external data to ground the debate.*

| Agent ID | Name | Role | Provider | Status |
| :--- | :--- | :--- | :--- | :--- |
| **R1** | **Macro Researcher** | Fetches GDP, rates, commodities via Google Search. | Gemini (Required) | ✅ Active |
| **R2** | **Sentiment Researcher** | Fetches news, analyst notes via Google Search. | Gemini (Required) | ✅ Active |
| **R3** | **Fundamental Researcher** | Analyzes segments & competition via Google Search. | Gemini (Required) | ✅ Active |

## Layer 3: The Committee (Reasoning & Valuation)
*Responsible for high-level judgment, debating assumptions, and producing the final investment thesis.*

| Agent ID | Name | Role | System Prompt | Status |
| :--- | :--- | :--- | :--- | :--- |
| **C1** | **The Bull** | Optimistic Projections; Defends growth assumptions. | `prompts/debate/bull.json` | 🚧 Prototype |
| **C2** | **The Bear** | Pessimistic Projections; Attacks weak moats/margins. | `prompts/debate/bear.json` | 🚧 Prototype |
| **C3** | **The Synthesizer** | Arbitrates debate; Sets Final Valuation Range; Writes Memo. | `prompts/debate/synthesizer.json` | 🚧 Prototype |

---

## Agent Communication Protocol
- **L1 -> L2**: JSON File Handover (Async). Workers dump to DB; Analysts read from DB.
- **L2 -> L3**: "Context Injection". Analysts output a structured `FinancialContext` object (containing Golden Data + Red Flags + Narrative Shifts) which is injected into the System Prompt of the L3 agents.
- **L3 <-> L3**: **Round Table Debate**. Real-time chat loop (max 3 turns) arbitrated by the Synthesizer.

## Current Implementation Priorities
1.  **Refine A2 (Extractor)**: Improve multi-year column recognition.
2.  **Build B1 (Accountant)**: Implement the "Rolling Merger" to create the Golden Data Series.
3.  **Deploy C-Suite**: Connect the debate loop to the Golden Data.
