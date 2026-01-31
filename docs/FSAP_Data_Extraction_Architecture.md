# FSAP Data Extraction Architecture Documentation

This document details the FSAP (Financial Statement Analysis and Projections) Engine's data extraction engineering, structural design, processing sequence, and the core design rationale behind them.

## 1. Architecture Overview

The core goal of the FSAP Engine is to convert unstructured SEC Edgar 10-K/10-Q filings into structured, computable, institutional-grade financial models.

The system follows the **"Deterministic Calculation, Agentic Reasoning"** principle:
- **LLM Agent**: Responsible for semantic mapping (identifying accounts), qualitative analysis (extracting growth rates, risks), and non-standard data processing.
- **Go Engine**: Responsible for all arithmetic operations, ratio calculations, and accounting balance verification ($Assets = Liabilities + Equity$).

## 2. Engineering Structure & Concurrency Model

The FSAP extraction pipeline adopts a **Parallel Multi-Agent Architecture**.

### Orchestration
The system uses Go's `sync.WaitGroup` to implement true concurrent execution, rather than serial processing. This means the total time to process a 100-page 10-K file depends on the **slowest** Agent, not the sum of all Agents' times.

The pipeline is divided into 5 parallel tracks:

| Track | Target Data | Corresponding SEC Section | Processing Engine |
| :--- | :--- | :--- | :--- |
| **Track A (Core)** | **Quantitative Financial Data** (BS/IS/CF) | Item 8 (Financial Statements) | **FEE Hybrid Engine** |
| **Track B** | **Strategy & Growth** (Strategy) | Item 7 (MD&A) | Strategy Agent |
| **Track C** | **Capital Allocation** (Capital) | Item 7 & Notes | Capital Agent |
| **Track D** | **Segment Normalization** (Segments) | Item 1 (Business) & Notes | Segment Agent |
| **Track E** | **Risk Radar** (Risks) | Item 1A, 3, 7A, 9A | Risk Agent |
| **Track F** | **Note Map Indexing** (Navigation) | Item 8 (Notes Header) | **Indexer Agent** |

## 3. Processing Sequence & Data Flow

Data follows a strict pipeline sequence from ingestion to delivery:

### Step 1: Full Markdownification
*   **Action**: Immediately convert the HTML file to high-fidelity GitHub Flavored Markdown (GFM) after download.
*   **Technical Details**: Special handling of HTML anchors (`<a id="foo">`) during conversion, transforming them into Markdown-visible markers `[ANCHOR:id]`.
*   **Rationale**:
    *   **Token Efficiency**: Markdown saves about 40% of tokens compared to HTML, significantly reducing LLM costs and increasing the effective context window.
    *   **Semantic Purity**: Removes HTML style noise, allowing Agents to focus on text content.

### Step 2: Task-Driven TOC Scout
*   **Action**: Use a lightweight LLM (TOC Scout) to scan the document's **Table of Contents**.
*   **Difference**: Instead of looking for literals like "Item 1", it identifies corresponding titles and anchor IDs based on tasks (e.g., "Find Risk Factors section").
*   **Rationale**: Resolves inconsistent naming across companies (e.g., some companies call it "Risk Factors", others "Principal Risks").

### Step 3: Anchor-First Slicing
*   **Action**: Go backend physically slices the large Markdown file based on `[ANCHOR:id]` returned by the Scout.
*   **Strategy**:
    1.  **Priority**: Look for `[ANCHOR:id]` markers to achieve 100% precision jumps.
    2.  **Fallback**: If the anchor fails, use "Skip First Match" title search strategy (the first match is usually the link in the TOC, the second is the body title).
*   **Rationale**: Avoids the **"TOC Echoes"** problem (where the Agent incorrectly extracts the summary from the TOC instead of the body content).

### Step 4: Parallel Extraction (Multi-Year)
*   All sliced segments are distributed simultaneously to the Agents of the 5 parallel tracks.
*   **Key Upgrade**: Agents are now instructed to extract **Comparative Columns** (e.g., extracting 2024, 2023, and 2022 data from a single 2024 10-K filing). This "Redundant Extraction" is crucial for the **Zipper Engine** to detect restatements.


### Step 5: Embedded Validation
*   **Action**: Each Agent performs a self-check before generating results.
*   **Mechanism**: The System Prompt includes instructions: "Check if the input text is actually the [Target Section]. If it is a TOC or irrelevant text, return an empty result."
*   **Rationale**: Prevents hallucinations. Without validation, LLMs might try to "fabricate" data even from incorrect text segments.

### Step 6: Hybrid Persistence (The Vault)
*   **Action**: Extracted JSON structures are persisted immediately after generation.
*   **Mechanism**: **Hybrid Vault** architecture.
    *   **Primary**: PostgreSQL (`fsap_extractions` table) for production queries and synthesis.
    *   **Fallback**: Local Filesystem (`.cache/edgar/...`) for offline development/debugging and redundancy.
*   **Result**: The "Atomic Extraction" phase ends here. The data is now immutable and ready for downstream consumers (like the Synthesis Engine).
*   **Rationale**: Ensures zero data loss and strict separation of concerns. The Extraction Agent does not know or care about historical data merging; its only job is to faithfully record what it sees in the current file.

## 4. Key Design Rationale

### Why "Markdown-First"?
HTML structure is too noisy for LLMs and prone to token overflow. Markdown is structurally clear (Headers, Lists), perfectly matching the LLM training data format, and supports reading long documents (up to 1M Context) in a single pass without RAG slicing, ensuring cross-paragraph context coherence.

### Why "Anchor-First" Navigation?
SEC files are typically huge (5MB+). Relying solely on text search for titles is extremely unreliable (body titles may differ slightly from TOC titles, or appear multiple times). HTML anchors are the only "physical addresses" in the file. By preserving anchors during Markdown conversion, we achieve machine-level positioning precision, which is the cornerstone of automated extraction reliability.

### Why force "Fail-Soft"?
Qualitative data (like Strategy Analysis) is "nice-to-have", while Quantitative data (Balance Sheet) is "must-have". The architecture ensures that even if the Risk Agent times out or fails for some reason, as long as Track A (FEE Engine) succeeds, the user still receives a usable valuation model.

### Why not use RAG (Vector DB)?
For deep understanding of a single long document like a 10-K, RAG's "slice-and-retrieve" mechanism breaks semantic coherence (e.g., a growth target mentioned by management on page 5 might depend on a footnote on page 50). The TIED platform leverages modern LLMs (like **DeepSeek V3**, **Gemini 1.5 Pro**, **Qwen-Long**) with ultra-long context capabilities to **directly load relevant sections (or the entire report) into Context**, implementing a deeper understanding capability than RAG.

### Why Decoupled Synthesis?
Traditional pipelines try to "extract and finalize" in one step. We split them. **Extraction** is an observation of a single document at a point in time (immutable). **Synthesis** is the construction of a financial reality across time (mutable). By decoupling them, we can handle **Restatements** (when a 2024 filing changes 2023 numbers) simply by re-running the synthesis logic, without needing to re-extract the old 2023 documents.
