---
name: dev-resume
description: Resume an interrupted development session by finding the first in-progress or incomplete step in TODO.md.
disable-model-invocation: true
---

# Resume Development

1. Read: CLAUDE.md, .rec53/TODO.md, .rec53/BACKLOG.md
2. Find the first task containing a `[~]` (in-progress) or the first task with mixed `[x]`/`[ ]` steps
3. Report: task title, steps completed `[x]`, current step `[~]` or next `[ ]`, steps remaining
4. **Wait for confirmation, then continue per /dev per-step loop rules starting from the identified step**
