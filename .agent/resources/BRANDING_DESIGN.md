# TIED: Brand & Visual Design System

This document is the "Visual Bible" for the TIED platform. It defines the exact color codes, typography, and component styles that define our verified, premium financial aesthetic.

## 1. Brand Core Identity
- **Name**: TIED (Transparent, Integrated, Evidence-Driven)
- **Vibe**: "Bloomberg Terminal meets Linear". High density, dark mode preference, neon accents for "Evidence".
- **Philosophy**:
    - **Green** is for Input (Growth).
    - **Blue** is for Action (Evidence).
    - **Grey** is for Logic (Calculations).

---

## 2. Color Palette (The "Semantics")

### A. Financial Data Colors (Crucial for FSAP)
These colors convey the *nature* of the number.

| Token | Hex | Tailwind | Usage |
| :--- | :--- | :--- | :--- |
| **`fsap.input`** | `#059669` | `text-emerald-600` | **Hardcodes**. User-editable values (e.g., Growth Rate). |
| **`fsap.sum`** | `#1e293b` | `text-slate-800` | **Aggregations**. Total Assets, EBITDA. |
| **`fsap.calc`** | `#475569` | `text-slate-600` | **Formulas**. Derived ratios (Margins, ROE). |
| **`fsap.error`** | `#dc2626` | `text-red-600` | **Imbalance**. Assets != L+E. |

### B. UI Foundations
| Token | Hex | Tailwind | Usage |
| :--- | :--- | :--- | :--- |
| **`bg.primary`** | `#ffffff` | `bg-white` | Main panels. |
| **`bg.canvas`** | `#f8fafc` | `bg-slate-50` | App background context. |
| **`accent.primary`** | `#2563eb` | `bg-blue-600` | Primary Buttons, "Evidence" Links. |
| **`accent.glow`** | `#2563eb20` | `ring-blue-600/20` | Focus states, active model selection. |

---

## 3. Typography
We use a **Dual-Font System** to separate "Interface" from "Data".

### A. Interface Font: `Inter` (sans-serif)
- Used for: Headers, Labels, Buttons, Chat.
- **Weights**: Medium (500) for body, Bold (700) for headers.
- **Tracking**: `-0.02em` (Tight) for modern feel.

### B. Data Font: `Consolas` / `JetBrains Mono` (monospace)
- Used for: **Financial Tables**, Python Code, JSON Snippets.
- **Why**: Tabular alignment is non-negotiable for comparing millions vs billions.

---

## 4. Component Styles (The "Vibe Coding" Kit)

### The "Glass Card"
```css
.tied-card {
  @apply bg-white/90 backdrop-blur-sm border border-slate-200 shadow-sm rounded-xl;
}
```

### The "Data Cell"
```css
.fsap-cell {
  @apply font-mono text-right px-3 py-1.5 border-b border-slate-100 hover:bg-slate-50 transition-colors;
}
```

### The "Agent Thought" (Stream)
```css
.agent-thought {
  @apply font-mono text-xs text-slate-500 italic border-l-2 border-blue-400 pl-3 my-2 animate-pulse;
}
```
