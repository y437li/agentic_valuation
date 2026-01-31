# Multi-Agent Assumption Debate System

> **Goal**: 5-agent debate system for generating financial assumptions with real-time transcript, background execution, and web search.

## User Experience (UX) Flow

1.  **Initiation**: User starts a debate for a company (e.g., "Ford FY2025").
2.  **Streaming View**: User is taken to the **Debate Live Room** to watch agents argue in real-time.
3.  **Multitasking**: User can navigate away to other pages (e.g., Data Mapping, Dashboard).
    *   **Global Indicator**: A "🟢 Live Debate: Ford" badge appears in the global header/sidebar.
    *   **Background Running**: The backend continues the debate independently of the frontend connection.
4.  **Re-joining**: Clicking the global indicator takes the user back to the **Debate Live Room**, reconnecting to the stream to see full history + live new messages.
5.  **Completion**:
    *   **Toast Notification**: "Debate Concluded: Consensus Reached for Ford."
    *   **Action**: Clicking notification opens the final Assumption Summary.

---

## System Architecture: Async & Stateful

To support "start and leave", the system uses a **Job Manager Pattern**:

```
┌─────────────────────────┐      ┌──────────────────────────────────────────┐
│       FRONTEND          │      │                 BACKEND                  │
│                         │      │                                          │
│  [Start Button] ───────┼──────┼──> POST /api/debate/start ─────────────┐ │
│                         │      │      (Returns debate_id)                 │ │
│                         │      │                                          │ │
│  [Debate Page] <───────┼──────┼─── GET /api/debate/{id}/stream ◄──────┐ │ │
│   (EventSource)         │      │      (Connects to broadcast)           │ │ │
│                         │      │                                          │ │ │
│  [Global Badge] <──────┼──────┼─── GET /api/debate/active ────────────┤ │ │
│   (Poller/SSE)          │      │                                          │ │ │
└─────────────────────────┘      │    ┌──────────────────────────────────┐  │ │
                                 │    │ 🔄 DebateManager (Singleton)     │  │ │
                                 │    │                                  │  │ │
                                 │    │  [Job 1: Ford] ──────────────────┴──┘ │
                                 │    │    └─ Orchestrator (Goroutine)        │
                                 │    │       └─ Agents (Gemini)              │
                                 │    │       └─ History Buffer               │
                                 │    │       └─ Broadcaster (Channels)       │
                                 │    └───────────────────────────────────────┘
                                 └──────────────────────────────────────────UN──┘
```

---

## Backend Design

### 1. DebateManager (The Container)
Holds references to all active debates in memory.
```go
type DebateManager struct {
    activeDebates map[string]*DebateOrchestrator // Key: debate_id
    mu            sync.RWMutex
}

func (m *DebateManager) GetDebate(id string) *DebateOrchestrator
func (m *DebateManager) StartDebate(company string) string // returns ID
```

### 2. DebateOrchestrator (The Engine)
Runs in its own goroutine. Maintains history for late-joiners.
```go
type DebateOrchestrator struct {
    ID            string
    History       []DebateMessage // Store full history here
    Subscribers   []chan DebateMessage // Active frontend connections
    IsFinished    bool
    // ... context and state
}

// When a new client connects via SSE:
// 1. Send all History messages immediately
// 2. Add client channel to Subscribers for future messages
```

---

## Frontend Design

### 1. Global State (Zustand)
Tracks active debate ID to show the "Live" indicator.
```typescript
interface GlobalState {
  activeDebateId: string | null;
  activeDebateStatus: 'idle' | 'running' | 'completed';
  setActiveDebate: (id: string) => void;
}
```

### 2. Notifications
Uses `sonner` or standard toast library.
- **Listen**: Global SSE or Polling for status change (Running -> Completed).
- **Trigger**: `toast.success("Analysis Complete", { action: { label: "View", onClick: ... } })`

---

## Agent Config (Gemini Grounding)

| Agent | Role | Data Source | Provider |
|:---|:---|:---|:---|
| **Macro Agent** | GDP, interest rates, commodities | **Google Search** | Gemini (Required) |
| **Sentiment Agent** | News sentiment, social | **Google Search** | Gemini (Required) |
| **Fundamentals Agent** | Market & Segment Analysis | **Google Search** + FSAP | Gemini (Required) |
| **Skeptic** | Challenges assumptions | Shared Context | Universal (Configurable) |
| **Optimist** | Defends assumptions | Shared Context | Universal (Configurable) |

---

## API Endpoints

1.  **`POST /api/debate`**
    *   Payload: `{ company: "F", year: "2025" }`
    *   Response: `{ debate_id: "uuid-..." }`

2.  **`GET /api/debate/{id}/stream`**
    *   SSE Endpoint.
    *   Events: `message` (transcript), `status` (completed), `error`.

3.  **`GET /api/debate/active`**
    *   Returns list of active debate IDs (for UI restoration).

---

### Human-in-the-Loop Interaction (Roundtable Trigger)

When the debate is started in `interactive` mode (`"mode": "interactive"`), the Orchestrator enforces a **"Pause & Pivot"** protocol:

1.  **Automatic Pause**: The system automatically pauses execution at key checkpoints:
    *   **Post-Research**: After Phase 1 (Data Gathering) is complete.
    *   **Inter-Round**: Between every round of Phase 2 (Debate).
2.  **Human Trigger**: While paused, the system awaits one of two triggers via API:
    *   **`POST /api/debate/question`**: Users can inject a "God Mode" question to any specific agent. The agent will answer immediately, and the system *remains paused*, allowing for follow-up questions.
    *   **`POST /api/debate/resume`**: Signals the Orchestrator to proceed to the next phase or round.

This allows for deep-dive inspections of specific agents (e.g., questioning the *Accountant* on a specific adjustment) before allowing the debate consensus to proceed.

---

## Timeline Implementation Step

1.  **Backend Core**: `DebateManager` & Async Logic.
2.  **Backend Agents**: Integrate Gemini Grounding.
3.  **API**: SSE Streaming with History Replay.
4.  **Frontend**: Global Indicator & Notification logic.
5.  **Frontend**: Debate Room UI.
