# TIED Backend Packages (`pkg/`)

Go packages for the TIED financial modeling platform.

## 📦 Package Overview

| Package | Description |
|:---|:---|
| `pkg/api` | HTTP handlers and API routes |
| `pkg/core` | Core business logic (see below) |
| `pkg/models` | Database models and DTOs |

---

## 🏗️ Core Package Architecture (`pkg/core/`)

Implements **TIED v2.0 Architecture**: "Fixed Skeleton, Dynamic Flesh"

### Key Packages

| Package | Purpose | Tests |
|:---|:---|:---|
| `core/edgar` | SEC filing parsing, FSAPValue + Citation | 5 ✅ |
| `core/projection` | Polymorphic Node System (Strategy Pattern) | 12 ✅ |
| `core/knowledge` | Unified Knowledge Layer (RAG) | 10 ✅ |
| `core/assumption` | AssumptionSet backend | 13 ✅ |
| `core/llm` | Multi-provider LLM client | - |
| `core/fee` | Financial Extraction Engine | - |
| `core/debate` | Multi-agent debate orchestration | - |

### Core Concepts

```
┌─────────────────────────────────────────────────┐
│           Fixed Skeleton (Go enforced)          │
│  Revenue → COGS → GrossProfit → OpEx → NetInc   │
├─────────────────────────────────────────────────┤
│         Dynamic Drivers (AI attached)           │
│  auto_price, auto_volume, auto_unit_cost...     │
└─────────────────────────────────────────────────┘
```

- **AI cannot delete** skeleton nodes (Revenue, COGS, etc.)
- **AI can attach** dynamic driver nodes (Price × Volume)
- **Strategy Pattern**: Each node carries a `ProjectionStrategy`

---

## 🧪 Running Tests

```powershell
# All core tests (40 tests)
go test ./pkg/core/... -v

# Specific packages
go test ./pkg/core/projection/... -v   # 12 tests
go test ./pkg/core/knowledge/... -v    # 10 tests
go test ./pkg/core/assumption/... -v   # 13 tests
```

---

## 📁 Directory Structure

```
pkg/
├── api/
│   └── edgar/           # Edgar API endpoints
├── core/
│   ├── agent/           # Agent utilities
│   ├── assumption/      # AssumptionSet backend ⭐ NEW
│   ├── calc/            # Calculation engine
│   ├── debate/          # Multi-agent debate
│   ├── edgar/           # SEC parsing + FSAPValue
│   ├── fee/             # Financial Extraction Engine
│   ├── ingest/          # File ingestion
│   ├── knowledge/       # Knowledge Layer ⭐ NEW
│   ├── llm/             # LLM providers
│   ├── projection/      # Polymorphic Nodes ⭐ NEW
│   ├── prompt/          # Prompt templates
│   ├── store/           # Data stores
│   └── utils/           # Utilities
└── models/              # Database models
```

---

**⭐ NEW** = TIED v2.0 Architecture packages (40 tests passing)
