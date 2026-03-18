# Performance Regression Rules

This document defines the standard performance regression workflow for rec53.
Use it when evaluating performance-sensitive changes (cache, state machine, IP
pool, metrics, networking, and pprof-related code).

For full recursive DNS coverage (functional, e2e, release gates), see
[`docs/recursive-dns-test-plan.md`](recursive-dns-test-plan.md).

## 1) Preconditions

- Build in the same environment when comparing before/after runs.
- Keep `config.yaml` stable across comparison runs.
- Record Go version (`go version`) and CPU model in the PR/change note.

## 2) Correctness Gate (must pass first)

```bash
go test -race ./...
```

Do not accept performance numbers if race tests fail.

## 3) Benchmark Gate (micro)

Run benchmark suites with allocation metrics:

```bash
go test -run '^$' -bench . -benchmem ./server/...
go test -run '^$' -bench . -benchmem ./monitor/...
go test -run '^$' -bench . -benchmem ./e2e/...
```

Required review points:

- For changed hot paths, `allocs/op` should not increase unless justified.
- For changed hot paths, `ns/op` regression greater than 10% requires explicit justification.
- Always include before/after numbers for at least:
  - `BenchmarkCacheGetHit`
  - `BenchmarkStateMachineCacheHit`
  - `BenchmarkRecordLatency`

## 4) Load Gate (macro, dnsperf)

Use `tools/dnsperf` for network-level regression checks:

```bash
go build -o tools/dnsperf/dnsperf ./tools/dnsperf
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 128 -d 20s -proto udp
```

Required review points:

- Report QPS, P50, P95, P99, errors, and timeouts.
- No unexplained error/timeout increase compared to baseline.
- If changing concurrency/network logic, include at least two concurrency levels
  (for example `c=64` and `c=128`).

### Reproducible limit profile (release baseline)

When updating performance baseline documents, run the fixed matrix below instead
of a single-shot load:

```bash
mkdir -p /tmp/dnsperf-runs
echo -e "concurrency\trun\tqueries\tduration_s\tqps\tp50\tp95\tp99\terrors\ttimeouts" > /tmp/dnsperf_matrix.tsv
for c in 64 128 192; do
  for i in 1 2 3; do
    f="/tmp/dnsperf-runs/c${c}-r${i}.txt"
    tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
      -f tools/dnsperf/queries-sample.txt -c "$c" -d 20s -proto udp > "$f"
    q=$(awk '/^  Summary/{flag=1; next} flag && /^  Queries:/ {print $2; exit}' "$f")
    d=$(awk '/^  Summary/{flag=1; next} flag && /^  Duration:/ {print $2; exit}' "$f")
    qps=$(awk '/^  Summary/{flag=1; next} flag && /^  QPS:/ {print $2; exit}' "$f")
    p50=$(awk '/^  Latency:/{flag=1; next} flag && /^    P50/ {print $2; exit}' "$f")
    p95=$(awk '/^  Latency:/{flag=1; next} flag && /^    P95/ {print $2; exit}' "$f")
    p99=$(awk '/^  Latency:/{flag=1; next} flag && /^    P99[[:space:]]/ {print $2; exit}' "$f")
    err=$(awk '/^  Summary/{flag=1; next} flag && /^  Errors:/ {print $2; exit}' "$f")
    to=$(awk '/^  Summary/{flag=1; next} flag && /^  Timeouts:/ {print $2; exit}' "$f")
    printf "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n" "$c" "$i" "$q" "$d" "$qps" "$p50" "$p95" "$p99" "$err" "$to" >> /tmp/dnsperf_matrix.tsv
  done
done
cat /tmp/dnsperf_matrix.tsv
```

Acceptance/reporting requirements for this profile:

- Use median QPS per concurrency level as the baseline number.
- Record min/max QPS range (run-to-run jitter).
- Record timeout/error counts and wall-time drift (for example 20s target run
  stretched to 24-25s under overload).
- Default stable load level is `c=64`; do not promote higher levels as baseline
  if they increase timeout rate.

## 5) pprof Gate (for performance PRs)

Collect denoised CPU and allocation profiles during active load:

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

Required review points:

- Explain hotspot shifts in `ServeDNS -> Change -> cacheLookup` related paths.
- `alloc_space` is cumulative allocation, not RSS. Interpret accordingly.
- For allocation-targeted PRs, include before/after hotspot percentages.

## 6) Baseline Source

Use [`docs/benchmarks.md`](docs/benchmarks.md) as the baseline metrics snapshot.
If new measurements materially change expected ranges, update that file in the
same commit.

## 7) Sustainability Rules

- Keep command sets and query corpus stable across comparisons.
- Update methodology docs in the same commit when changing test parameters.
- Record "not run" items explicitly when environment limitations exist.
- Keep baseline snapshots in [`docs/benchmarks.md`](docs/benchmarks.md), and use
  this file as the authoritative reproducibility protocol.
