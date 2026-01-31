# Core Package (`pkg/core/`)

Core business logic for the TIED financial modeling platform.

## 📦 Package Overview

| Package | Description | Tests |
|:---|:---|:---|
| **`assumption`** | AssumptionSet backend (syncs with frontend) | 13 ✅ |
| **`projection`** | Polymorphic Node System (Strategy Pattern) | 12 ✅ |
| **`knowledge`** | Unified Knowledge Layer (RAG support) | 10 ✅ |
| `edgar` | SEC filing parsing + FSAPValue | 5 ✅ |
| `fee` | Financial Extraction Engine | - |
| `llm` | Multi-provider LLM client | - |
| `debate` | Multi-agent debate orchestration | - |
| `calc` | Calculation engine | - |
| `ingest` | File ingestion pipeline | - |
| `prompt` | Prompt templates | - |
| `store` | Data stores | - |
| `agent` | Agent utilities | - |
| `utils` | Utilities | - |

**Bold** = TIED v2.0 Architecture packages

---

## 🏗️ v2.0 Architecture: "Fixed Skeleton, Dynamic Flesh"

### Core Concept

```
┌─────────────────────────────────────────────────┐
│           Fixed Skeleton (Go enforced)          │
│  Revenue → COGS → GrossProfit → OpEx → NetInc   │
├─────────────────────────────────────────────────┤
│         Dynamic Drivers (AI attached)           │
│  auto_price, auto_volume, auto_unit_cost...     │
└─────────────────────────────────────────────────┘
```

- **AI cannot delete** skeleton nodes (accounting identity enforced)
- **AI can attach** dynamic driver nodes (Price × Volume)
- **Data-driven strategy**: AI selects strategy from discovered data

### Key Types

```go
// pkg/core/projection
type ProjectionStrategy interface {
    Name() string
    Calculate(ctx Context) (float64, error)
    RequiredDrivers() []string
}

// pkg/core/knowledge  
type KnowledgeAsset struct { /* SEC, PDF, WEB, EXCEL */ }
type Chunk struct { /* Semantic unit for RAG */ }

// pkg/core/assumption
type AssumptionSet struct { /* Container syncs with frontend */ }
```

---

## 🧪 Testing

```powershell
# All core tests (40 tests)
go test ./pkg/core/... -count=1

# v2.0 packages only
go test ./pkg/core/projection/... -v   # 12 tests
go test ./pkg/core/knowledge/... -v    # 10 tests  
go test ./pkg/core/assumption/... -v   # 13 tests
go test ./pkg/core/edgar/... -v        # 5 tests
```

---

## 📁 Directory Structure

```
core/
├── agent/           # Agent utilities
├── assumption/      # AssumptionSet ⭐
│   ├── types.go
│   └── assumption_test.go
├── calc/            # Calculation engine
├── debate/          # Multi-agent debate
├── edgar/           # SEC parsing + FSAPValue ⭐
│   └── types.go     # Citation, DataSourceType
├── fee/             # Financial Extraction Engine
├── ingest/          # File ingestion
├── knowledge/       # Knowledge Layer ⭐
│   ├── types.go
│   └── store.go
├── llm/             # LLM providers
├── projection/      # Polymorphic Nodes ⭐
│   ├── strategy.go
│   ├── skeleton.go
│   └── selector.go
├── prompt/          # Prompt templates
├── store/           # Data stores
└── utils/           # Utilities
```

⭐ = v2.0 Architecture packages
