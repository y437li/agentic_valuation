# Multi-Agent Onboarding & Coordination Skill

This skill defines how the `.agent/skills` directory acts as the "Standard Operating Procedure" (SOP) library for all AI agents (Gemini, GPT, Local LLMs) interacting with the TIED platform.

## 1. Skills as Shared Intelligence
Our skills are stored in `.md` format specifically so they are **Model-Agnostic**.
- **The Concept**: Any "New Agent" added to the app should first be pointed to the `.agent/skills/` directory.
- **The Protocol**: Before any model performs a task, it must ingest the relevant `SKILL.md` to ensure its output complies with TIED's "Constitution".

---

## 2. Onboarding Workflow for New Models
When you call a new model (e.g., via Python/Node.js) to perform a specific valuation task:

### Step 1: Context Injection
- **Action**: Read the relevant `SKILL.md` file from the filesystem.
- **Implementation**: Prepend the content of the skill to the model's `System Prompt`.
- *Example*: If calling a model to parse a PDF, inject `fsap_ingest_logic/SKILL.md`.

### Step 2: Protocol Compliance
The new model must explicitly acknowledge the "TIED" principles:
1. **Explainability**: "I must provide reasoning for every number."
2. **Integrity**: "I must validate my JSON using the `json_integrity` logic."
3. **Traceability**: "I must cite source coordinates."

---

## 3. Coordinating "Specialist" Agents
| Role | Recommended Skill Usage |
| :--- | :--- |
| **Accountant Agent** | `fsap_ingest_logic`, `json_integrity` |
| **Software Architect Agent** | `code_standards`, `schema_sync`, `auto_test_gen` |
| **Product Manager Agent** | `req_logger`, `decision_log` |

## 4. Benefit: "Vibe Continuity"
By using these shared skills, you prevent "Model Drift." Even if Model A (Analyst) and Model B (Validator) are different LLMs, they will speak the same "TIED language," use the same directory structures, and respect the same financial constraints.
