---
name: test
description: Systematically improve test coverage in dependency order.
argument-hint: [package-path or blank for full project]
disable-model-invocation: true
---

# Test Coverage Workflow

Phase 1 — Planning:
1. Read: CLAUDE.md, .rec53/TEST_PLAN.md
2. Scan source files vs existing tests, run `go test -cover ./...` for baseline
3. Update TEST_PLAN.md: utilities → core → interfaces → integration order
4. **Present plan. Wait for confirmation.**

Phase 2 — Execution (per file):
1. Write test file per conventions
2. Run `go test ./... -race` — fix until pass
3. Update TEST_PLAN.md status, report
4. **Wait for confirmation before next file**
After each batch: run `go test -cover ./...`, report delta, **wait for instruction**

Phase 3 — Wrap-up: full suite pass, final coverage report, update TEST_PLAN.md + CLAUDE.md + TODO.md
