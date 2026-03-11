---
name: test
description: Systematically improve test coverage in dependency order.
argument-hint: [package-path or blank for full project]
disable-model-invocation: true
---

# Test Coverage Workflow

## Phase 1: Planning

1. Read CLAUDE.md, .rec53/TEST_PLAN.md
2. Scan source files, compare against existing tests
3. Run `go test -coverprofile=coverage.out ./...` for baseline
4. Update TEST_PLAN.md sorted by dependency: utilities → core → interfaces → integration
5. Present plan. **Wait for confirmation.**

## Phase 2: Execution

For each file:
1. Write test file following conventions
2. Run `go test -race ./...` — fix until pass
3. Update TEST_PLAN.md status
4. Report. **Wait for confirmation before next file.**

After each batch:
1. Run `go test -coverprofile=coverage.out ./...`, report delta
2. **Wait for instruction to continue**

## Phase 3: Wrap-up

1. Full test suite pass
2. Final coverage report
3. Update TEST_PLAN.md, CLAUDE.md, TODO.md