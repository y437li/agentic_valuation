---
name: decision_log
description: Using the TIED Project Decision Log to maintain architectural continuity.
---

# Decision Log Skill

## 1. Purpose

To ensure that all future development is **"TIED"** (consistent) with previous key architectural, product, and strategic decisions. The Decision Log acts as an **immutable transaction record** - decisions are never deleted, only superseded.

## 2. Source of Truth

The Decision Log is located at:  
**[`docs/DECISION_LOG.md`](file:///c:/Users/y437l/OneDrive/Rotman/agentic_valuation/docs/DECISION_LOG.md)**

---

## 3. When to CHECK the Log (Read)

You **MUST** check the Decision Log before:

| Scenario | Why |
|----------|-----|
| Starting any new feature | Ensure it aligns with existing architectural decisions |
| Changing technology choices | Verify no conflicts with TX-003 (Tech Stack) decisions |
| Modifying core data models | Check database decisions (TX-004) |
| Proposing UI/UX changes | Verify consistency with branding (TX-001) and UI decisions |
| Adding new dependencies | Ensure compatibility with established patterns |

**How to Check:**
```
1. Read docs/DECISION_LOG.md
2. Search for relevant Category (Branding, Strategy, Tech Stack, Database, UI/UX, Feature, etc.)
3. Note any Active decisions that apply to your work
4. If in conflict, discuss with user before proceeding
```

---

## 4. When to UPDATE the Log (Write)

You **SHOULD** add a new entry when:

| Trigger | Example |
|---------|---------|
| **New architectural choice** | Adding a new library, framework, or service |
| **Strategy shift** | Changing development priorities or roadmap |
| **Breaking change** | Modifying APIs, schemas, or interfaces |
| **Feature decision** | Committing to a specific implementation approach |
| **Technology replacement** | Superseding a previous technology choice |
| **User consensus reached** | Any significant decision agreed upon in conversation |

**DO NOT add entries for:**
- Bug fixes or minor refactoring
- Implementation details that don't affect architecture
- Temporary workarounds (unless they become permanent)

---

## 5. Transaction Entry Format

Each decision follows this structure:

```markdown
| ID | Date | Category | Decision | Rationale (Why) | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **TX-XXX** | YYYY-MM-DD | **Category** | **Decision Title** | Explanation of why this decision was made | `Status` |
```

### Field Definitions

| Field | Format | Description |
|-------|--------|-------------|
| **ID** | `TX-XXX` | Sequential transaction ID (e.g., TX-007, TX-008) |
| **Date** | `YYYY-MM-DD` | Date decision was made (use current date) |
| **Category** | Bold text | One of: Branding, Strategy, Tech Stack, Database, UI/UX, Feature, Integration, Security, Performance |
| **Decision** | Bold, concise | What was decided (imperative statement) |
| **Rationale** | Explanation | Why this decision was made - context, tradeoffs considered |
| **Status** | Code style | One of: `Active`, `Implemented`, `Superseded`, `Deprecated` |

### Status Definitions

| Status | Meaning |
|--------|---------|
| `Active` | Decision is in effect, development should align with it |
| `Implemented` | Decision has been fully implemented in the codebase |
| `Superseded` | Replaced by a newer decision (reference the new TX-ID) |
| `Deprecated` | No longer recommended but may still exist in codebase |

---

## 6. Mandatory Workflow for Adding Entries

When adding a new decision, follow this workflow:

### Step 1: Determine Next ID
```
1. Read the last entry in DECISION_LOG.md
2. Extract the current max ID (e.g., TX-006)
3. Increment to get next ID (e.g., TX-007)
```

### Step 2: Get Current Date
Use today's date in `YYYY-MM-DD` format (e.g., 2026-01-18)

### Step 3: Choose Category
Select the most appropriate category from:
- **Branding** - Project naming, identity, messaging
- **Strategy** - Roadmap, priorities, timelines
- **Tech Stack** - Languages, frameworks, libraries
- **Database** - Schema, storage, data architecture
- **UI/UX** - Interface design, user experience
- **Feature** - Specific feature implementation decisions
- **Integration** - External services, APIs, third-party tools
- **Security** - Authentication, authorization, data protection
- **Performance** - Optimization, scaling, caching strategies

### Step 4: Write the Entry
```markdown
| **TX-XXX** | YYYY-MM-DD | **Category** | **Decision Title** | Rationale explaining why | `Active` |
```

### Step 5: Append to Log
Use file editing tools to append the new row to the table in `docs/DECISION_LOG.md`.

---

## 7. Example: Adding a New Decision

**Scenario**: User decides to use jsonrepair library for JSON handling.

**New Entry:**
```markdown
| **TX-007** | 2026-01-18 | **Tech Stack** | **Integrate JSON Repair Libraries** | To handle malformed JSON from LLM outputs. Using hjson-go (Go) and jsonrepair (TypeScript) for language-native solutions instead of Python dependencies. | `Implemented` |
```

**Append Command:**
```
1. Open docs/DECISION_LOG.md
2. Add new row at the end of the table
3. Ensure proper markdown table formatting
```

---

## 8. Superseding Decisions

When a decision is replaced:

1. **Update old entry's status** from `Active` to `Superseded`
2. **Add reference** to the superseding TX-ID in the Rationale
3. **Add new entry** with the replacement decision

**Example of superseding:**
```markdown
| **TX-003** | 2026-01-17 | **Tech Stack** | **Hybrid Stack: Go + Next.js + Python** | ... | `Superseded by TX-010` |
| **TX-010** | 2026-02-15 | **Tech Stack** | **Migrate to Full Go Stack** | Python scripts replaced by Go services for unified deployment. | `Active` |
```

---

## 9. Quick Reference Commands

### Check for conflicts before work:
```
Read docs/DECISION_LOG.md and verify no Active decisions conflict with proposed changes.
```

### Add new decision after consensus:
```
1. Get next TX-ID (increment from last entry)
2. Format new row with current date
3. Append to table in docs/DECISION_LOG.md
```

### Mark decision as implemented:
```
Update Status column from `Active` to `Implemented`
```

---

## 10. Integration with Other Skills

| Related Skill | Interaction |
|---------------|-------------|
| `code_standards` | Code changes must align with Tech Stack decisions |
| `schema_sync` | Database changes must align with Database decisions |
| `frontend_sync` | UI changes must align with UI/UX decisions |
| `req_logger` | Requirement changes may trigger new Decision Log entries |
