# API Package (`pkg/api/`)

HTTP handlers and REST API endpoints for the TIED platform.

## 📦 Sub-packages

| Package | Description |
|:---|:---|
| `api/assistant` | AI Assistant navigation endpoint |
| `api/config` | Configuration management API |
| `api/debate` | Multi-agent debate orchestration API |
| `api/edgar` | SEC filing extraction endpoints |

---

## 🔗 Key Endpoints

### Edgar API (`api/edgar/`)

| Endpoint | Method | Description |
|:---|:---|:---|
| `/api/edgar/stream` | GET | SSE stream for 10-K extraction with progress |
| `/api/edgar/filings` | GET | List available filings for a CIK |
| `/api/edgar/parse` | POST | Parse raw SEC filing |

### Debate API (`api/debate/`)

| Endpoint | Method | Description |
|:---|:---|:---|
| `/api/debate/stream` | GET | SSE stream for multi-agent debate |
| `/api/debate/assumptions` | GET | Get generated assumptions |

### Assistant API (`api/assistant/`)

| Endpoint | Method | Description |
|:---|:---|:---|
| `/api/assistant/navigate` | POST | AI-powered navigation intent parsing |

---

## 🏗️ Handler Pattern

All handlers follow the standard pattern:

```go
func HandleSomething(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    // 2. Validate input
    // 3. Call core business logic
    // 4. Return JSON response
}
```

---

## 📁 Directory Structure

```
api/
├── assistant/     # AI navigation endpoint
├── config/        # Config management
├── debate/        # Multi-agent debate API
└── edgar/         # SEC filing extraction
    ├── handler.go
    ├── routes.go
    └── stream_handler.go  # SSE streaming
```
