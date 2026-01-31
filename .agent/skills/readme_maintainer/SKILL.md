---
name: readme_maintainer
description: Automates the maintenance and updating of README.md files across the codebase.
---

# Readme Maintainer Skill

## 1. Purpose
This skill is designed to automatically keep `README.md` files up-to-date with the actual content of their directories. It ensures that as code evolves, the documentation (folder structure, purpose, key files) remains accurate.

## 2. Usage Strategy
- **Trigger**: Run this skill when:
    - New directories or significant modules are added.
    - Code structure changes significantly.
    - A user explicitly requests "update documentation".
- **Scope**: Can be applied to a specific sub-directory or the entire project root.

## 3. Implementation Logic (Manual Execution for Agent)
When asked to "maintain all readme.md" or similar:

1.  **Identify Target Directories**: Use `list_dir` to find directories effectively.
2.  **Analyze Content**: For each key directory, list files and read the package statement or main comment of key files (e.g., `package main`, `// Package...`).
3.  **Generate/Update README**:
    - **Header**: Use directory name or existing title.
    - **Description**: Summarize the purpose based on the file contents.
    - **Structure**: Create a file tree or table of contents.
    - **Key Components**: List important files and their responsibilities.
4.  **Consistency**: Ensure the format is uniform across folders.

## 4. Standard README Template
```markdown
# [Directory Name]

## Overview
[Brief description of what this module/directory does.]

## Key Files
- `[filename]`: [Description]
- `[filename]`: [Description]

## Usage
[Optional: How to run or import this package]
```

## 5. Workflow (Step-by-Step)
1.  **Scan**: `list_dir` target path.
2.  **Read**: `view_file` existing `README.md` (if any).
3.  **Draft**: Create updated content in memory.
4.  **Write**: `write_to_file` to update `README.md`.
