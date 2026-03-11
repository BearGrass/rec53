## Document Self-Maintenance Rules

### CLAUDE.md Update Triggers

- Added a package or directory → update Architecture section
- Added an external dependency → update Dependencies section
- Changed interfaces or test strategy → update Testing section
- Added commands or build steps → update Build & Run section
- Found CLAUDE.md description inconsistent with code → fix immediately

### README.md Update Triggers

- Added user-facing feature → update feature list
- Changed config format or CLI flags → update usage instructions
- Changed build requirements → update install steps
- Version number changed → update version badge

### Documentation Directory Update Triggers

The documentation directory is configurable (default: `.docs`).

### TODO.md Update Triggers

- Completed a task → mark as done and move to Completed section
- Found a new bug → add BUG item with discovery context
- Found optimization opportunity outside current task → add OPT item
- Task interrupted → update progress checkboxes to latest state
- Introduced technical debt → add DEBT item with source reference

### BACKLOG.md Update Triggers

- During development, discovered a prerequisite feature is needed → add to Unplanned
- Requirement development complete → move from Planned to Completed

### Execution Rules

- Do NOT make a separate round just to update docs. Update in the same task that caused the change.
- After updating, mention what changed in one line, e.g.: "Updated CLAUDE.md Architecture: added internal/middleware/"