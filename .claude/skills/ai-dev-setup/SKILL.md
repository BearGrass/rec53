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

Default doc directory is `.docs`. Override with argument, e.g., `/ai-dev-setup docs` or `/ai-dev-setup .rec53`.

## Process Overview

1. **Detect Environment** → Language, build/test/lint commands
2. **Scan Codebase** → Source files, dependencies, TODOs, git history
3. **Generate Documentation** → ARCHITECTURE, CONVENTIONS, ROADMAP, TODO, TEST_PLAN, BACKLOG
4. **Create CLAUDE.md** → Project instructions with self-maintenance rules
5. **Create Project Skills** → /plan, /dev, /test, /sync-docs
6. **Verify & Report** → Validate outputs, summarize findings

---

## Step 1: Environment Detection

1. Detect primary language from config files (see [references/env-detection.md](./references/env-detection.md))
2. Set variables: `LANG`, `BUILD_CMD`, `TEST_CMD`, `LINT_CMD`, `TEST_COVER_CMD`, `RACE_FLAG`
3. Check existing: CLAUDE.md, `.claude/skills/`, DOC_DIR

**Report detection results. Wait for user confirmation before proceeding.**

If user rejects → ask which commands to adjust, then re-detect.

---

## Step 2: Codebase Scan

1. Read all source files (exclude: vendor/, node_modules/, .git/, dist/, build/, __pycache__/, .venv/, target/, *.min.js)
2. Read dependency files (go.mod, package.json, requirements.txt, Cargo.toml, etc.)
3. Run `TEST_COVER_CMD` for coverage baseline (if tests exist)
4. Collect TODO/FIXME/HACK/XXX comments with file:line
5. Run `git log --oneline -20` for recent history

---

## Step 3: Generate Documentation

Create `DOC_DIR` and generate files using templates from [references/doc-templates.md](./references/doc-templates.md):

| File | Content Source |
|------|----------------|
| README.md | Index of all documentation |
| ARCHITECTURE.md | Directory structure from actual files, key components from code |
| CONVENTIONS.md | Code patterns extracted from actual code (use project's language) |
| ROADMAP.md | Version history from git tags, recent commits |
| TODO.md | TODO/FIXME/HACK comments collected, categorized as BUG/OPT/DEBT |
| TEST_PLAN.md | Coverage baseline, test files found, dependency-ordered batches |
| BACKLOG.md | Empty template for user to add requirements |

**Critical rules:**
- All content must come from actual code — never fabricate
- Use the project's language for code examples, not Go
- Mark unknown values as `__%` or `[TBD]`

---

## Step 4: Create CLAUDE.md

Generate CLAUDE.md with these required sections:

```markdown
# Project Name

## Build & Run
- Build: {BUILD_CMD}
- Run: [how to run the application]

## Testing
- Test: {TEST_CMD}
- Coverage: {TEST_COVER_CMD}

## Architecture
[One paragraph describing purpose and structure]

## Dependencies
[List external dependencies with purpose]

## Coding Conventions
See {DOC_DIR}/CONVENTIONS.md. Core principles:
1. [Principle from actual code]
2. [Principle from actual code]
3. [Principle from actual code]

## Documentation
- [Architecture](./{DOC_DIR}/ARCHITECTURE.md)
- [Conventions](./{DOC_DIR}/CONVENTIONS.md)
- [Roadmap](./{DOC_DIR}/ROADMAP.md)
- [TODO](./{DOC_DIR}/TODO.md)
- [Test Plan](./{DOC_DIR}/TEST_PLAN.md)
- [Backlog](./{DOC_DIR}/BACKLOG.md)
```

Then append self-maintenance rules from [templates/claude-md-appendix.md](./templates/claude-md-appendix.md).

**If CLAUDE.md exists:**
1. Show current sections
2. Offer: augment (recommended) / backup & overwrite / cancel
3. Wait for user choice

---

## Step 5: Create Project Skills

Generate skills into `.claude/skills/` using templates from [references/skill-templates.md](./references/skill-templates.md):

| Skill | Purpose |
|-------|---------|
| /plan | Analyze BACKLOG requirements, decompose into TODO tasks |
| /dev | Develop TODO tasks with code + tests + doc updates |
| /dev-resume | Resume interrupted development session |
| /test | Improve test coverage systematically |
| /test-resume | Resume interrupted testing session |
| /sync-docs | Sync documentation with code changes |

Replace placeholders: `{DOC_DIR}`, `{TEST_CMD}`, `{LINT_CMD}`, `{TEST_COVER_CMD}`, `{RACE_FLAG}`

**If skills directory exists:**
1. List existing skills
2. Warn about potential overwrites
3. Wait for confirmation

---

## Step 6: Verify & Report

### Verification Checklist

- [ ] All commands in CLAUDE.md execute successfully
- [ ] Directory structure in ARCHITECTURE.md matches reality
- [ ] Dependency list is complete (cross-check with lock files)
- [ ] All DOC_DIR files are non-empty
- [ ] All SKILL.md files have valid YAML frontmatter

### Final Report

Present to user:

```
## Setup Complete

**Project:** {name}
**Language:** {LANG}
**Files:** {count} source files

### Test Coverage
- Baseline: {coverage}%

### Issues Found
- BUG: {count}
- OPT: {count}
- DEBT: {count}

### Generated Files
- CLAUDE.md
- .claude/skills/{plan,dev,dev-resume,test,test-resume,sync-docs}/SKILL.md
- {DOC_DIR}/[README,ARCHITECTURE,CONVENTIONS,ROADMAP,TODO,TEST_PLAN,BACKLOG].md

### Recommended Next Steps
1. Review CLAUDE.md for accuracy
2. Add requirements to {DOC_DIR}/BACKLOG.md
3. Run /plan to break down requirements
4. Run /test to improve coverage
```

---

## Error Handling

| Situation | Action |
|-----------|--------|
| Unknown language | Ask user for build/test/lint commands |
| No tests found | Note 0% coverage, suggest /test |
| No source files | Suggest this is a new project, recommend scaffolding tools |
| Command fails | Report error, suggest manual intervention, continue with available info |
| User rejects detection | Ask which commands to adjust, re-run detection |
| User cancels setup | Clean up any partial files, exit gracefully |