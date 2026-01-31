# TIED: Agentic Valuation Platform
**Transparent, Integrated, Evidence-Driven**

> **Mission**: To build an "Auditor-First" AI valuation platform where every number is tied to original evidence, every assumption is debated by agents, and every decision is transparent.

---

## Quick Start

1.  **Set Environment Variables**:
    Create a `.env` file at the project root:
    ```env
    DEEPSEEK_API_KEY=your_api_key_here
    ```

2.  **Run the Valuation Engine**:
    We have consolidated the end-to-end analysis into a single CLI tool.
    ```bash
    go run cmd/pipeline/main.go
    ```
    *Note: This runs a demo on cached Apple FY2024 data to avoid API costs during testing.*

3.  **Expected Output**:
    The engine will produce a console report including:
    *   **Core Financials**: Revenue, Net Income, Margins.
    *   **Penman Decomposition**: RNOA, NBC, ROCE Analysis.
    *   **Segment Analysis**: Revenue & Operating Income by Geography.
    *   **Validation Check**: Cross-check between Segment Sum and Consolidated Totals (implied Corporate Overhead).
    *   **Forensic Screening**: Benford's Law and Beneish M-Score risk flags.

---

## 🗺️ Documentation Map
New to the project? Start here.

### 1. Project Architecture
*   `pkg/core`: Core valuation logic (Extraction, Analysis, Forensic).
*   `cmd/pipeline`: Main entry point for the CLI.
*   `tests/`: Integration tests.
- **[Project Structure](.agent/resources/PROJECT_STRUCTURE.md)**: A complete map of directories, modules, and agent resources.
- **[Full Pipeline Verification](tests/e2e/full_pipeline_test.go)**: The "Gold Standard" integration test verifying the `Extraction -> Synthesis -> Debate -> Report` loop.
- **[Database Schema](.agent/resources/SCHEMA_DESIGN.md)**: The "Hybrid Vault" architecture (Postgres + JSONB + Vector).

### 2. Workflows & Standards
- **[Standard Workflows](docs/WORKFLOWS.md)**: How to implement features, update docs, and run tests.
- **Requirements Log**: `docs/REQUIREMENT_LOG.md` (Active requirements).
- **Decision Log**: `docs/DECISION_LOG.md` (Immutable architecture decisions).

### 3. Key Systems
- **[FSAP Engine](docs/FSAP_DOCUMENTATION.md)**: The Financial Statement Analysis Platform core.
- **[Multi-Agent Debate](docs/MULTI_AGENT_DEBATE_SYSTEM.md)**: The 5-agent assumption generation protocol.
- **[Prompt Library](resources/prompts/)**: Centralized prompt management with JSON-based templates.

### 4. TIED v2.0 Core Architecture
- **[Backend Packages](pkg/README.md)**: AI-native financial modeling ("Fixed Skeleton, Dynamic Flesh")
  - `pkg/core/projection` - Polymorphic Node System (Strategy Pattern)
  - `pkg/core/knowledge` - Unified Knowledge Layer (RAG support)
  - `pkg/core/assumption` - AssumptionSet backend (syncs with frontend)

---

## 🏗️ Core Stack
**"The Hybrid Stack"**
- **Backend**: Go (Strict Financial Logic, Concurrency)
- **Frontend**: Next.js 14+ (App Router, TypeScript)
- **Database**: Supabase (PostgreSQL, pgvector)
- **AI**: Multi-Provider (DeepSeek, Qwen) + Gemini Grounding
- **Prompt Library**: JSON-based templates in `resources/prompts/`

---
*For specific agent capabilities, check `.agent/skills/`.*

