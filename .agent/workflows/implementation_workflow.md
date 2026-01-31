---
description: Standard workflow for implementing new features or functions.
---

# Structured Implementation Workflow

This workflow enforces a strict "Read First, Code Second" protocol to maintain architectural integrity.

## Step 1: Ground Yourself
**Read the Project Map**
First, understand where you are in the codebase.
- **Action**: Read the project structure documentation.
- **Tool**: `view_file`
- **Target**: `.agent/resources/PROJECT_STRUCTURE.md`

## Step 2: Impact Analysis
**Identify & Learn**
Decide which files are relevant to your task and read them to understand existing patterns.
- **Action**: List relevant directories and view specific files.
- **Tool**: `list_dir`, `view_file`
- **Questions to Answer**:
    1. Where should the new code live?
    2. What existing models or utilities can I reuse?
    3. What are the established patterns for this module (naming, error handling, imports)?

## Step 3: Implementation
**Execute**
Once you have the context, proceed with the changes.
- **Action**: Create or modify files.
- **Tool**: `write_to_file`, `replace_file_content`


## 4. Documentation

**Purpose**: Ensures all code changes are architecturally sound and documented.

**Process**:
1.  **Ground Yourself**: Read `.agent/resources/PROJECT_STRUCTURE.md` to map the territory.
2.  **Impact Analysis**: `list_dir` and `view_file` to understand relevant existing code.
3.  **Implementation**: Write the actual code.
4.  **Documentation Sync**: Update logic, schema, or structure documentation to match code.
5. **Target**:`README.MD`,`.docs/REQUIREMENT_LOG.md`,`.docs/DECISION_LOG.md`,`.agent/resources/PROJECT_STRUCTURE.md`

## 5. Package Registry

**Purpose**: Track all external dependencies and tools.

**Rule**: When adding a new package or tool, you MUST update the Package Registry.

**Process**:
1.  **Go Package**: After `go get <package>`, update `docs/PACKAGE_REGISTRY.md`
2.  **NPM Package**: After `npm install <package>`, update `docs/PACKAGE_REGISTRY.md`
3.  **CLI Tool**: After installing external tool (e.g., Pandoc), update `docs/PACKAGE_REGISTRY.md`

**Target**: `docs/PACKAGE_REGISTRY.md`

## 6. Agent Registry

**Purpose**: Maintain a centralized registry of all AI agents in the system.

**Rule**: When adding, modifying, or removing an agent, you MUST update the Agent Registry.

**Triggers**:
- Creating a new agent struct (e.g., `type NewAgent struct`)
- Adding a new `AgentRole` constant
- Modifying agent prompts in `prompts.go`
- Changing data sources an agent uses

**Process**:
1.  Read `docs/AGENT_REGISTRY.md` to understand current agent inventory
2.  After implementation, update the relevant table:
    - **Debate Agents**: Add role, description, data sources
    - **Extraction Agents**: Add section, purpose
    - **Pipeline Agents**: Add trigger, purpose
3.  Update the Data Sources Matrix if new data sources are integrated

**Target**: `docs/AGENT_REGISTRY.md`

## 7. Interaction Flows

**Purpose**: Maintain documentation of all user interaction logic and UX flows.

**Rule**: When adding or modifying user-facing interactions, you MUST update the Interaction Flows document.

**Triggers**:
- Adding new user-triggered actions (buttons, toggles)
- Modifying workflow sequences (e.g., Case entry flow)
- Adding background processes with user notifications
- Implementing diff/review mechanics

**Process**:
1.  Read `docs/INTERACTION_FLOWS.md` to understand current documented flows
2.  After implementation, update the relevant section:
    - **Case Lifecycle**: Entry, creation, loading flows
    - **Roundtable Debate Flow**: Stages, triggers, outputs
    - **Assumption Management**: Apply/Remove, Diff Review
    - **Filing Update Flow**: Background monitoring, notifications
3.  Include ASCII diagrams for complex flows

**Target**: `docs/INTERACTION_FLOWS.md`
