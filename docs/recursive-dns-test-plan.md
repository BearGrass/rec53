# Recursive DNS Test Plan

This document defines a complete and repeatable test strategy for rec53 as a
recursive DNS resolver. It is designed for continuous use in day-to-day R&D,
not only for one-time release validation.

## 1. Goals

- Verify resolver correctness (answers, delegation, CNAME, negative responses).
- Catch regressions in concurrency safety (`-race`) and stability.
- Track performance trends (latency/QPS/allocations) with reproducible methods.
- Produce comparable evidence for PRs and release decisions.

## 1.1 Test Docs & Tools Map (Single Source of Truth)

Use this table to keep testing documentation and tooling aligned:

| Item | Role | Canonical Scope |
|------|------|-----------------|
| `docs/recursive-dns-test-plan.md` | Master test strategy (this doc) | Layering, release gates, artifacts, ownership boundaries |
| `docs/perf-regression.md` | Performance execution protocol | Exact perf commands, acceptance thresholds, reproducible matrix method |
| `docs/benchmarks.md` | Baseline snapshots | Measured numbers only (no policy decisions) |
| `tools/dnsperf` | Load generator | Macro load tests (`-f` replay / `-random-prefix` miss stress) |
| `tools/validate-v050.sh` | v0.5.0 dual-metric automation | One-command dnsperf + pprof validation workflow |

Drift-prevention rule:

- If perf command templates or thresholds change, update
  `docs/perf-regression.md` first, then update references in this file and
  `docs/benchmarks.md` in the same commit.
- Do not copy divergent command variants across docs; reference canonical
  commands when possible.

## 2. Test Layers

| Layer | Purpose | Command / Scope | Trigger |
|------|---------|------------------|---------|
| Unit | State logic and helper correctness | `go test ./server/... ./utils/...` | Every PR |
| Race | Concurrency safety | `go test -race ./...` | Every perf/concurrency PR; before release |
| Integration/E2E | End-to-end resolver behavior | `go test -v ./e2e/...` | Every PR touching resolution flow |
| Bench (micro) | Hot-path latency and allocations | `go test -run '^$' -bench . -benchmem ./server/... ./monitor/...` | Performance-sensitive PRs |
| Load (macro) | Service-level throughput and tail latency | `tools/dnsperf/dnsperf ...` | Performance-sensitive PRs + release |
| Profiling | Hotspot attribution | `go tool pprof ...` during active load | After load run when regressions appear |

## 2.1 Tooling Standard (`tools/`)

- Rebuild perf tools before load testing:

```bash
go build -o tools/dnsperf/dnsperf ./tools/dnsperf
```

- `tools/dnsperf` modes:
  - Replay mode (`-f tools/dnsperf/queries-sample.txt`): stable cache-hit and
    reproducible regression comparisons.
  - Random-prefix mode (`-random-prefix example.com`): cache-miss/iterative
    stress profile.
- `tools/validate-v050.sh`:
  - Linux-oriented helper for v0.5.0 dual-metric gate.
  - Requires `dig`, `curl`, `go tool pprof`, GNU `grep -P`.
  - Writes outputs to `/tmp/rec53-v050-validation`.

## 3. Functional Coverage Checklist

Minimum functional scenarios:

- Iterative resolution from root to authoritative servers.
- Cache hit/miss behavior for A/AAAA/CNAME/NS.
- CNAME chain handling (including cross-zone transitions).
- Negative caching behavior (NXDOMAIN/NODATA with SOA TTL).
- Glueless NS referral handling.
- Forwarding and hosts precedence (`hosts > forwarding > cache > iterative`).
- UDP truncation and TCP path behavior.
- Graceful shutdown + snapshot restore behavior.

## 4. Performance Coverage Checklist

Required benchmark set (before/after for performance PRs):

- `BenchmarkCacheGetHit`
- `BenchmarkStateMachineCacheHit`
- `BenchmarkRecordLatency`

Required load profile set:

- `dnsperf` replay profile at `c=64` and `c=128`, UDP, `20s` per case.
- For release baseline refresh, run fixed matrix `c=64/128/192`, `20s x 3`
  and use median QPS per level.
- Optional miss-stress profile: `-random-prefix example.com`.
- Record QPS, P50, P95, P99, errors, timeouts.

Required pprof set (denoised):

```bash
go tool pprof -top \
  -focus='rec53/server|github.com/miekg/dns' \
  -ignore='runtime/pprof|compress/flate|net/http/pprof|internal/runtime/syscall|runtime.futex' \
  http://127.0.0.1:6060/debug/pprof/profile?seconds=15

go tool pprof -top -sample_index=alloc_space \
  -focus='rec53/server|github.com/miekg/dns' \
  -ignore='runtime/pprof|compress/flate|net/http/pprof' \
  http://127.0.0.1:6060/debug/pprof/heap
```

## 5. Execution Profiles

### PR Quick Gate (developer loop)

```bash
go test ./server/... ./utils/...
go test -race ./server/...
go test -run '^$' -bench 'BenchmarkCacheGetHit|BenchmarkStateMachineCacheHit|BenchmarkRecordLatency' -benchmem ./server/...
go build -o tools/dnsperf/dnsperf ./tools/dnsperf
tools/dnsperf/dnsperf -server 127.0.0.1:5353 -f tools/dnsperf/queries-sample.txt -c 64 -d 10s -proto udp
```

### Performance PR Gate

```bash
go test -race ./...
go test -run '^$' -bench . -benchmem ./server/... ./monitor/... ./e2e/...
go build -o tools/dnsperf/dnsperf ./tools/dnsperf
tools/dnsperf/dnsperf -server 127.0.0.1:5353 -f tools/dnsperf/queries-sample.txt -c 64 -d 20s -proto udp
tools/dnsperf/dnsperf -server 127.0.0.1:5353 -f tools/dnsperf/queries-sample.txt -c 128 -d 20s -proto udp
# optional miss-stress
tools/dnsperf/dnsperf -server 127.0.0.1:5353 -random-prefix example.com -c 32 -d 20s -proto udp
```

### Release Gate

- Run full race suite: `go test -race ./...`.
- Run full e2e suite: `go test -v ./e2e/...`.
- Run performance gate (bench + load + pprof) and update baselines if changed.
- For v0.5.0-style dual-metric validation, `tools/validate-v050.sh` is allowed
  as an automation wrapper; include raw outputs in artifacts.

## 6. Pass/Fail Rules

- Correctness tests: zero failures.
- Race tests: zero race reports.
- Load tests: no unexplained increase in errors/timeouts.
- Benchmarks: no unexplained `ns/op` regression >10% on changed hot paths.
- Allocation-targeted changes: `allocs/op` must not regress on target benchmarks.

Any exception must be documented in PR notes with root cause and follow-up action.

## 7. Evidence and Artifacts

For performance-sensitive changes, attach:

- Benchmark output (`-benchmem`) before and after.
- `dnsperf` summary output for required concurrency levels.
- Denoised `pprof -top` output (CPU + alloc_space).
- Environment metadata: Go version, CPU model, config profile.

Store baseline snapshots in [`docs/benchmarks.md`](benchmarks.md), and keep
execution rules in [`docs/perf-regression.md`](perf-regression.md).

Testing tool outputs to preserve when relevant:

- `dnsperf` run outputs (`/tmp/dnsperf-runs/*.txt` or CI artifacts).
- Reproducible matrix table (`/tmp/dnsperf_matrix.tsv`).
- v0.5.0 script outputs (`/tmp/rec53-v050-validation/*`) when script is used.

## 8. Sustainability Rules (R&D Process)

- Use the same command set and query file across iterations unless intentionally changed.
- If methodology changes (query set, duration, filters), update docs in the same commit.
- Never update benchmark tables with unverified numbers.
- If local environment cannot run a required step, explicitly mark it as not run.
- Keep tool mode explicit in reports: replay (`-f`) vs miss-stress
  (`-random-prefix`), protocol, duration, concurrency.
