# TIED Platform: Project Structure & Directory Guide

This document provides a comprehensive map of the TIED (Transparent, Integrated, Evidence-Driven) Agentic Valuation Platform codebase.

## 📂 Root Directory
| Directory | Purpose |
| :--- | :--- |
| **`.agent/`** | **Agent Brain**: Contains AI-specific context, skills, and resources. |
| **`cmd/`** | **Entry Points**: Main executables and CLI utilities. |
| **`config/`** | **Configuration**: Environment variables and app config files. |
| **`docs/`** | **Documentation**: Human-readable project documentation. |
| **`pkg/`** | **Backend Core**: The main Go application logic (Business Logic Layer). |
| **`resources/`** | **Prompt Library**: JSON-based LLM prompts and response schemas. |
| **`web-ui/`** | **Frontend**: The Next.js / TypeScript user interface. |
| `api.exe` | Compiled backend binary (Windows). |
| `start_all.bat` | One-click startup script for the entire platform. |

---

## 📝 Prompt Library (`resources/`)
Centralized management for all LLM prompts. Loaded at runtime by `pkg/core/prompt`.

| Directory | Purpose |
| :--- | :--- |
| **`prompts/debate/`** | Debate agent system prompts (macro, sentiment, fundamental, skeptic, optimist, synthesizer). |
| **`prompts/extraction/`** | Financial statement extraction prompts (balance_sheet, income_statement, cash_flow, etc.). |
| **`prompts/qualitative/`** | Qualitative analysis agent prompts (strategy, capital_allocation, segment, risk). |
| **`prompts/assistant/`** | AI assistant prompts (navigation intent parser). |
| **`schemas/`** | JSON response schemas for LLM output validation. |

---

## 🧠 Agent Resources (`.agent/`)
This folder bridges human intent and AI execution.

| Directory | Purpose |
| :--- | :--- |
| **`resources/`** | **Context Store**: High-level architectural docs, schema designs, and project structure guides (like this one) for the agent to reference. |
| **`skills/`** | **Capabilities**: Executable skill definitions (e.g., `fsap_data`, `readme_maintainer`) that extend the agent's ability to interact with the codebase. |
| **`workflows/`** | **Standard Procedures**: Step-by-step guides for common tasks (e.g., "Deploy to Production", "New Cycle Start"). |

---

## 🚀 Entry Points (`cmd/`)
Go applications start here. Each subdirectory usually contains a `main.go`.

| Directory | Purpose |
| :--- | :--- |
| **`api/`** | **Main Server**: The primary HTTP API server entry point. |
| **`pipeline/`** | **Valuation CLI**: Main CLI for End-to-End Extraction, Segmentation, and Analysis. |
| `validate_mapping/` | **Utility**: Tool to valid FSAP mapping logic. |
| `test_edgar.../` | **Tests**: Standalone runners for testing specific modules (Edgar, TOC, Submissions). |

---

## ⚙️ Backend Core (`pkg/`)
The heart of the application, written in Go.

### `pkg/api/` (Interface Layer)
HTTP handlers and routing logic.
- **`edgar/`**: Endpoints for SEC filing retrieval and processing status.
- **`assistant/`**: AI navigation assistant endpoints.
- **`debate/`**: Multi-agent debate streaming endpoints.

### `pkg/core/` (Business Logic Layer)
| Directory | Purpose |
| :--- | :--- |
| **`agent/`** | **Orchestration**: Logic for managing AI agent lifecycles and tasks. |
| **`calc/`** | **Calculation Engine**: Financial math, ratio calculations, and valuation model logic (FSAP engine). |
| **`debate/`** | **Multi-Agent Debate**: Debate orchestration, agent definitions, and consensus building. |
| **`edgar/`** | **SEC Integration**: Clients for SEC EDGAR, filing retrieval, and parsing. Includes `segment_agent.go`, `qualitative_agents.go`. |
| **`fee/`** | **Financial Extraction Engine**: Advanced logic for parsing 10-K HTML, locating tables, and extracting data. |
| **`ingest/`** | **Data Pipeline**: Workflows for ingesting raw data into the system. |
| **`llm/`** | **LLM Providers**: Provider implementations for LLMs (DeepSeek, Qwen, Gemini, etc.). |
| **`prompt/`** | **Prompt Library**: Runtime prompt loading, registry, and template rendering. |
| **`utils/`** | **Shared Utilities**: Logging, error handling, JSON helpers, and common tools. |

### `pkg/models/`
Data structures and database models (structs) shared across the application.

---

## 🖥️ Frontend (`web-ui/`)
A Next.js 14+ application using App Router and TypeScript.

| Directory | Purpose |
| :--- | :--- |
| **`src/app/`** | **Pages & Routing**: App Router structure. `page.tsx` files define routes. |
| **`src/components/`** | **UI Library**: Reusable React components (Buttons, Panels, Graphs). |
| **`src/lib/`** | **Client Utilities**: Helper functions, API clients, and hooks for the frontend. |
| `src/types/` | **TypesOps Definitions**: Shared interfaces for frontend state. |
| `public/` | **Static Assets**: Images, fonts, and icons. |

---

## 📚 Documentation (`docs/`)
Key documentation files for developers and stakeholders.

| File | Purpose |
| :--- | :--- |
| `MULTI_AGENT_DEBATE_SYSTEM.md` | Architecture for the multi-agent assumption debate feature. |
| `DECISION_LOG.md` | **Governance**: Record of architectural decisions and their rationale. |
| `REQUIREMENT_LOG.md` | **Governance**: Tracking of user requirements and feature requests. |
| `FSAP_DOCUMENTATION.md` | Documentation for the Financial Statement Analysis Platform engine. |
| `PRD_ZH.md` | Product Requirement Document (Chinese). |

