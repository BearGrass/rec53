# Contributing

This guide covers the expected development workflow for rec53.

## Local Setup

Typical commands:

```bash
go build -o rec53 ./cmd
./generate-config.sh
./rec53 --config ./config.yaml
```

Recommended operator-style workflow during development:

```bash
./rec53ctl build
./rec53ctl run
```

## Before Changing Code

Read first:

- [Architecture](../architecture.md)
- [Testing](testing.md)
- [Coding Conventions](conventions.md)
- [AGENTS.md](../../AGENTS.md)

Pay special attention to:

- state machine flow in `server/`
- cache copy invariants
- IP pool concurrency
- startup and shutdown ordering

## Change Scope

Preferred approach:

- small, bounded changes
- explicit tests for lifecycle or concurrency fixes
- no speculative abstraction
- no unrelated cleanup mixed into functional changes

Avoid:

- large state-machine rewrites before a release
- changing default behavior and docs in separate commits
- enabling optional features by default without operator validation

## Documentation Sync

Keep these in sync when behavior changes:

- `README.md` and `README.zh.md`
- relevant `docs/user/*`
- relevant `docs/dev/*`
- `docs/architecture.md` when developer-facing behavior or structure changes

If you add dependencies, update the relevant docs in the same change.
