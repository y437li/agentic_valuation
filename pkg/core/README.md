# Core Package (`pkg/core/`)

Core business logic for the TIED financial modeling platform.

## 📦 Package Overview

| Package | Description | Tests |
|:---|:---|:---|
| **`edgar`** | SEC parsing + v2.0 Extraction (Navigator/Mapper/GoExtractor) | 5 ✅ |
| **`analysis`** | Multi-year Analysis Engine (Common-Size, ROCE, Forensics) | ✅ |
| **`projection`** | Polymorphic Node System (Strategy Pattern) | 12 ✅ |
| **`assumption`** | AssumptionSet backend (syncs with frontend) | 13 ✅ |
| **`knowledge`** | Unified Knowledge Layer (RAG support) | 10 ✅ |
| **`pipeline`** | End-to-end Pipeline Orchestrator | - |
| `calc` | Deterministic calculation engine | - |
| `synthesis` | Zipper algorithm + Reclassification | - |
| `debate` | Multi-agent debate orchestration | - |
| `llm` | Multi-provider LLM client | - |
| `store` | Supabase persistence layer | - |
| `prompt` | Centralized prompt registry | - |
| `ingest` | File ingestion pipeline | - |

**Bold** = v2.0 Architecture packages

---

## 🏗️ v2.0 Extraction Architecture

### Navigator → Mapper → GoExtractor Pipeline

```
 ┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
 │ NavigatorAgent  │ ──► │ TableMapperAgent │ ──► │   GoExtractor   │
 │   (LLM: TOC)    │     │ (LLM: Row→FSAP)  │     │ (Go: Values)    │
 └─────────────────┘     └──────────────────┘     └─────────────────┘
          │                       │                        │
    SectionMap             LineItemMapping           []*FSAPValue
```

**Responsibility Slicing**:
- **LLM** = Semantic understanding (TOC parsing, row-to-variable mapping)
- **Go** = Deterministic value extraction and struct population

### Entry Points

```go
// New v2.0 entry point
v2 := edgar.NewV2Extractor(provider)
result, err := v2.Extract(ctx, markdown, fillingMeta)

// Legacy adapter (internally uses V2Extractor)
analyzer := edgar.NewLLMAnalyzer(provider)
result, err := analyzer.ParallelFullTableExtraction(ctx, markdown, meta)
```

---

## 🔗 Pipeline Orchestrator

End-to-end flow from ticker to analyzed company profile:

```go
orchestrator := pipeline.NewPipelineOrchestrator(
    fetcher,   // ContentFetcher
    provider,  // AIProvider
    store,     // AnalysisRepository
)
err := orchestrator.RunForCompany(ctx, "TSLA", "1318605", filings)
```

---

## 📁 Directory Structure

```
core/
├── analysis/        # Multi-year Analysis Engine ⭐
│   ├── engine.go    # AnalyzeCompany(), ThreeLevelAnalysis
│   └── types.go     # AnalysisResult, ForensicResults
├── edgar/           # SEC Extraction ⭐
│   ├── v2_extractor.go  # V2Extractor (main entry)
│   ├── navigator.go     # NavigatorAgent (TOC)
│   ├── mapper.go        # TableMapperAgent (semantics)
│   ├── go_extractor.go  # GoExtractor (values)
│   └── types.go         # FSAPValue, FSAPDataResponse
├── pipeline/        # Orchestration ⭐
│   └── orchestrator.go
├── projection/      # Polymorphic Nodes ⭐
├── assumption/      # AssumptionSet ⭐
├── knowledge/       # Knowledge Layer ⭐
├── calc/            # Calculation engine
├── synthesis/       # Zipper + Reclassification
├── debate/          # Multi-agent debate
├── llm/             # LLM providers
├── store/           # Supabase repos
└── prompt/          # Prompt registry
```

⭐ = v2.0 Architecture packages

---

## 🧪 Testing

```powershell
# All core tests
go test ./pkg/core/... -count=1

# v2.0 packages
go test ./pkg/core/edgar/... -v
go test ./pkg/core/analysis/... -v
go test ./pkg/core/projection/... -v
```
