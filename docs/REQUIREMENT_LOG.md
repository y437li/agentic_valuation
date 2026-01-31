# TIED Project Requirement Log

This document tracks all new, modified, or deleted requirements for the TIED Agentic Valuation Platform.

| ID | Date | Type | Requirement Description | Origin | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **REQ-001** | 2026-01-17 | **New** | **Explainable Excel Import**: System must provide "Agent Reasoning" and "Original Cell" coordinates for every auto-mapped field. | User Chat | `Implemented` |
| **REQ-002** | 2026-01-17 | **New** | **3-Month Vibe Coding Plan**: Project roadmap must be compressed into a 3-month timeline (Core -> Product -> Integration). | User Chat | `Active` |
| **REQ-003** | 2026-01-17 | **Modify** | **"Modeling" over "DCF"**: Rename "DCF Model" to generic "Modeling" and support dynamic methodology switching (LBO, Perp Growth). | User Chat | `Implemented` |
| **REQ-004** | 2026-01-17 | **New** | **Hybrid Database**: Use Supabase (PostgreSQL) with JSONB for lineage and Vector for evidence search. | User Chat | `Active` |
| **REQ-005** | 2026-01-17 | **New** | **Filing Hierarchy Logic**: Historic data (pre-CurrentYear) uses 10K. Current Year uses 10Q/8K for *Assumptions Updates*. 8K is primarily for unstructured signals. | User Chat | `Active` |
| **REQ-006** | 2026-01-18 | **New** | **JSON Repair & Validation**: System must auto-repair malformed JSON from LLM outputs (missing quotes, trailing commas, unclosed brackets). Use language-native libraries: hjson-go + json-repair (Go), jsonrepair (TypeScript). No Python dependencies. | User Chat | `Implemented` |
| **REQ-007** | 2026-01-18 | **New** | **Frontend Model Selector**: Enable dynamic switching between LLM providers (DeepSeek, Qwen) directly from the Sidebar/Settings UI. Selected provider must persist for the session via Backend API. | User Chat | `Implemented` |
| **REQ-008** | 2026-01-18 | **New** | **Multi-Scenario Comparison**: Support Base, Bull, and Bear scenarios within a single valuation view. No model duplication. | User Chat | `Implemented` |
| **REQ-009** | 2026-01-18 | **New** | **Scenario Parameter Delegation**: Bull/Bear scenarios must inherit historical data and unchanged parameters from the Base scenario automatically. | User Chat | `Implemented` |
| **REQ-010** | 2026-01-18 | **New** | **Case Discovery Workflow**: Prompt users when creating a case for a company that already exists. Suggest reuse or archiving old context. | User Chat | `Implemented` |
| **REQ-011** | 2026-01-19 | **New** | **Structured Implementation Workflow**: All new function implementations must follow a 3-step workflow: 1. Read Project Structure, 2. Analyze Impact/Files, 3. Implement. This must include documentation sync. | User Chat | `Implemented` |
| **REQ-012** | 2026-01-19 | **Modify** | **Switch to Gemini Grounding**: Replace Brave Search with Gemini Grounding (Google Search Tool) for the Debate System to reduce costs (Free Tier) and simplify integration. | User Chat | `Active` |
| **REQ-013** | 2026-01-20 | **New** | **Consolidated Financial Analysis Tabs**: Integrate Ratio Analysis, DuPont Analysis, and Growth Rates into FinancialStatements component as separate tabs. Sidebar links serve as quick navigation shortcuts. | User Chat | `Implemented` |
| **REQ-014** | 2026-01-20 | **New** | **Beneish M-Score Tab**: Add fraud detection analysis (8 M-Score variables) as a tab within Financial Statements. Display risk level with color-coded indicators. | User Chat | `Implemented` |
| **REQ-015** | 2026-01-20 | **New** | **Altman Z-Score Tab**: Add bankruptcy prediction analysis (5 Z-Score components) as a tab within Financial Statements. Display zone classification (Distress/Grey/Safe). | User Chat | `Implemented` |
| **REQ-016** | 2026-01-20 | **Modify** | **Minimalist UI Style**: Remove flashy gradients and colorful cards from AnalysisPanel and ForecastPanel Details. Use professional neutral colors and clean borders. | User Chat | `Implemented` |
| **REQ-017** | 2026-01-20 | **New** | **Embedded Analysis Mode**: AnalysisPanel supports `embedded` prop to hide redundant headers when used inside FinancialStatements tabs. | User Chat | `Implemented` |
| **REQ-018** | 2026-01-20 | **New** | **Assumption Graph Visualization**: Strategic Assumption Hub must have Tree/Graph toggle. Graph view uses React Flow to display parent-child relationships between assumptions. | User Chat | `Implemented` |
| **REQ-019** | 2026-01-20 | **New** | **3-Layer Dynamic Assumptions**: Level 1 = fixed templates, Level 2/3 = Agent-generated based on 10-K segment structure. Specific values determined by Roundtable Debate. | User Chat | `Active` |
| **REQ-020** | 2026-01-20 | **New** | **Roundtable-Driven Assumption Values**: Synthesizer outputs unified JSON with both memorandum and assumptions[]. This is Single Source of Truth for frontend and calculation engine. | User Chat | `Active` |
| **REQ-021** | 2026-01-20 | **New** | **Roundtable Trigger Control**: New Case = mandatory Roundtable with progress display + Skip. Existing Case = manual [Refresh] trigger only. No auto-override of user data. | User Chat | `Active` |
| **REQ-022** | 2026-01-20 | **New** | **Assumption Diff Review**: When Roundtable suggests changes to existing Case, show Diff Modal. User can Accept/Reject each change individually. | User Chat | `Active` |
| **REQ-023** | 2026-01-20 | **New** | **Assumption Apply/Remove Toggle**: Users can toggle assumptions on/off without deleting. Only applied assumptions affect Forecast calculations. Supports non-destructive scenario comparison. | User Chat | `Active` |
