# Frontend Architecture: Agentic Valuation Platform

This document outlines the frontend architecture for the Agentic Valuation Platform, focusing on high-precision financial data rendering and real-time agentic interaction.

## 1. Technology Stack
- **Framework**: [Next.js 15+](https://nextjs.org/) (App Router)
- **Language**: [TypeScript](https://www.typescriptlang.org/) (Strict Mode)
- **Styling**: [Tailwind CSS 4.0](https://tailwindcss.com/) (Using Oxide engine for performance)
- **Data Visualization**: [Recharts](https://recharts.org/) (Declarative charts)
- **Animations**: [Framer Motion](https://www.framer.com/motion/) (Micro-interactions & state transitions)
- **Icons**: [Lucide React](https://lucide.dev/)
- **Communication**: [SSE (Server-Sent Events)](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) for real-time streaming of agent thought processes.

## 2. Component Architecture
The UI is divided into three primary zones:

### 2.1 The Ingestion Engine (Green Zone)
- **Purpose**: Raw data upload and semantic mapping review.
- **Key Features**: 
    - Drag-and-drop PDF uploader.
    - Side-by-side view (Original PDF vs. Mapped FSAP items).
    - Highlighted reclassification alerts.

### 2.2 The Analytical Dashboard (White/Grey Zone)
- **Purpose**: Financial statement rendering and interactive analysis.
- **Key Components**:
    - **FSAP Table**: Pivot-like table with color coding (Green: Input, Grey: Sum, White: Calc).
    - **Drill-down Modals**: Click a cell to see the underlying "Agent Reason" and source Note.
    - **Recharts Integration**: Multi-series line charts for revenue growth and margin trends.

### 2.3 The Agentic Side-car (Terminal)
- **Purpose**: Real-time dialogue and control.
- **UI Mechanism**: 
    - Floating or docked chat interface.
    - **SSE Stream**: Displays "Thought Steps" (Scanning page 1... Identifying COGS... Comparing with 2023...) as they happen.
    - **Global Model Switcher**: UI control to hit `/api/config/switch` and change model providers on the fly.

## 3. State Management
- **Local State**: `useState` / `useReducer` for UI-only toggles.
- **Server State**: `TanStack Query` (React Query) for fetching FSAP data and snapshots.
- **Real-time**: Custom `useSSE` hook to subscribe to agentic execution streams.

## 4. Design System (Tailwind 4)
- **Vibe**: Dark mode by default, premium "Glassmorphism" effects for tooltips and side-panels.
- **Colors**:
    - `Input/Green`: `#10B981`
    - `Sum/Grey`: `#6B7280`
    - `Calc/White`: `#FFFFFF`
    - `Accent/Premium`: Cyan/Gold gradients for valuation results.

## 5. Development Workflow
1.  **Project Init**: `create-next-app` in `web-ui`.
2.  **Tailwind 4 Setup**: Configure the new `@theme` block.
3.  **SSE Client**: Implement the connection manager for streaming agent reasoning.
4.  **Component Library**: Primitive building blocks (Button, Input, Table) using Framer Motion.
