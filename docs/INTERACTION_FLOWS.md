# TIED Interaction Flows

This document maintains all user interaction logic and flows for the TIED platform.

---

## Table of Contents

1. [Case Lifecycle](#1-case-lifecycle)
2. [Roundtable Debate Flow](#2-roundtable-debate-flow)
3. [Assumption Management](#3-assumption-management)
4. [Filing Update Flow](#4-filing-update-flow)

---

## 1. Case Lifecycle

### New Case Entry

```
User creates new Case
        │
        ▼
┌─────────────────────────────┐
│  Pipeline Progress (6 steps) │
│  1. Input Data              │
│  2. Fetch Filing (SEC)      │
│  3. Parse Data              │
│  4. Validation              │
│  5. Roundtable (mandatory)  │
│  6. Ready                   │
└─────────────────────────────┘
        │
        ▼
User can [Skip] Roundtable → Uses default assumptions
User can [Wait] → Sees progress, gets Agent-generated assumptions
```

### Existing Case Entry

```
User opens existing Case
        │
        ▼
Load saved assumptions → Enter workspace directly
        │
        └── [Refresh Assumptions] button available in Roundtable panel
```

---

## 2. Roundtable Debate Flow

### Trigger Location

**Primary**: Roundtable Panel header → [▶ Start Roundtable] button

### Stages (6 total)

| Stage | Agent | Duration |
|-------|-------|----------|
| 1 | Macro Analyst | ~15-20s |
| 2 | Market Sentiment | ~15-20s |
| 3 | Fundamental Analyst | ~15-20s |
| 4 | The Skeptic | ~10-15s |
| 5 | The Optimist | ~10-15s |
| 6 | Synthesizer (CIO) | ~20-30s |

### Output

Synthesizer produces **single JSON** containing:
- `memorandum`: Markdown report
- `assumptions[]`: Structured assumption data

This is the **Single Source of Truth** for:
- Frontend display
- Assumption store
- Calculation engine

---

## 3. Assumption Management

### Three-Layer Hierarchy

| Layer | Nature | Source |
|-------|--------|--------|
| Level 1 | Fixed templates | Predefined |
| Level 2 | Dynamic segments | Agent (from 10-K) |
| Level 3 | Dynamic details | Agent (from 10-K) |

### Apply/Remove Toggle

```
┌─────────────────────────────────────────┐
│  Revenue Growth                [✓ Applied]
│  ├── Ford Blue      -2.4%      [✓]
│  ├── Model e        +35.0%     [✓]
│  └── Ford Pro       +5.4%      [ ] ← Disabled
└─────────────────────────────────────────┘
```

- **Applied (☑️)**: Included in Forecast calculations
- **Removed (◻️)**: Excluded but data preserved
- Can toggle anytime without losing data

### Diff Review (for existing Cases)

When Roundtable suggests changes:

```
┌─────────────────────────────────────────┐
│  Suggested Changes                       │
│                                          │
│  Revenue Growth: 3.2% → 2.8%            │
│  [Accept] [Reject]                       │
│                                          │
│  + NEW: EV Subsidy Impact               │
│  [Add] [Ignore]                          │
│                                          │
│  [Accept All] [Reject All]              │
└─────────────────────────────────────────┘
```

**Rules**:
- Existing structure is NOT auto-replaced
- Only suggestions for changes/additions
- User has full control

---

## 4. Filing Update Flow

### Background Monitoring

```
Backend Worker (hourly)
        │
        ▼
Check SEC RSS for ticker
        │
        ├── No new filing → Do nothing
        │
        └── New 10-Q/8-K found
                │
                ▼
        Background fetch + parse
                │
                ▼
        WebSocket notification to frontend
                │
                ▼
        ┌─────────────────────────────────┐
        │  "New Q3 data available"        │
        │  [Review] [Auto-Apply]          │
        └─────────────────────────────────┘
```

### Data Types

| Type | Update Behavior |
|------|-----------------|
| **Historical** (Q3 Revenue) | Auto-fill, no Agent needed |
| **Forward-Looking** (Guidance) | Agent analyzes, suggests assumption changes |

---

## 5. Version History

### Assumption Versions

```
v3 - Current (Jan 20, 2026 15:00) ← Active
v2 - After Q3 Update (Jan 15, 2026)
v1 - Initial Roundtable (Jan 10, 2026)

[Compare v2 vs v3]  [Restore v2]
```

---

## Related Documents

- `docs/DECISION_LOG.md` - TX-020 to TX-024
- `docs/REQUIREMENT_LOG.md` - REQ-019 to REQ-023
- `docs/AGENT_REGISTRY.md` - Agent inventory
