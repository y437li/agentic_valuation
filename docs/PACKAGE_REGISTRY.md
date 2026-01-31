# TIED Platform - Package Registry

This document tracks all external packages and tools used in the TIED platform.
**Workflow Rule**: When adding a new package, update this document.

---

## Go Backend Packages

### Core Dependencies

| Package | Purpose | Documentation |
|---------|---------|---------------|
| `github.com/joho/godotenv` | Load environment variables from `.env` files | [GitHub](https://github.com/joho/godotenv) |
| `gopkg.in/yaml.v2` | YAML parsing for configuration files | [Go.dev](https://pkg.go.dev/gopkg.in/yaml.v2) |

### HTTP & API

| Package | Purpose | Documentation |
|---------|---------|---------------|
| `net/http` | Standard library HTTP server | [Go.dev](https://pkg.go.dev/net/http) |

### HTML/Markdown Processing

| Package | Purpose | Documentation |
|---------|---------|---------------|
| `github.com/PuerkitoBio/goquery` | jQuery-style HTML parsing and manipulation | [GitHub](https://github.com/PuerkitoBio/goquery) |
| `github.com/JohannesKaufmann/html-to-markdown` | HTML to Markdown conversion (fallback) | [GitHub](https://github.com/JohannesKaufmann/html-to-markdown) |
| `github.com/yuin/goldmark` | Markdown parsing and validation | [GitHub](https://github.com/yuin/goldmark) |

### Database

| Package | Purpose | Documentation |
|---------|---------|---------------|
| `github.com/jackc/pgx/v5` | High-performance PostgreSQL driver | [GitHub](https://github.com/jackc/pgx) |

### LLM Integration

| Package | Purpose | Documentation |
|---------|---------|---------------|
| `github.com/sashabaranov/go-openai` | OpenAI API client | [GitHub](https://github.com/sashabaranov/go-openai) |
| `github.com/google/generative-ai-go` | Google Gemini API client | [GitHub](https://github.com/google/generative-ai-go) |

---

## External CLI Tools

| Tool | Purpose | Installation | Documentation |
|------|---------|--------------|---------------|
| **Pandoc** | Gold-standard HTML → Markdown conversion (handles colspan/rowspan) | `winget install JohnMacFarlane.Pandoc` | [pandoc.org](https://pandoc.org) |

---

## Frontend Packages (web-ui)

### Core Framework

| Package | Purpose | Documentation |
|---------|---------|---------------|
| `next` | React framework with App Router | [nextjs.org](https://nextjs.org) |
| `react` / `react-dom` | UI library | [react.dev](https://react.dev) |
| `typescript` | Type-safe JavaScript | [typescriptlang.org](https://www.typescriptlang.org) |

### UI Components

| Package | Purpose | Documentation |
|---------|---------|---------------|
| `lucide-react` | Icon library | [lucide.dev](https://lucide.dev) |
| `react-markdown` | Markdown rendering | [GitHub](https://github.com/remarkjs/react-markdown) |
| `remark-gfm` | GitHub Flavored Markdown support | [GitHub](https://github.com/remarkjs/remark-gfm) |

### State & Data

| Package | Purpose | Documentation |
|---------|---------|---------------|
| `@supabase/supabase-js` | Supabase client for database access | [supabase.com](https://supabase.com/docs) |

---

## Adding a New Package

When you add a new package to the project:

1. **Go Package**: Run `go get <package>` and update this document
2. **NPM Package**: Run `npm install <package>` in `web-ui/` and update this document
3. **CLI Tool**: Document installation command and add to this registry

---

*Last Updated: 2026-01-20*
