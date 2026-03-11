---
name: ai-dev-setup
description: |
  Bootstrap AI-collaborative dev environment for existing codebases.
  TRIGGER when: user says "setup ai dev", "bootstrap project docs", "create CLAUDE.md",
                "generate project documentation", "setup development workflow",
                "initialize dev environment", "init project docs", "/ai-dev-setup",
                or mentions setting up AI assistant for a codebase, or wants CLAUDE.md created.
  Also trigger when user asks to "set up this project for AI" or "make this project AI-ready".
  DO NOT TRIGGER when: project is new/empty, or user only wants to explore without setup.
license: Apache-2.0
compatibility:
  tools: [Read, Write, Edit, Bash, Glob, Grep, AskUserQuestion]
  platforms: [linux, macos, windows]
argument-hint: "[doc-directory-name]"
disable-model-invocation: true
---

# AI Development Environment Setup

Set up a complete AI-collaborative development environment for an existing codebase.

## Usage

```
/ai-dev-setup [doc-directory-name]
```

Default doc directory: `.docs`. Override with argument, e.g., `/ai-dev-setup docs`.

## Context Budget Rules

**Load reference files on demand — not all at once. Each step specifies which file to load.**

- Do NOT read all source files. Use directory listing + targeted sampling.
- Skip: vendor/, node_modules/, .git/, dist/, build/, __pycache__/, .venv/, target/, *.min.js, *.lock
- For large codebases (>50 source files): read 2-3 representative files per component, not every file.
- Keep generated docs concise — summaries, not code transcription.

---

## Step 1: Environment Detection

> **Load:** [references/env-detection.md](./references/env-detection.md)

1. Check project root config files → detect `LANG`, `BUILD_CMD`, `TEST_CMD`, `LINT_CMD`, `TEST_COVER_CMD`, `RACE_FLAG`, `DOC_DIR`
2. Check for existing: CLAUDE.md, `.claude/skills/`, DOC_DIR — handle per env-detection.md conflict rules

**Report detection results. Wait for user confirmation.**

---

## Step 2: Codebase Scan

**Context-efficient — do NOT read all source files.**

1. List top-level structure (2 levels deep) to map components
2. Read dependency files: go.mod, package.json, requirements.txt, Cargo.toml, etc.
3. For each component directory: read 1-2 representative files to understand patterns
4. `git log --oneline -20` for recent history
5. Grep for TODO/FIXME/HACK/XXX → collect file:line only, do not read surrounding code
6. Run `TEST_COVER_CMD` for coverage baseline (skip if no tests)

---

## Step 3: Generate Documentation

> **Load:** [references/doc-templates.md](./references/doc-templates.md)

Create `DOC_DIR/` and generate per templates. Rules:
- All content from actual code — never fabricate
- Mark unknowns as `__%` or `[TBD]`
- Each file max ~100 lines — link to code rather than quoting it

Files: README.md, ARCHITECTURE.md, CONVENTIONS.md, ROADMAP.md, TODO.md, TEST_PLAN.md, BACKLOG.md

---

## Step 4: Create CLAUDE.md

```markdown
# {Project Name}

## Build & Run
- Build: {BUILD_CMD}
- Run: [how to run]

## Testing
- Test: {TEST_CMD}
- Coverage: {TEST_COVER_CMD}

## Architecture
[One paragraph: purpose and structure]

## Dependencies
[External deps with purpose]

## Coding Conventions
See {DOC_DIR}/CONVENTIONS.md. Core principles:
1. [From actual code]
2. [From actual code]

## Documentation
- [Architecture](./{DOC_DIR}/ARCHITECTURE.md)
- [Conventions](./{DOC_DIR}/CONVENTIONS.md)
- [Roadmap](./{DOC_DIR}/ROADMAP.md)
- [TODO](./{DOC_DIR}/TODO.md)
- [Test Plan](./{DOC_DIR}/TEST_PLAN.md)
- [Backlog](./{DOC_DIR}/BACKLOG.md)
```

Append: [templates/claude-md-appendix.md](./templates/claude-md-appendix.md)

**If CLAUDE.md exists:** show sections, offer augment (recommended) / backup+overwrite / cancel. Wait.

---

## Step 5: Create Project Skills

> **Load:** [references/skill-templates.md](./references/skill-templates.md)

Generate into `.claude/skills/`: plan, dev, dev-resume, test, test-resume, sync-docs.

Replace: `{DOC_DIR}`, `{TEST_CMD}`, `{LINT_CMD}`, `{TEST_COVER_CMD}`, `{RACE_FLAG}`

**If skills directory exists:** list existing, warn overwrites, wait for confirmation.

---

## Step 6: Verify & Report

- [ ] Commands in CLAUDE.md execute successfully
- [ ] Directory structure in ARCHITECTURE.md matches reality
- [ ] All DOC_DIR files non-empty, all SKILL.md files have valid frontmatter

```
## Setup Complete

Project: {name} | Language: {LANG} | Source files: {count}
Coverage baseline: {coverage}%
Issues: BUG {n} / OPT {n} / DEBT {n}

Generated:
- CLAUDE.md
- .claude/skills/{plan,dev,dev-resume,test,test-resume,sync-docs}/SKILL.md
- {DOC_DIR}/[README,ARCHITECTURE,CONVENTIONS,ROADMAP,TODO,TEST_PLAN,BACKLOG].md

Next: review CLAUDE.md → add to BACKLOG.md → /plan → /test
```

---

## Error Handling

| Situation | Action |
|-----------|--------|
| Unknown language | Ask user for commands |
| No tests | Note 0% coverage, suggest /test |
| Command fails | Report, continue with available info |
| User rejects detection | Ask which commands to adjust, re-detect |
| User cancels | Clean up partial files, exit |