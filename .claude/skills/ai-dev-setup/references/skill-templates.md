# Skill Templates

Generated into `.claude/skills/` for project-specific workflows.

## Placeholder Substitution

| Placeholder | Example |
|-------------|---------|
| `{DOC_DIR}` | `.docs`, `docs` |
| `{TEST_CMD}` | `go test ./...` |
| `{LINT_CMD}` | `go vet ./...` |
| `{TEST_COVER_CMD}` | `go test -coverprofile=coverage.out ./...` |
| `{RACE_FLAG}` | `-race` or empty |

## Directory Structure

```
.claude/skills/
├── plan/SKILL.md
├── dev/SKILL.md
├── dev-resume/SKILL.md
├── test/SKILL.md
├── test-resume/SKILL.md
└── sync-docs/SKILL.md
```

---

## /plan

```markdown
---
name: plan
description: Analyze requirements from BACKLOG.md, decompose into development tasks in TODO.md. Use when new requirements need technical analysis and task breakdown.
argument-hint: [requirement-id or blank for all]
disable-model-invocation: true
---

# Requirement Planning

1. Read: CLAUDE.md, {DOC_DIR}/BACKLOG.md, {DOC_DIR}/ARCHITECTURE.md, {DOC_DIR}/TODO.md, {DOC_DIR}/ROADMAP.md
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
```

## /dev

```markdown
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

Read: CLAUDE.md, {DOC_DIR}/TODO.md, {DOC_DIR}/ARCHITECTURE.md, {DOC_DIR}/CONVENTIONS.md

**Present target: task title, which step(s) will be worked, files involved. Wait for confirmation.**

## Per-Step Loop

For each step in scope:
1. Mark step `[ ]` → `[~]` (in-progress) in TODO.md
2. Write code changes for that file + unit tests
3. Run `{TEST_CMD} {RACE_FLAG}` on relevant package — fix until pass
4. Update docs if needed: new package → CLAUDE.md, arch change → ARCHITECTURE.md, user feature → README.md, new issue → TODO.md
5. Mark step `[~]` → `[x]` in TODO.md
6. **Report step done. Wait for confirmation before next step.**

## Task Completion

When all steps of a task are `[x]`:
1. Run full test suite: `{TEST_CMD} {RACE_FLAG}` — must pass
2. Run lint: `{LINT_CMD}` — no warnings
3. Mark task `[ ]` → `[x]` in TODO.md
4. Move requirement to "Completed" in BACKLOG.md
```

## /dev-resume

```markdown
---
name: dev-resume
description: Resume an interrupted development session by finding the first in-progress or incomplete step in TODO.md.
disable-model-invocation: true
---

# Resume Development

1. Read: CLAUDE.md, {DOC_DIR}/TODO.md, {DOC_DIR}/BACKLOG.md
2. Find the first task containing a `[~]` (in-progress) or the first task with mixed `[x]`/`[ ]` steps
3. Report: task title, steps completed `[x]`, current step `[~]` or next `[ ]`, steps remaining
4. **Wait for confirmation, then continue per /dev per-step loop rules starting from the identified step**
```

## /test

```markdown
---
name: test
description: Systematically improve test coverage in dependency order.
argument-hint: [package-path or blank for full project]
disable-model-invocation: true
---

# Test Coverage Workflow

Phase 1 — Planning:
1. Read: CLAUDE.md, {DOC_DIR}/TEST_PLAN.md
2. Scan source files vs existing tests, run `{TEST_COVER_CMD}` for baseline
3. Update TEST_PLAN.md: utilities → core → interfaces → integration order
4. **Present plan. Wait for confirmation.**

Phase 2 — Execution (per file):
1. Write test file per conventions
2. Run `{TEST_CMD} {RACE_FLAG}` — fix until pass
3. Update TEST_PLAN.md status, report
4. **Wait for confirmation before next file**
After each batch: run `{TEST_COVER_CMD}`, report delta, **wait for instruction**

Phase 3 — Wrap-up: full suite pass, final coverage report, update TEST_PLAN.md + CLAUDE.md + TODO.md
```

## /test-resume

```markdown
---
name: test-resume
description: Resume an interrupted test coverage session by reading TEST_PLAN.md for pending items.
disable-model-invocation: true
---

# Resume Test Workflow

1. Read: CLAUDE.md, {DOC_DIR}/TEST_PLAN.md, {DOC_DIR}/TODO.md
2. Find first "Not started" or "In Progress" entry
3. Report: X completed, Y remaining, current batch
4. **Wait for confirmation, then continue per /test Phase 2 rules**
```

## /sync-docs

```markdown
---
name: sync-docs
description: Check docs against code and fix inconsistencies.
disable-model-invocation: true
---

# Documentation Sync

1. Read CLAUDE.md "Document Self-Maintenance Rules" section
2. Run `git diff --name-only` (or `HEAD~5` if no staged changes)
3. Check each modified file against trigger rules in CLAUDE.md
4. List discrepancies: doc, section, current state, correct state
5. **Wait for confirmation**
6. Apply fixes, report updates
```
