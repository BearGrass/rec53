# AGENTS.md

Minimal working guide for AI coding agents in this repository.

## Scope

- This file is for execution guidance: what to run, what to avoid, and which repo rules matter.
- Do not treat it as full architecture documentation; use `docs/architecture.md` for deeper system design.
- There is currently no `.cursorrules`, `.cursor/rules/`, or `.github/copilot-instructions.md` in this repo.

## Project Facts

- Language: Go
- Module: `rec53`
- Go version: `go 1.25.0`
- Main binary entry: `./cmd`
- Main packages: `cmd`, `server`, `monitor`, `utils`, `e2e`

## Build And Run

```bash
go build -o rec53 ./cmd
./generate-config.sh
./rec53 --config ./config.yaml
./rec53 --config ./config.yaml --no-warmup
./rec53 --config ./config.yaml -listen 0.0.0.0:53 -metric :9099 -log-level debug
```

## Format, Lint, Verify

```bash
gofmt -w .
gofmt -l .
go vet ./...
```

- There is no `Makefile` and no repo-local `golangci-lint` config.
- Standard verification is `gofmt`, `go vet`, and the most relevant `go test` command.

## Test Commands

```bash
# Full suite
go test -race ./...
go test -race -timeout 120s ./... -count=1

# Single exact test
go test -v -run '^TestName$' ./server/...

# Common exact examples
go test -v -run '^TestValidateConfig$' ./cmd/...
go test -v -run '^TestResolverIntegration$' ./e2e/...
go test -v -run '^TestIPPool_GetBestIPsV2_MultipleIPs$' ./server/...

# Package-level runs
go test -v ./cmd/...
go test -v ./server/...
go test -v ./e2e/...
go test -v ./monitor/...

# Other useful modes
go test -short ./...
go test -bench=. ./server/...
go test -cover ./...
```

## Pick The Smallest Relevant Test First

- CLI/config/logging/startup/signal changes: `./cmd/...`
- Resolver/cache/warmup/snapshot/XDP/state-machine changes: `./server/...`
- End-to-end query behavior/forwarding/recursion/lifecycle changes: `./e2e/...`
- Metrics/logger changes: `./monitor/...`
- Prefer exact `-run` first, then package tests, then `-race ./...`.

## Repo-Specific Testing Rules

- Prefer `-race` for non-trivial changes.
- `e2e/main_test.go` owns `TestMain`; do not add per-file `init()` setup in e2e tests.
- Initialize monitor globals once in test bootstrap, not once per file.
- Do not call `FlushCacheForTest()` or `ResetIPPoolForTest()` unless a test truly requires cold state.
- Avoid unnecessary cold-cache resets; iterative resolution can be slow.
- Use `NewMockAuthorityServer(t, zone)` from `e2e/helpers.go` for authoritative fixtures.
- Use `SetIterPort` / `ResetIterPort` when tests need iterative resolver port control.

## Code Style: Keep To Existing Patterns

- Run `gofmt -w .` before finishing Go changes.
- Import order: standard library, external deps, internal `rec53/*` packages.
- Packages are lowercase; exported identifiers use PascalCase; unexported identifiers use camelCase.
- Keep existing constructor and state naming patterns such as `newXState(...)` and `newXStateWithContext(...)`.
- Use pointer receivers for mutating/stateful structs.
- Preserve existing public signatures and flow-control return patterns unless a change is necessary.

## Error Handling And Logging

- Check errors immediately with `if err != nil`.
- Wrap propagated errors with `fmt.Errorf("...: %w", err)`.
- State handlers return `(int, error)`; the `int` is flow control.
- Include enough context to identify the failed state or operation.
- Use `monitor.Rec53Log` for logging.
- Include domain, query type, and upstream IP when relevant.
- Prefix resolver hot-path logs with stable tags like `[IN_CACHE]` or `[ITER]`.

## Concurrency And Shared State

- Preserve the existing synchronization strategy around shared maps such as DNS cache and IP pool.
- Use `sync.RWMutex` for shared maps and `sync/atomic` for hot counters/flags.
- Use `context.Context` for cancellation.
- Avoid goroutines that cannot be stopped.
- Prefer bounded concurrency where the repo already uses semaphore patterns.

## Resolver And Cache Safety

- Keep state transitions explicit and testable.
- Typical resolver flow is `STATE_INIT -> IN_CACHE -> CHECK_RESP -> IN_GLUE -> IN_GLUE_CACHE -> ITER -> RET_RESP`.
- Cache keys follow `domain.:qtype`.
- Use cache helpers such as `getCacheCopyByType` and `setCacheCopyByType`; do not bypass them.
- Cache-read DNS messages are not safe for in-place RR field mutation; deep-copy before mutation.

## Docs To Update When Behavior Changes

- User-facing behavior, flags, examples, or operations: update `README.md` and `README.zh.md` together.
- Architecture or package responsibility changes: update `docs/architecture.md`.
- New standard coding patterns: update `docs/dev/conventions.md`.
- If benchmark docs change, run relevant benchmarks when feasible; do not invent fresh numbers.

## Agent Workflow

- Start narrow: read the package you will touch, run one targeted test, then widen scope.
- Prefer package-level verification during iteration.
- Before finishing substantial Go work, run `gofmt -w .`, `go vet ./...`, and the most relevant `go test` command.
- Use this file for execution rules, `docs/dev/conventions.md` for longer conventions, and `docs/architecture.md` for deeper design context.
