# Skill Templates

Generated into `.claude/skills/` for project-specific workflows.

## Placeholder Substitution

| Placeholder | Description | Example |
|-------------|-------------|---------|
| `{DOC_DIR}` | Documentation directory | `.rec53`, `docs` |
| `{TEST_CMD}` | Test command | `go test ./...` |
| `{LINT_CMD}` | Lint command | `go vet ./...` |
| `{TEST_COVER_CMD}` | Coverage command | `go test -coverprofile=coverage.out ./...` |
| `{RACE_FLAG}` | Race detection flag | `-race` or empty |

## /plan - Requirement Planning

**Target:** `.claude/skills/plan/SKILL.md`

```markdown
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
   - {DOC_DIR}/BACKLOG.md
   - {DOC_DIR}/ARCHITECTURE.md
   - {DOC_DIR}/TODO.md
   - {DOC_DIR}/ROADMAP.md

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
```

## /dev - Development Workflow

**Target:** `.claude/skills/dev/SKILL.md`

```markdown
---
name: dev
description: Develop the highest priority task from TODO.md with code, tests, and doc updates.
argument-hint: [task-id or blank for highest priority]
disable-model-invocation: true
---

# Development Workflow

## Steps

1. Read: CLAUDE.md, {DOC_DIR}/TODO.md, {DOC_DIR}/ARCHITECTURE.md, {DOC_DIR}/CONVENTIONS.md
2. Find highest priority "Not started" task (or use $ARGUMENTS if specified)
3. Present plan: files to modify, approach. **Wait for confirmation.**

## Per-File Loop

For each file in the task:
1. Write code changes
2. Write/update unit tests
3. Run `{TEST_CMD} {RACE_FLAG}` on relevant package — fix until pass
4. Update docs if needed:
   - New package/dependency → CLAUDE.md
   - Architecture change → ARCHITECTURE.md
   - User-facing feature → README.md
   - Found issue → TODO.md
5. Report changes. **Wait for confirmation.**

## Task Completion

1. Run full test suite: `{TEST_CMD} {RACE_FLAG}` — must pass
2. Run lint: `{LINT_CMD}` — must have no warnings
3. Mark task completed in TODO.md
4. Move requirement to "Completed" in BACKLOG.md
```

## /dev-resume - Resume Development

**Target:** `.claude/skills/dev-resume/SKILL.md`

```markdown
---
name: dev-resume
description: Resume an interrupted development session by reading TODO.md for in-progress tasks.
disable-model-invocation: true
---

# Resume Development Workflow

1. Read CLAUDE.md, {DOC_DIR}/TODO.md, {DOC_DIR}/BACKLOG.md
2. Find tasks with status "In Progress" in TODO.md
3. Report current progress: which task, which files are done, what remains
4. **Wait for user confirmation before continuing**

After confirmation, continue following the same rules as /dev: per-file loop with self-check, wait for confirmation after each file.
```

## /test - Test Coverage Workflow

**Target:** `.claude/skills/test/SKILL.md`

```markdown
---
name: test
description: Systematically improve test coverage in dependency order.
argument-hint: [package-path or blank for full project]
disable-model-invocation: true
---

# Test Coverage Workflow

## Phase 1: Planning

1. Read CLAUDE.md, {DOC_DIR}/TEST_PLAN.md
2. Scan source files, compare against existing tests
3. Run `{TEST_COVER_CMD}` for baseline
4. Update TEST_PLAN.md sorted by dependency: utilities → core → interfaces → integration
5. Present plan. **Wait for confirmation.**

## Phase 2: Execution

For each file:
1. Write test file following conventions
2. Run `{TEST_CMD} {RACE_FLAG}` — fix until pass
3. Update TEST_PLAN.md status
4. Report. **Wait for confirmation before next file.**

After each batch:
1. Run `{TEST_COVER_CMD}`, report delta
2. **Wait for instruction to continue**

## Phase 3: Wrap-up

1. Full test suite pass
2. Final coverage report
3. Update TEST_PLAN.md, CLAUDE.md, TODO.md
```

## /test-resume - Resume Testing

**Target:** `.claude/skills/test-resume/SKILL.md`

```markdown
---
name: test-resume
description: Resume an interrupted test coverage session by reading TEST_PLAN.md for pending items.
disable-model-invocation: true
---

# Resume Test Workflow

1. Read CLAUDE.md, {DOC_DIR}/TEST_PLAN.md, {DOC_DIR}/TODO.md
2. Find the first entry with status "Not started" or "In Progress"
3. Report progress: X completed, Y remaining, current batch Z
4. **Wait for confirmation before continuing**

After confirmation, continue following /test Phase 2 rules: per-file loop with self-check, wait for confirmation.
```

## /sync-docs - Documentation Sync

**Target:** `.claude/skills/sync-docs/SKILL.md`

```markdown
---
name: sync-docs
description: Check docs against code and fix inconsistencies.
disable-model-invocation: true
---

# Documentation Sync

1. Read CLAUDE.md "Document Self-Maintenance Rules" section
2. Run `git diff --name-only` (or `HEAD~5` if no staged changes)
3. Check each modified file against trigger rules:
   - CLAUDE.md: Architecture, Dependencies, Testing, Build sections?
   - README.md: features, usage, install up to date?
   - {DOC_DIR}/TODO.md: completed items marked? new issues?
   - {DOC_DIR}/TEST_PLAN.md: statuses current?
   - {DOC_DIR}/ARCHITECTURE.md: descriptions accurate?
   - {DOC_DIR}/BACKLOG.md: completed items moved?
4. List discrepancies: which doc, which section, what's wrong, what it should say
5. **Wait for confirmation**
6. Apply fixes, report updates
```

## Directory Creation

Each skill needs its own directory:

```
.claude/skills/
├── plan/
│   └── SKILL.md
├── dev/
│   └── SKILL.md
├── dev-resume/
│   └── SKILL.md
├── test/
│   └── SKILL.md
├── test-resume/
│   └── SKILL.md
└── sync-docs/
    └── SKILL.md
```

Create directories if they don't exist before writing files.