# Prompt Library

Centralized management for all LLM prompts used in the TIED Agentic Valuation Platform.

## Overview

The Prompt Library allows prompts to be stored as JSON files and loaded at runtime, making it easy to:
- Update prompts without code changes
- Version control prompts separately from code
- A/B test different prompt variations
- Support multiple languages or domains

## Directory Structure

```
resources/
├── prompts/
│   ├── debate/           # Multi-agent debate system prompts
│   │   ├── macro.json
│   │   ├── sentiment.json
│   │   ├── fundamental.json
│   │   ├── skeptic.json
│   │   ├── optimist.json
│   │   └── synthesizer.json
│   ├── extraction/       # Financial statement extraction prompts
│   │   ├── balance_sheet.json        # [v1.0] Direct LLM extraction
│   │   ├── income_statement.json     # [v1.0] Direct LLM extraction
│   │   ├── cash_flow.json            # [v1.0] Direct LLM extraction
│   │   ├── supplemental.json
│   │   ├── toc_analysis.json
│   │   ├── notes_analysis.json
│   │   ├── base_rules.json
│   │   ├── v2_navigator_toc.json           # [v2.0] LLM Navigator
│   │   ├── v2_table_mapper_balance_sheet.json   # [v2.0] Table Mapper
│   │   ├── v2_table_mapper_income_statement.json
│   │   └── v2_table_mapper_cash_flow.json
│   ├── qualitative/      # Qualitative analysis agent prompts
│   │   ├── strategy.json
│   │   ├── capital_allocation.json
│   │   ├── segment.json
│   │   └── risk.json
│   └── assistant/        # AI assistant prompts
│       └── navigation.json
└── schemas/              # JSON response schemas (optional)
    └── ...
```

## Extraction Architecture

### v1.0 Architecture (Direct LLM Extraction)

```
LLM Prompt → JSON with all extracted values
```

**Files**: `balance_sheet.json`, `income_statement.json`, `cash_flow.json`

- LLM directly extracts numeric values
- Higher token cost
- Potential for numeric errors

### v2.0 Architecture (LLM Navigator + Go Extractor)

```
LLM Navigator → Section locations (SectionMap)
        ↓
LLM Table Mapper → Row indices + FSAP mappings (LineItemMapping)
        ↓
Go Extractor → Precise numeric extraction (FSAPValue[])
        ↓
aggregation.go → Cross-validation (ValidationReport)
```

**Files**: `v2_navigator_toc.json`, `v2_table_mapper_*.json`

- LLM only provides **indices and mappings** (not values)
- Go handles **precise numeric parsing**
- Lower token cost, higher accuracy
- Built-in cross-validation

### Choosing an Architecture

| Use Case | Recommendation |
|----------|----------------|
| Quick extraction | v1.0 (simpler) |
| Production accuracy | v2.0 (recommended) |
| Audit trail needed | v2.0 (has provenance) |
| Jump-to-source | v2.0 (has markdown_line) |

## JSON Prompt Format

Each prompt file follows this structure:

```json
{
  "id": "extraction.balance_sheet",
  "name": "Balance Sheet Extraction",
  "category": "extraction",
  "description": "Extracts Balance Sheet data from SEC 10-K",
  "version": "1.0.0",
  "system_prompt": "You are an expert Financial Analyst...",
  "user_prompt_template": "Extract data for {{.CompanyName}} FY{{.FiscalYear}}...",
  "response_schema_ref": "balance_sheet_response",
  "variables": [
    {"name": "CompanyName", "type": "string", "required": true},
    {"name": "FiscalYear", "type": "int", "required": true}
  ]
}
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique identifier (e.g., `extraction.balance_sheet`) |
| `name` | Yes | Human-readable name |
| `category` | No | Category for grouping (auto-detected from folder) |
| `description` | No | Description of prompt purpose |
| `version` | No | Version for tracking changes |
| `system_prompt` | Yes | The system prompt content |
| `user_prompt_template` | No | Go template for user prompt |
| `response_schema_ref` | No | Reference to JSON schema for validation |
| `variables` | No | Variables used in template |

## Usage in Go Code

### 1. Basic Usage

```go
import "agentic_valuation/pkg/core/prompt"

// Get a prompt by ID
p, err := prompt.Get().GetPrompt("debate.macro")
if err != nil {
    // Handle error
}
systemPrompt := p.SystemPrompt
```

### 2. Using Convenience Functions

```go
// Get debate prompts
sysPrompt, _ := prompt.GetDebatePrompt("macro")

// Get extraction prompts
extractPrompt, _ := prompt.GetExtractionPrompt("balance_sheet")

// Get qualitative prompts
strategyPrompt, _ := prompt.GetQualitativePrompt("strategy")
```

### 3. Using Typed IDs

```go
p, _ := prompt.Get().GetPrompt(prompt.PromptIDs.DebateSynthesizer)
p, _ := prompt.Get().GetPrompt(prompt.PromptIDs.ExtractionBalanceSheet)
```

### 4. Rendering User Prompts with Variables

```go
ctx := prompt.NewContext().
    Set("CompanyName", "Apple Inc.").
    Set("FiscalYear", 2024)

userPrompt, err := prompt.RenderUserPrompt(p, ctx)
```

## Fallback Behavior

All code using the prompt library includes fallback to hardcoded prompts in case:
- The prompt library fails to load
- A specific prompt file is missing
- The JSON is malformed

This ensures the system continues to function even if prompt files are not available.

## Adding a New Prompt

1. Create a new JSON file in the appropriate category folder
2. Add the prompt ID to `pkg/core/prompt/convenience.go` in `PromptIDs`
3. Update the relevant Go code to use the new prompt

## Best Practices

1. **Keep prompts focused**: Each prompt should have a single, clear purpose
2. **Use descriptive IDs**: Follow the `category.name` convention
3. **Version your prompts**: Increment version when making significant changes
4. **Test changes**: Verify prompt changes don't break existing functionality
5. **Document variables**: Clearly describe what each template variable represents
