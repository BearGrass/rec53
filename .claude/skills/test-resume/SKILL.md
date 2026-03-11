---
name: test-resume
description: Resume an interrupted test coverage session by reading TEST_PLAN.md for pending items.
disable-model-invocation: true
---

# Resume Test Workflow

1. Read CLAUDE.md, .rec53/TEST_PLAN.md, .rec53/TODO.md
2. Find the first entry with status "Not started" or "In Progress"
3. Report progress: X completed, Y remaining, current batch Z
4. **Wait for confirmation before continuing**

After confirmation, continue following /test Phase 2 rules: per-file loop with self-check, wait for confirmation.