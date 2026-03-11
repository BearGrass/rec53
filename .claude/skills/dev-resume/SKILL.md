---
name: dev-resume
description: Resume an interrupted development session by reading TODO.md for in-progress tasks.
disable-model-invocation: true
---

# Resume Development Workflow

1. Read CLAUDE.md, .rec53/TODO.md, .rec53/BACKLOG.md
2. Find tasks with status "In Progress" in TODO.md
3. Report current progress: which task, which files are done, what remains
4. **Wait for user confirmation before continuing**

After confirmation, continue following the same rules as /dev: per-file loop with self-check, wait for confirmation after each file.