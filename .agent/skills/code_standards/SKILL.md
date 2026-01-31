---
name: code_standards
description: Defines the "TIED" Code Frame and Vibe Coding Format Standards.
---

# Code Standards & Structure Skill

## 1. The Code Frame (Project Structure)
Maintain strict separation of concerns to support the "Hybrid Stack".

### A. Frontend (`/web-ui`)
- **`src/app`**: Pages and Layouts (Server Components default).
- **`src/components`**:
    - **`ingest/`**: "Green Zone" (PDF Upload, Mapping).
    - **`financial/`**: "White/Grey Zone" (Tables, Charts).
    - **`agent/`**: "Side-car" (Chat, SSE Stream).
- **`src/lib`**:
    - **`branding.ts`**: **MUST** be used for all colors (`TIED_BRANDING`). No Magic Hex Codes!
    - **`api.ts`**: Strongly typed fetch wrappers.

### B. Backend (`/pkg`) - The Go Engine
- **`core/`**: Pure business logic (Calculations).
- **`ingest/`**: Excel/PDF parsing logic.
- **`api/`**: HTTP Handlers (Gin/Fiber).
- **`storage/`**: Database repositories (Supabase/Postgres).

### C. Scripts (`/scripts`) - The Python Utilities
- Standalone Python scripts for heavy ML/OCR tasks. outputting JSON.

---

## 2. The Format Checker (Style Guidelines)

### A. General Principles (TIED)
1.  **Transparency**: Variable names must be verbose. `revenue_growth_yoy` > `r_g`.
2.  **Evidence**: Every major calculation function must ideally accept a `reasoning` or `source` context.
3.  **Integrity**:
    - **Go**: Handle EVERY error. No `_` ignores.
    - **TSX**: No `any`. Define interfaces for all Props.

### B. Frontend Rules (Vibe Coding)
- **Styling**: Use Tailwind Utility Classes.
- **Components**: Functional Components only.
- **State**: Use `useState` for local, but prefer URL state (searchParams) for shareable views.
- **Naming**: `PascalCase` for Components, `camelCase` for props.

### C. Database Rules (Supabase)
- **SQL Keywords**: `UPPERCASE` (SELECT, FROM).
- **Snake_Case**: Table and column names (`user_id`, not `userId`).
- **Comments**: Complex SQL queries MUST explain the *business logic*.

---

## 3. The "Self-Correction" Checklist
*Before writing detailed code, verify:*
- [ ] am I using `TIED_BRANDING` constants instead of raw colors?
- [ ] Is my Go error handling aggressive (fail-fast)?
- [ ] Did I create a Type/Interface before using the object?
- [ ] Is this file in the correct "Code Frame" directory?
