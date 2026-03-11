---
name: dev
description: Develop tasks from TODO.md with code, tests, and doc updates. Accepts task-id (e.g. F-001) to work the whole task, or task-id/step (e.g. F-001/2) to work a single step.
argument-hint: [task-id | task-id/step | blank for highest priority]
disable-model-invocation: true
---

# Development Workflow

## Argument Parsing

Parse $ARGUMENTS:
- blank → find highest priority task with any "Not started" step
- `F-001` → work task F-001 from its first "Not started" step through all steps
- `F-001/2` → work only step 2 of task F-001

Read: CLAUDE.md, .rec53/TODO.md, .rec53/ARCHITECTURE.md, .rec53/CONVENTIONS.md

**Present target: task title, which step(s) will be worked, files involved. Wait for confirmation.**

## Per-Step Loop

For each step in scope:
1. Mark step `[ ]` → `[~]` (in-progress) in TODO.md
2. Write code changes for that file + unit tests
3. Run `go test ./... -race` on relevant package — fix until pass
4. Update docs if needed: new package → CLAUDE.md, arch change → ARCHITECTURE.md, user feature → README.md, new issue → TODO.md
5. Mark step `[~]` → `[x]` in TODO.md
6. **Report step done. Wait for confirmation before next step.**

## Task Completion

When all steps of a task are `[x]`:
1. Run full test suite: `go test ./... -race` — must pass
2. Run lint: `go vet ./...` — no warnings
3. Mark task `[ ]` → `[x]` in TODO.md
4. Move requirement to "Completed" in BACKLOG.md
