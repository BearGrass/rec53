---
name: dev
description: Develop the highest priority task from TODO.md with code, tests, and doc updates.
argument-hint: [task-id or blank for highest priority]
disable-model-invocation: true
---

# Development Workflow

## Steps

1. Read: CLAUDE.md, .rec53/TODO.md, .rec53/ARCHITECTURE.md, .rec53/CONVENTIONS.md
2. Find highest priority "Not started" task (or use $ARGUMENTS if specified)
3. Present plan: files to modify, approach. **Wait for confirmation.**

## Per-File Loop

For each file in the task:
1. Write code changes
2. Write/update unit tests
3. Run `go test -race ./...` on relevant package — fix until pass
4. Update docs if needed:
   - New package/dependency → CLAUDE.md
   - Architecture change → ARCHITECTURE.md
   - User-facing feature → README.md
   - Found issue → TODO.md
5. Report changes. **Wait for confirmation.**

## Task Completion

1. Run full test suite: `go test -race ./...` — must pass
2. Run lint: `go vet ./...` — must have no warnings
3. Mark task completed in TODO.md
4. Move requirement to "Completed" in BACKLOG.md