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
