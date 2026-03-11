---
name: plan
description: Analyze requirements from BACKLOG.md, decompose into development tasks in TODO.md. Use when new requirements need technical analysis and task breakdown.
argument-hint: [requirement-id or blank for all]
disable-model-invocation: true
---

# Requirement Planning Workflow

## Steps

1. Read these files to build full project context:
   - CLAUDE.md
   - .rec53/BACKLOG.md
   - .rec53/ARCHITECTURE.md
   - .rec53/TODO.md
   - .rec53/ROADMAP.md

2. Find all items under "Unplanned" in BACKLOG.md. If the user specified a requirement ID in $ARGUMENTS, process only that one.

3. For each item, produce a technical analysis:
   - Which existing files need modification
   - Which new files need to be created
   - External library dependencies (if any)
   - Conflicts with existing architecture (if any)
   - Estimated complexity: Small / Medium / Large
   - Suggested implementation order considering dependencies

4. Present the analysis. **Wait for user confirmation.**

5. After confirmation:
   - Move confirmed items from "Unplanned" to "Planned" in BACKLOG.md
   - Create specific tasks in TODO.md for each requirement, broken down to file level
   - If requirement affects architecture → update ARCHITECTURE.md
   - If requirement belongs to a new version → update ROADMAP.md
   - Update CLAUDE.md if any section is affected

6. Tell the user: ready to start with /dev

## Rules

- Do NOT write any code in this phase
- If a requirement is ambiguous, list your questions and ask the user. Do not assume.
- If a requirement conflicts with existing functionality, state the conflict clearly and let the user decide.