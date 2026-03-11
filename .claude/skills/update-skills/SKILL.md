---
name: update-skills
description: |
  Regenerate project workflow skills (.claude/skills/) without touching any documentation.
  TRIGGER when: user says "update skills", "regenerate skills", "refresh skills",
                "update project skills", "只更新skill", "重新生成技能",
                or wants to update skills after ai-dev-setup without redoing docs.
  DO NOT TRIGGER when: user wants full setup (use /ai-dev-setup instead).
license: Apache-2.0
compatibility:
  tools: [Read, Write, Edit, Bash, Glob]
  platforms: [linux, macos, windows]
argument-hint: "[doc-directory-name]"
disable-model-invocation: true
---

# Update Project Skills

Regenerate `.claude/skills/` workflow files for the current project. Docs are untouched.

## Usage

```
/update-skills [doc-directory-name]
```

If `$ARGUMENTS` is provided, use it as `DOC_DIR`. Otherwise detect from CLAUDE.md.

---

## Step 1: Read Project Config

1. Read `CLAUDE.md` — extract:
   - `DOC_DIR`: look for documentation links like `(./.docs/` or `(./docs/` — use that directory name
   - `BUILD_CMD`, `TEST_CMD`, `LINT_CMD`, `TEST_COVER_CMD` from Build & Run / Testing sections
   - If `$ARGUMENTS` is provided, override `DOC_DIR` with it
2. If CLAUDE.md not found: ask user for `DOC_DIR` and commands before proceeding

Set variables: `DOC_DIR`, `TEST_CMD`, `LINT_CMD`, `TEST_COVER_CMD`, `RACE_FLAG` (Go projects only, `-race`)

**Report extracted values. Wait for confirmation.**

---

## Step 2: Check Existing Skills

List `.claude/skills/` contents. Report which of these already exist:
- plan, dev, dev-resume, test, test-resume, sync-docs

If any exist: warn they will be overwritten. Wait for confirmation.

---

## Step 3: Generate Skills

> **Load:** [../ai-dev-setup/references/skill-templates.md](../ai-dev-setup/references/skill-templates.md)

Generate all 6 skills into `.claude/skills/`, replacing placeholders:
- `{DOC_DIR}` → detected DOC_DIR
- `{TEST_CMD}` → detected TEST_CMD
- `{LINT_CMD}` → detected LINT_CMD
- `{TEST_COVER_CMD}` → detected TEST_COVER_CMD
- `{RACE_FLAG}` → detected RACE_FLAG

Skills to generate: plan, dev, dev-resume, test, test-resume, sync-docs

---

## Step 4: Report

```
## Skills Updated

DOC_DIR: {DOC_DIR}
TEST_CMD: {TEST_CMD}

Updated:
- .claude/skills/plan/SKILL.md
- .claude/skills/dev/SKILL.md
- .claude/skills/dev-resume/SKILL.md
- .claude/skills/test/SKILL.md
- .claude/skills/test-resume/SKILL.md
- .claude/skills/sync-docs/SKILL.md

Docs untouched. Run /plan to start planning, /dev to develop, /test to improve coverage.
```

---

## Error Handling

| Situation | Action |
|-----------|--------|
| CLAUDE.md missing | Ask user for DOC_DIR and commands |
| DOC_DIR not found in CLAUDE.md | Ask user which directory to use |
| skill-templates.md not found | Report path tried, ask user to provide commands manually |
| `.claude/skills/` missing | Create it |
