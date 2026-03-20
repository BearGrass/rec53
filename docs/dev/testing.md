# Testing

rec53 includes unit, package, benchmark, and end-to-end tests. Prefer targeted runs while iterating, then run a wider verification pass before merging or releasing.

## Common Commands

Full suite with race detection:

```bash
go test -race ./...
go test -race -timeout 120s ./... -count=1
```

Short mode:

```bash
go test -short ./...
```

Package-focused runs:

```bash
go test -v ./server/...
go test -v ./e2e/...
go test -v -run TestServerRunAndShutdown ./server/...
```

Coverage:

```bash
go test -cover ./...
```

## Expectations

- use `-race` for concurrency-sensitive work
- prefer table-driven tests where practical
- add targeted tests for startup, shutdown, malformed input, and cache behavior when touching those paths
- avoid unnecessary global resets in e2e tests because cold cache can make tests slow and noisy

## E2E Notes

- `e2e/main_test.go` owns `TestMain`
- do not add per-file `init()` setup in `e2e/`
- use helpers from `e2e/helpers.go` for mock authority servers

## What To Test For Lifecycle Changes

When changing `cmd/` or `server/server.go`, cover:

- startup success
- startup failure
- listener readiness behavior
- graceful shutdown
- warmup cancellation
- optional feature degradation paths

## Observability Checks

When changing metrics or labels, verify:

- metric names and labels remain bounded and intentional
- `docs/metrics.md` stays accurate
- operator-facing queries or dashboards do not silently break
- feature-gated metrics such as XDP counters are documented as conditional

## Performance Work

For benchmark or performance claims:

- prefer existing benchmark docs and tools under `tools/`
- start with `./tools/run-dnsperf.sh hit` for a quick macro-load sanity check
- use `./tools/run-dnsperf.sh miss` when you need cache-miss / iterative stress
- use `./tools/validate-perf.sh` only for the full dual-metric gate (`dnsperf` + `pprof`)
- do not present unmeasured improvements as fresh results
- update performance docs only when you actually ran the relevant validation
