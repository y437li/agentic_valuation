---
name: markdown_standards
description: Defines the strict GitHub Flavored Markdown (GFM) standards for all Agent outputs.
---

# Markdown Standards

## Core Principle
All AI Agents must output **clean, raw GitHub Flavored Markdown (GFM)**.
- **NO** conversational filler ("Here is the report...", "Sure, I can help with that...").
- **NO** wrapping code blocks (do not wrap the entire output in ```markdown ... ```).
- **NO** HTML unless strictly necessary for tables not supported by GFM (avoid if possible).

## Structural Rules
1. **Top-Level Header**: Start with a `# Header 1` or `## Header 2` immediately.
2. **Hierarchy**: Use proper header nesting (# -> ## -> ###).
3. **Lists**: Use standard hyphens `-` for bullet points.
4. **Tables**: Use standard Markdown table syntax.
   ```markdown
   | Column A | Column B |
   |----------|----------|
   | Data 1   | Data 2   |
   ```

## Example Valid Output
```markdown
## Executive Summary
The company has shown strong resilience...

### Key Risks
- **Inflation**: Rising costs...
- **Competition**: New entrants...
```

## Example INVALID Output
```text
Sure! Here is the analysis you asked for:

```markdown
# Executive Summary
...
```
Hope this helps!
```
