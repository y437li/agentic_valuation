---
name: req_logger
description: Automates the logging of requirement changes into a centralized log.
---

# Requirement Logger Skill

## 1. Trigger
Activate this skill whenever:
- The user explicitly requested a **new feature**.
- The user **modifies** an existing requirement.
- The user **removes** a planned feature.
- Scope changes are discussed (e.g., "Let's move this to Phase 2").

## 2. Action
You must **APPEND** a new entry to the Requirement Log at:
**`Docs/REQUIREMENT_LOG.md`**

## 3. Log Format
Use the following Markdown table row structure:

| ID | Date | Type | Requirement Description | Origin | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `REQ-001` | `YYYY-MM-DD` | `New` / `Modify` | Brief description of the requirement. | `User Chat` / `Doc` | `Pending` |

## 4. Instructions
1.  **Read** the current `Docs/REQUIREMENT_LOG.md` to determine the next ID (e.g., if REQ-005 exists, use REQ-006).
2.  **Append** the new row to the table.
3.  **Do not** overwrite existing history. This is an append-only log.
