---
name: auto_test_gen
description: Automates the creation of test cases and function checks for new code.
---

# Auto Test Generation Skill

## 1. Trigger
Activate this skill whenever:
- You create a new **Calculate Function** (e.g., "Calculate EBITDA").
- You write a **Parsing Logic** (e.g., "Extract Table from PDF").
- You implement a **UI Component** with complex logic.
- The user explicitly asks to "Add tests".

## 2. Action
You must generate a **Test Suite** to verify the function works as expected.

## 3. Storage Rules ("Store in Test")

### A. Backend (Go) - Functional Checks
- **Location**: `tests/backend/` or `pkg/.../*_test.go` (if Unit Test).
- **Naming**: `[feature]_test.go`
- **Content**:
    - Use "Table-Driven Tests" (inputs -> expected outputs).
    - Cover: Normal Case, Edge Case (Null/Zero), Error Case.

### B. Frontend (TSX) - Component Checks
- **Location**: `tests/frontend/` or `__tests__` folders.
- **Naming**: `[Component].test.tsx`
- **Content**:
    - Use `jest` / `react-testing-library` patterns (simulated).
    - Check: Rendering, User Clicks, Data Prop Changes.

### C. Logic / Scenario Cases (The "Test Plan")
- **Location**: `tests/scenarios/[feature].md`
- **Format**:
    ```markdown
    # Test Case: [Feature Name]
    - [ ] Input: Revenue = 100, COGS = 50 -> Expect Gross Profit = 50
    - [ ] Input: Revenue = 0 -> Expect ZeroDivisionError handled nicely
    ```

## 4. Instructions
1.  **Analyze** the code you just wrote. What are the inputs and outputs?
2.  **Generate** distinct test cases (Happy Path + Edge Cases).
3.  **Write** the test code or scenario file to the `tests/` directory (or co-located).
4.  **Verify**: If possible, run the test to ensure it passes.
