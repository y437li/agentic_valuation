---
name: schema_sync
description: Keeps the Schema Design document in sync with the actual Supabase/SQL implementation.
---

# Schema Sync Skill

## 1. Trigger
Activate this skill whenever:
- User asks to "Create a table", "Add a column", or "Modify a relationship".
- You edit `schema/supabase_schema.sql` or any `.sql` migration file.
- You discuss changes to the Data Architecture (e.g., "Let's store embeddings").

## 2. Action
You must **UPDATE** the Schema Design document at:
**`.agent/resources/SCHEMA_DESIGN.md`**

## 3. Instructions
1.  **Analyze** the change: What table/feature was added or modified?
2.  **Edit** `SCHEMA_DESIGN.md`:
    - If a **Table** changed: Update its "Table Specifications" section.
    - If a **Concept** changed (e.g., adding Vector): Update the "Core Architecture" summary.
3.  **Consistency Check**: Ensure the Markdown description matches the SQL reality 1:1.
