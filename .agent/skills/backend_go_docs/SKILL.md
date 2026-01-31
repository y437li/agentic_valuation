---
name: backend_go_docs
description: Instructions for maintaining high-quality Go documentation (GoDoc).
---

# Backend Go Documentation Skill

## 1. Purpose
To ensure the Go backend code is self-documenting, traversable, and readable by both humans and standard tooling (godoc). Good documentation reduces cognitive load and accelerates onboarding.

## 2. When to Use
Activate this skill when:
- Creating a **new package**.
- Adding **exported (public)** functions, types, or constants.
- Refactoring complex logic that needs explanation.
- Updating API contracts.

## 3. GoDoc Standards

### A. Package Documentation
Every package **MUST** have a package comment immediately preceding the `package` clause.
For non-main packages, this comment should explain the *purpose* of the package.

**Simple Package:**
```go
// Package llm provides interfaces and implementations for Large Language Model providers.
package llm
```

**Complex Package (`doc.go`):**
For packages with many files, creating a separate `doc.go` file is recommended.
```go
// Package agent manages the lifecycle, configuration, and execution of AI agents.
//
// It handles:
// - Provider selection (DeepSeek, OpenAI, etc.)
// - Agent-specific configuration
// - Prompt execution and instruction adaptation
package agent
```

### B. Exported Members
**ALL** exported symbols (Starting with Uppercase) **MUST** have a documentation comment.
The comment **MUST** start with the name of the symbol.

**Functions:**
```go
// RepairJSON attempts to fix common syntax errors in malformed JSON strings.
// It returns an error if repair is impossible.
func RepairJSON(input string) (string, error)
```

**Types:**
```go
// Provider defines the interface for LLM backends.
type Provider interface { ... }
```

### C. Deprecation
Use the `// Deprecated:` marker to signal obsolete code.
```go
// Generate is the old way to call LLMs.
// Deprecated: Use GenerateResponse instead.
func Generate(...)
```

## 4. Documentation workflow
1.  **Write the Code**.
2.  **Add Comments**: Immediately add GoDoc comments for all exported members.
3.  **Verify**: Ensure sentences are complete and grammar is correct.
4.  **Update `README.md`**: For major architectural components, update the directory's `README.md` (if it exists) to reflect high-level changes.

## 5. Example: Well-Documented Function
```go
// SmartParse tries to parse the input string into the v interface.
//
// Strategy:
// 1. Attempt standard JSON Unmarshal.
// 2. If that fails, attempt to RepairJSON then Unmarshal.
// 3. If that fails, attempt ParseHJSON (relaxed syntax).
//
// It returns a comprehensive error listing all attempted strategies if all fail.
func SmartParse(input string, v interface{}) error {
    // ... implementation
}
```
