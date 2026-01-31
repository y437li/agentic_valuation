# TIED: Frontend Design & Architecture

This document outlines the visual design system and component architecture for the **TIED** platform, ensuring a premium, trusted financial experience.

## 1. Design Philosophy: "Vibe Coding" Premium
- **Aesthetic**: Dark mode by default, high-contrast financial data. "Glassmorphism" for depth.
- **Typography**: `Inter` for UI, `Consolas` for Financial Tables.
- **Interaction**: Instant feedback via Optimistic UI.

---

## 2. The TIED Design System (Token DNA)

### Color Palette
| Token | Hex | Usage |
| :--- | :--- | :--- |
| **`fsap.input`** | `#059669` (Green) | Validated User Edits / Hardcodes. |
| **`fsap.sum`** | `#1e293b` (Dark) | Calculated Aggregations (Non-editable). |
| **`fsap.calc`** | `#475569` (Slate) | Derived values (formulas). |
| **`fsap.error`** | `#dc2626` (Red) | Balance Sheet Imbalance (!). |
| **`accent`** | `#2563eb` (Blue) | Primary Actions (Save, Run Model). |

### Typography
- **Numbers**: Monospaced (`Consolas`), Right-Aligned.
- **Labels**: Uppercase, Tracking-Wider (`0.05em`), Muted Color.

---

## 3. Component Architecture

### A. The Ingestion Engine (Green Zone)
*   **Split View**: PDF Viewer (Left) <-> Mapping Table (Right).
*   **Drag & Drop**: Framer Motion animations for file upload.
*   **Diff UI**: Highlights "What changed vs last version".

### B. The Financial Grid (The "Football Field")
*   **Structure**: Pivot-table style grid.
*   **Interaction**: Click cell -> Slide-out "Audit Trail" panel.
*   **Visualization**: Sparklines embedded in table rows for 3Y Trends.

### C. Agentic Side-Car
*   **Stream**: Server-Sent Events (SSE) stream agent thoughts ("Reading page 12...").
*   **Controls**: "Approve/Reject" buttons for agent suggestions.

---

## 4. Tech Stack Implementation
- **Framework**: Next.js 15 (App Router).
- **Styling**: Tailwind CSS v4.
- **State**: React Query (Server) + Zustand (Client).
- **Charts**: Recharts (Composed Charts for Valuation Football Field).
