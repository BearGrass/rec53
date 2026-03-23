# AGENTS.md

Execution guide for coding agents working in `rec53`.

## Scope

- This file is the fast path for build, test, style, and workflow expectations.
- Use `docs/architecture.md` for system design, `docs/dev/testing.md` for deeper test notes, and `docs/dev/conventions.md` for longer-form coding conventions.
- This repository currently has no `.cursorrules`, no `.cursor/rules/`, and no `.github/copilot-instructions.md`.

## Project Snapshot

- Language: Go
- Module: `rec53`
- Go version: `1.25.0`
- Main binary entry: `./cmd`
- Main packages: `cmd`, `server`, `monitor`, `tui`, `utils`, `e2e`
- Operational helper script: `./rec53ctl`
- Local ops TUI binary: `rec53top`

## Preferred Dev Workflow

1. Read the package you will change and the nearby tests first.
2. Make the smallest possible change that fits existing patterns.
3. Run one exact test or one package test while iterating.
4. Run `gofmt -w` on touched files before finishing.
5. Run the most relevant `go test` command, then widen to `go test -race ./...` for non-trivial work.
6. Update docs when user-visible behavior, metrics, flags, or ops flow changes.

## Build Commands

Recommended operator-style commands:

```bash
./rec53ctl config
./rec53ctl build
./rec53ctl build-top
./rec53ctl run
./rec53ctl top
```

Direct Go build commands:

```bash
mkdir -p dist && go build -o dist/rec53 ./cmd
mkdir -p dist && go build -o dist/rec53top ./cmd/rec53top
go build ./...
```

Useful install and lifecycle commands:

```bash
sudo ./rec53ctl install
sudo ./rec53ctl upgrade
sudo ./rec53ctl uninstall
sudo ./rec53ctl uninstall --purge
```

## Format And Static Checks

Primary formatting and verification commands:

```bash
gofmt -w .
gofmt -l .
go vet ./...
```

Notes:

- There is no repo-local `golangci-lint` config and no `Makefile`.
- `gofmt` is mandatory; do not hand-format Go code.
- Use `go vet` as the default static check before wider test runs.

## Test Commands

Full-suite commands:

```bash
go test ./...
go test -race ./...
go test -race -timeout 120s ./... -count=1
go test -short ./...
go test -cover ./...
```

Package-focused commands:

```bash
go test -v ./cmd/...
go test -v ./server/...
go test -v ./monitor/...
go test -v ./tui/...
go test -v ./e2e/...
go test -v ./utils/...
```

Run a single exact test by name:

```bash
go test -v -run '^TestName$' ./server/...
go test -v -run '^TestName$' ./cmd/...
go test -v -run '^TestName$' ./e2e/...
```

Real examples from this repo:

```bash
go test -v -run '^TestServerRunAndShutdown$' ./server/...
go test -v -run '^TestReadinessHandlerReportsColdStartAndWarming$' ./monitor/...
go test -v -run '^TestRunTraceModeWritesOrderedTrace$' ./cmd/...
go test -v -run '^TestRec53ctlTopBuildsAndExecsTUIBinary$' ./...
go test -v -run '^TestTraceDomainCapturesCacheHitSuccess$' ./server/...
go test -v -run '^TestResolverIntegration$' ./e2e/...
```

Benchmarks:

```bash
go test -bench=. ./server/...
go test -bench=. ./monitor/...
```

Performance helpers:

```bash
./tools/run-dnsperf.sh hit
./tools/run-dnsperf.sh miss
./tools/validate-perf.sh
```

## Which Tests To Run For Common Changes

- `cmd/`: config validation, CLI flags, startup/shutdown, signal handling, trace mode
- `server/`: resolver logic, cache, warmup, snapshot, XDP, state machine, lifecycle
- `monitor/`: metrics registration, readiness, logger behavior
- `tui/`: metric parsing, rendering, detail pages, keyboard flow
- `e2e/`: real resolver behavior, forwarding, snapshot, XDP, concurrency
- `utils/`: helpers like zone and root handling

Prefer this escalation path:

1. exact `-run` test
2. affected package test
3. `go test -race ./...`

## Repo-Specific Testing Rules

- Prefer `-race` for concurrency-sensitive or cache-related work.
- `e2e/main_test.go` owns `TestMain`; do not add per-file `init()` setup in `e2e/`.
- Use helpers from `e2e/helpers.go` for mock authority servers.
- Avoid unnecessary global cache or IP pool resets; cold state makes tests slower and noisier.
- Initialize monitor globals in shared test bootstrap when possible, not separately in every file.
- When changing metrics or labels, verify `docs/metrics.md` stays accurate.
- For lifecycle changes, test startup success, startup failure, readiness behavior, graceful shutdown, and optional-feature degradation paths.

## Code Style

- Follow standard Go formatting and idioms.
- Imports: standard library first, then third-party dependencies, then internal `rec53/...` packages.
- Packages stay lowercase and usually single-word.
- Exported names use PascalCase; unexported names use camelCase.
- Constants often follow existing `SCREAMING_SNAKE_CASE` patterns in state-machine code; do not rename existing conventions casually.
- Keep constructor naming aligned with nearby code, e.g. `newXState(...)`, `newXStateWithContext(...)`, `NewServer...`.
- Prefer table-driven tests where practical.
- Avoid speculative abstractions; this repo favors direct, explicit code.

## Types, Structs, And APIs

- Preserve existing public signatures unless the change truly requires an API change.
- Use pointer receivers for mutating or stateful structs.
- Keep state transitions explicit and easy to test.
- In state handlers, preserve the existing `(int, error)` return pattern; the integer controls flow.
- Do not bypass package helpers for cache or IP pool access.

## Error Handling

- Check errors immediately with `if err != nil`.
- Wrap returned errors with context using `fmt.Errorf("...: %w", err)`.
- Error messages should identify the failed operation, state, or subsystem.
- In hot resolver paths, do not hide the reason for fallback or degradation.
- When a path intentionally degrades rather than fails hard, log clearly and keep behavior explicit.

## Logging

- Use `monitor.Rec53Log` for application logging.
- Include useful context such as domain, query type, upstream IP, or state name when relevant.
- Keep log tags stable when a path already uses prefixes like `[SNAPSHOT]`, `[TRACE]`, `[XDP]`, or `[ITER]`.
- Do not add noisy debug logs to hot paths without a clear operational reason.

## Concurrency And Shared State

- Respect existing synchronization around cache, IP pool, and runtime globals.
- Use `sync.RWMutex` and `sync/atomic` consistently with nearby code.
- Goroutines must have a shutdown path; prefer `context.Context` for cancellation.
- Preserve bounded-concurrency patterns already used by warmup and IP prefetch flows.
- Do not read or mutate shared maps directly when helper methods exist.

## Cache And Resolver Safety

- Cache keys follow the repo's existing `domain.:qtype` style.
- Use cache helpers such as `getCacheCopyByType` and `setCacheCopyByType` instead of open-coding cache access.
- Cache-read DNS messages are shallow copies with shared RR pointers; do not mutate RR fields in place.
- If a code path must modify an RR from cached data, deep-copy that RR first.
- Keep resolver/state-machine flow explicit rather than clever.

## Documentation Sync

- User-facing CLI, config, or ops changes: update `README.md` and `README.zh.md` together.
- Developer-facing behavior or structure changes: update `docs/architecture.md` or `docs/dev/conventions.md` as needed.
- Metrics, labels, dashboard assumptions, or observability behavior: update `docs/metrics.md` and related operator docs.
- Do not invent benchmark numbers; only update performance docs after running the relevant tooling.

## Agent Reminders

- Unless the user explicitly requests another language, communicate with the user in Chinese.
- Start narrow and only widen scope when needed.
- Do not mix unrelated cleanup into functional changes.
- Match existing naming and structure before introducing new patterns.
- Before finishing substantial Go work, run formatting, the smallest relevant tests, and then broader verification.
