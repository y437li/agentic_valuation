---
name: frontend_sync
description: Keeps the Frontend Design document in sync with the actual Code Architecture using `web-ui` changes.
---

# Frontend Sync Skill

## 1. Trigger
Activate this skill whenever:
- You edit `web-ui/src/lib/branding.ts` (Design Tokens).
- You create new high-level Components or Layouts in `web-ui/`.
- You modify `globals.css` significantly.
- You introduce a new Library (e.g., "Adding Framer Motion").

## 2. Action
You must **UPDATE** the Frontend Design document at:
**`.agent/resources/FRONTEND_DESIGN.md`**

## 3. Instructions
1.  **Reflect** the change:
    - Did we change the **Color Palette**? Update the "Design System" section.
    - Did we add a new **Zone** (e.g., "History Panel")? Update "Component Architecture".
2.  **Verify**: Ensure the document accurately describes the *current* state of the codebase, not just the aspiration.
