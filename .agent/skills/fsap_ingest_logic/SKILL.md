---
name: fsap_ingest_logic
description: Logic for "Classified Data Ingestion" handling 10-K/10-Q/8-K hierarchy.
---

# Ingestion Logic Skill: The "Hierarchy of Evidence"

## 1. Trigger
Activate whenever the user uploads financial documents or the system processes a batch of filings.

## 2. Core Rule: The Timeline Split
**Concept**: We treat "Historic Years" differently from the "Current Year".

| Era | Primary Source | Secondary Source | Logic |
| :--- | :--- | :--- | :--- |
| **Historic ( < Current Year - 1)** | **10-K (Annual)** | Ignore 10-Q/8-K | Historic data is settled. We only want the audited final numbers. |
| **Current ( >= Current Year - 1)** | **10-Q (Quarterly)** | **8-K (Events)** | The story is unfolding. Use 10-Q for hard numbers, 8-K for *Assumptions/Soft Signals*. |

## 3. The 8-K Exception
- **Standard**: 8-Ks are treated as "Unstructured Assumption Updates" (e.g., "CEO fired", "New Contract").
- **Exception**: If an 8-K contains a "Press Release" with financial tables *before* the 10-Q is filed, it can be used for *Preliminary Numbers* (marked as distinct status).

## 4. Execution Flow
1.  **Classify**: specificy `filing_type` (10-K, 10-Q, 8-K) and `filing_date`.
2.  **Filter**:
    - IF `year < current_year` AND `type != 10-K` -> **DISCARD/ARCHIVE** (Do not process into FSAP).
    - IF `year == current_year` -> **PROCESS**:
        - 10-Q: Update `fsap_model_versions` (Hard Numbers).
        - 8-K: Update `assumptions` only (Soft Signals).
3.  **Store**: Save to `source_documents` with correct metadata.

## 5. Assumption Layering & Traceability
We strictly separate **Source Evidence** from **User Conviction**.

1.  **Dual Storage**:
    - `extracted_assumptions`: Read-only signals from 8-K/10-Q (e.g., "Mgt guides 15% revenue growth").
    - `user_assumptions`: Editable values that drive the model (e.g., "I believe 12% is safer").
    
2.  **Version Control (Rollback)**:
    - Every change to a parameter (e.g., changing Revenue Growth from 15% -> 12%) is logged as a discrete **Transaction**.
    - The tool must support **"Time Travel"**: Revert the model to how it looked at Step 5.

## 6. Agent Instructions
*"When processing files, calculate `CurrentYear`. If `FilingYear < CurrentYear - 1`, strictly look for 10-K files. For assumptions, NEVER overwrite user inputs. Always store 8-K signals in the `extracted` layer and notify the user of the new signal."*
