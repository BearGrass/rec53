# Documentation Templates

All documentation files are generated in `DOC_DIR` (default: `.docs` or user-specified).

## DOC_DIR/README.md

Documentation index with table format:

```markdown
# Documentation Index

| File | Purpose | Update Frequency |
|------|---------|------------------|
| ARCHITECTURE.md | System architecture overview | When structure changes |
| CONVENTIONS.md | Coding style and patterns | When conventions evolve |
| ROADMAP.md | Version history and planning | Per release |
| TODO.md | Active tasks and issues | During development |
| TEST_PLAN.md | Test coverage planning | When adding tests |
| BACKLOG.md | Requirements backlog | During planning |

## Quick Links

- [Architecture](./ARCHITECTURE.md)
- [Conventions](./CONVENTIONS.md)
- [Roadmap](./ROADMAP.md)
- [TODO](./TODO.md)
- [Test Plan](./TEST_PLAN.md)
- [Backlog](./BACKLOG.md)
```

## DOC_DIR/ARCHITECTURE.md

```markdown
# Architecture

## Overview

[One paragraph describing what the project does and its primary purpose.]

## Directory Structure

```
project-root/
├── [src or main directory]    # Source code
│   ├── [component 1]          # Component description
│   └── [component 2]          # Component description
├── [tests directory]          # Test files
├── [config directory]         # Configuration files
└── [docs directory]           # Documentation
```

Note: Adjust structure to match actual project layout.

## Core Flow

[ASCII diagram of main data/request flow if applicable]

Example patterns by architecture:
- Web API: Request → Handler → Service → Repository → Database
- CLI: Input → Parser → Processor → Output
- Library: Public API → Internal Logic → External Dependencies

## Key Components

### [Component 1 Name]

- **Responsibility**: [What it does]
- **Interface**: [Key exported functions/types]
- **Dependencies**: [What it depends on]

### [Component 2 Name]

...

## Design Constraints

- [Constraint 1: e.g., Must handle 10k req/sec]
- [Constraint 2: e.g., Single binary deployment]

## Known Limitations

- [Limitation 1]
- [Limitation 2]
```

## DOC_DIR/CONVENTIONS.md

```markdown
# Coding Conventions

## Language Style

[Reference to official style guide for the detected language]

## Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| [Extract from actual code] | | |
| [Example: Packages] | [Convention: lowercase] | `handlers`, `services` |
| [Example: Types/Classes] | [Convention: PascalCase] | `UserService`, `HTTPRequest` |

## Error Handling

[Pattern extracted from actual code - use project's language]

## Logging

[Logging pattern from actual code - use project's language]

## Testing Patterns

[Test patterns from actual code - use project's language]

## Code Review Checklist

- [ ] Error messages are descriptive
- [ ] Logs include relevant context
- [ ] Tests cover edge cases
- [ ] [Project-specific item from code]
```

## DOC_DIR/ROADMAP.md

```markdown
# Roadmap

## Version History

| Version | Date | Highlights |
|---------|------|------------|
| v1.0.0 | YYYY-MM-DD | Initial release |
| v1.1.0 | YYYY-MM-DD | Added feature X |

## Current Version: vX.Y.Z

### Features
- [Feature 1]
- [Feature 2]

### Known Issues
- [Issue 1]

## Next Version: vX.Y+1.Z

### Planned
- [ ] [Feature A]
- [ ] [Feature B]

### Under Consideration
- [Feature C] - [Brief description]

## Future

### Long-term Goals
- [Goal 1]
- [Goal 2]
```

## DOC_DIR/TODO.md

```markdown
# Task Management

## In Progress

(none)

## Backlog

### BUG
<!-- Format: - [ ] [B-001] description (file:line) -->

### Optimization
<!-- Format: - [ ] [O-001] description -->

### Technical Debt
<!-- Format: - [ ] [D-001] description (source) -->

## Completed
<!-- Move completed items here with completion date -->
<!-- Format: - [x] [B-001] description (completed YYYY-MM-DD) -->
```

### Task ID Convention

| Prefix | Type | Example |
|--------|------|---------|
| `B-xxx` | Bug | `B-001` |
| `O-xxx` | Optimization | `O-001` |
| `D-xxx` | Technical Debt | `D-001` |
| `F-xxx` | Feature (from backlog) | `F-001` |

## DOC_DIR/TEST_PLAN.md

```markdown
# Test Plan

## Overview

- Coverage baseline: __%
- Coverage target: 80%
- Last updated: YYYY-MM-DD

## Batch Schedule

Tests are organized by dependency order:
1. Foundation/Utility Layer - No dependencies
2. Core Logic Layer - Depends on foundation
3. Interface/Handler Layer - Depends on core
4. Integration - Full stack tests

### Batch 1: Foundation/Utility Layer

| Source file | Test file | Key test points | Status |
|-------------|-----------|-----------------|--------|
| [Source file path] | [Test file path] | Edge cases, error handling | Not started |

### Batch 2: Core Logic Layer

| Source file | Test file | Key test points | Status |
|-------------|-----------|-----------------|--------|
| | | | Not started |

### Batch 3: Interface/Handler Layer

| Source file | Test file | Key test points | Status |
|-------------|-----------|-----------------|--------|
| | | | Not started |

### Batch 4: Integration

| Source file | Test file | Key test points | Status |
|-------------|-----------|-----------------|--------|
| | | | Not started |
```

## DOC_DIR/BACKLOG.md

```markdown
# Requirement Backlog

## Template

Use this format for each requirement:

> ### [F-xxx] Title
> Priority: High / Medium / Low
> Description: What is needed in 1-2 sentences
> Acceptance criteria:
> - Criterion 1
> - Criterion 2

Use these prefixes:
- `[F-xxx]` for features
- `[B-xxx]` for bugs
- `[O-xxx]` for optimizations

## Unplanned

<!-- Write your requirements here -->

## Planned

<!-- Moved here by /plan after task decomposition into TODO.md -->

## Completed

<!-- Moved here by /dev after development is done -->
```

## Content Generation Rules

1. **Never fabricate** - All content must come from actual code
2. **Use actual file paths** - Don't invent directories
3. **Reference real code** - Include actual code snippets
4. **Mark placeholders** - Use `__%` or `[TBD]` for unknown values
5. **Track sources** - Note where each piece of info came from