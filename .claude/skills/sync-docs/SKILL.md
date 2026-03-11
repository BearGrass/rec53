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
   - .rec53/TODO.md: completed items marked? new issues?
   - .rec53/TEST_PLAN.md: statuses current?
   - .rec53/ARCHITECTURE.md: descriptions accurate?
   - .rec53/BACKLOG.md: completed items moved?
4. List discrepancies: which doc, which section, what's wrong, what it should say
5. **Wait for confirmation**
6. Apply fixes, report updates