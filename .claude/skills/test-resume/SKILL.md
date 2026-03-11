---
name: test-resume
description: Resume an interrupted test coverage session by reading TEST_PLAN.md for pending items.
disable-model-invocation: true
---

# Resume Test Workflow

1. Read: CLAUDE.md, .rec53/TEST_PLAN.md, .rec53/TODO.md
2. Find first "Not started" or "In Progress" entry
3. Report: X completed, Y remaining, current batch
4. **Wait for confirmation, then continue per /test Phase 2 rules**
