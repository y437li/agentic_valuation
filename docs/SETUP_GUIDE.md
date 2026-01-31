# TIED Agentic Valuation - Environment Setup Guide

## Prerequisites

| Tool | Version | Purpose |
|:---|:---|:---|
| **Go** | 1.21+ | Backend runtime |
| **Node.js** | 18+ | Frontend runtime |
| **pnpm** | 8+ | Package manager |
| **Pandoc** | 3.0+ | Document conversion (SEC filings) |

## 1. Clone & Install

```bash
# Clone the repository
git clone https://github.com/[your-org]/agentic_valuation.git
cd agentic_valuation

# Install frontend dependencies
cd web-ui
pnpm install
cd ..
```

## 2. Environment Variables

Create `.env` file in root directory:

```bash
# Supabase
SUPABASE_URL=https://fzaipbnclsaomczjekpl.supabase.co
SUPABASE_ANON_KEY=your_anon_key_here
SUPABASE_SERVICE_KEY=your_service_key_here

# LLM Providers (at least one required)
GEMINI_API_KEY=your_gemini_key
DEEPSEEK_API_KEY=your_deepseek_key  # Optional
QWEN_API_KEY=your_qwen_key          # Optional

# SEC EDGAR
SEC_USER_AGENT=YourName/YourEmail (for SEC API compliance)
```

## 3. Start Development Servers

### Backend (Go)
```bash
# From root directory
go run cmd/server/main.go
# Runs on http://localhost:8080
```

### Frontend (Next.js)
```bash
cd web-ui
pnpm dev
# Runs on http://localhost:3000
```

## 4. Verify Setup

1. Open http://localhost:3000
2. Try extracting a company: Enter ticker (e.g., "AAPL") and fiscal year
3. Check backend logs for extraction progress

## Project Structure

```
agentic_valuation/
├── cmd/server/         # Backend entry point
├── pkg/
│   ├── api/           # HTTP handlers
│   ├── core/
│   │   ├── edgar/     # SEC filing extraction
│   │   ├── calc/      # Financial calculations
│   │   ├── debate/    # Roundtable (assumption generation)
│   │   ├── analysis/  # Multi-year analysis engine
│   │   └── store/     # Supabase repositories
│   └── providers/     # LLM integrations
└── web-ui/            # Next.js frontend
```

## Key Files

| File | Purpose |
|:---|:---|
| `pkg/core/edgar/v2_extractor.go` | Main extraction pipeline |
| `pkg/core/debate/orchestrator.go` | Roundtable debate engine |
| `pkg/core/debate/material_pool.go` | Data pool for debates |
| `web-ui/src/components/RoundtablePanel.tsx` | Debate UI |

## Common Issues

| Issue | Solution |
|:---|:---|
| Port 8080 in use | Kill existing process: `netstat -ano \| findstr :8080` |
| Go build errors | Clear cache: `go clean -cache && go build ./pkg/...` |
| Supabase connection failed | Verify `.env` vars and project status |

## Database Tables

| Table | Purpose |
|:---|:---|
| `sec_filings` | Filing metadata |
| `fsap_financial_data` | Extracted financial statements |
| `sec_filing_notes` | Extracted notes (qualitative) |
| `sec_note_tables` | Normalized table data from notes |
| `analysis_results` | Multi-year analysis cache |
