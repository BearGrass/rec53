---
name: plan
description: Analyze requirements from BACKLOG.md, decompose into development tasks in TODO.md. Use when new requirements need technical analysis and task breakdown.
argument-hint: [requirement-id or blank for all]
disable-model-invocation: true
---

# Requirement Planning

1. Read: CLAUDE.md, .rec53/BACKLOG.md, .rec53/ARCHITECTURE.md, .rec53/TODO.md, .rec53/ROADMAP.md
2. Find "Unplanned" items (or filter by $ARGUMENTS if specified)
3. For each item, analyze: files to modify, new files needed, external deps, architecture conflicts, complexity (S/M/L), suggested order
4. **Present analysis. Wait for confirmation.**
5. After confirmation: move items to "Planned" in BACKLOG.md, write tasks to TODO.md, update ARCHITECTURE.md/ROADMAP.md if affected

## TODO.md Task Format

Each requirement becomes one task entry with numbered steps:

```
- [ ] [F-001] Title (source: BACKLOG.md)
  - [ ] [F-001/1] Create src/foo.go — implement X
  - [ ] [F-001/2] Update src/bar.go — add Y to Z
  - [ ] [F-001/3] Write tests for foo.go
```

Rules:
- Every step must reference a specific file
- Steps ordered by dependency (files depended upon come first)
- No code in this phase. Ask if ambiguous. State conflicts clearly.
