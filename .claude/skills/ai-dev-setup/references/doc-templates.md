# Documentation Templates

All files generated in `DOC_DIR`. Keep each file under ~100 lines — link to code, don't quote it.

## Content Rules

1. Never fabricate — all content from actual code
2. Use the project's actual language for examples (not Go)
3. Mark unknowns as `__%` or `[TBD]`
4. Use actual file paths — don't invent directories

---

## README.md — Index

Table of all docs: File | Purpose | Update Frequency. Quick links section.

## ARCHITECTURE.md — Structure

Sections: Overview (1 paragraph), Directory Structure (tree from actual ls), Core Flow (ASCII diagram if applicable), Key Components (name + responsibility + interface + deps), Design Constraints, Known Limitations.

## CONVENTIONS.md — Patterns

Sections: Language Style (link to official guide), Naming Conventions (table: Element | Convention | Example — from actual code), Error Handling (pattern from code), Logging (pattern from code), Testing Patterns, Code Review Checklist.

## ROADMAP.md — History & Plans

Sections: Version History (table: Version | Date | Highlights — from git tags), Current Version features + known issues, Next Version planned items, Future long-term goals.

## TODO.md — Tasks

```markdown
## In Progress
(none)

## Backlog
### Feature
<!-- - [ ] [F-001] Title (source: BACKLOG.md)
  - [ ] [F-001/1] Create src/foo.go — implement X
  - [ ] [F-001/2] Update src/bar.go — add Y
  - [ ] [F-001/3] Write tests for foo.go -->

### BUG
<!-- - [ ] [B-001] description (file:line)
  - [ ] [B-001/1] Fix src/foo.go line 42
  - [ ] [B-001/2] Add regression test -->

### Optimization
<!-- - [ ] [O-001] description -->

### Technical Debt
<!-- - [ ] [D-001] description (source) -->

## Completed
<!-- - [x] [F-001] Title (completed YYYY-MM-DD) -->
```

Task ID prefixes: `B-xxx` Bug, `O-xxx` Optimization, `D-xxx` Tech Debt, `F-xxx` Feature.
Step IDs: `F-001/1`, `F-001/2`, ... — addressable by `/dev F-001/2`.
Step states: `[ ]` not started, `[~]` in-progress, `[x]` done.

## TEST_PLAN.md — Coverage

Sections: Overview (baseline %, target 80%, date), Batch Schedule ordered by dependency:
1. Foundation/Utility — no deps
2. Core Logic — depends on foundation
3. Interface/Handler — depends on core
4. Integration — full stack

Each batch: table of Source file | Test file | Key test points | Status.

## BACKLOG.md — Requirements

Template format:
```
### [F-xxx] Title
Priority: High/Medium/Low
Description: 1-2 sentences
Acceptance criteria:
- Criterion 1
```

Sections: Unplanned (user writes here), Planned (moved by /plan), Completed (moved by /dev).
