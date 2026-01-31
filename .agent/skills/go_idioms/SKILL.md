---
name: go_idioms
description: Enforcing specific Go coding manners, idioms, and best practices.
---

# Go Code Manner (Idioms) Skill

## 1. Purpose
To maintain a high standard of "Go Manner" - writing code that feels native to Go (Idiomatic), rather than Go written like Java or Python. This ensures consistency, readability, and reliability.

## 2. Core Manners (The "TIED" Way)

### A. Error Handling (The Prime Directive)
- **Handle Everything**: Never use `_` to ignore errors in production code.
- **Fail Fast**: Check errors immediately. Avoid deep nesting (`else` blocks after validation).
- **Wrap, Don't Hide**: Use `fmt.Errorf("context: %w", err)` to add context while preserving the original error for inspection.
    ```go
    // Bad
    if err != nil { return err }

    // Good
    if err != nil { return fmt.Errorf("failed to open database: %w", err) }
    ```

### B. Naming Conventions
- **Package Scope**: Use concise, meaningful names. Avoid stuttering (`agent.AgentConfig` -> `agent.Config`).
- **Local Scope**: Short names (`i`, `ctx`, `w`, `r`) are acceptable and preferred for common types.
- **Getters**: Go doesn't often use `Get...` prefixes. `obj.Owner()` is better than `obj.GetOwner()`.
- **Interfaces**: Methods ending in `-er` (e.g., `Reader`, `Writer`, `Provider`).

### C. Structure & Organization
- **Keep Main Small**: `cmd/api/main.go` should only wire dependencies and start the server. Logic belongs in `pkg/`.
- **Package by Feature/Domain**: Group code by what it *does* (e.g., `valuation`, `ingest`), not technical layer (e.g., `controllers`, `models`).
- **Zero Values**: Make structs useful when empty (zero value) whenever possible.

### D. Concurrency
- **Context is King**: Pass `context.Context` as the first argument to any function that does I/O or takes time.
- **Do Not Leave Goroutines**: If you start a goroutine, you must know how it stops. Use `errgroup` or `sync.WaitGroup`.
- **Channels**: Use channels for signaling and data transfer, not just as locks.

## 3. The "Code Manner" Checklist
*Before committing Go code, ask:*

1.  **Is it formatted?** (Always run `go fmt`).
2.  **Is the error handling robust?** (Did I wrap errors?).
3.  **Is the naming simple?** (Did I avoid `ManagerServiceHelper` nonsense?).
4.  **Are dependencies clear?** (Is `main` wiring them up?).
5.  **Is there a test?** (At least for the happy path).

## 4. Anti-Patterns to Reject
- "Java-isms": Getters/Setters for everything, `AbstractFactoryPatterns`.
- "Python-isms": Dynamic typing abuse (too much `interface{}`), ignoring errors.
- Global State: Avoid global variables (except maybe configuration constants). Use Dependency Injection (passing structs).

## 5. Tooling
- **`go mod tidy`**: Keep dependencies clean.
- **`go test ./...`**: Ensure no regressions.
- **`staticcheck`**: (Optional) Use static analysis to catch bugs early.
