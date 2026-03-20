# Performance Regression Rules

This document defines the standard performance regression workflow for rec53.
Use it when evaluating performance-sensitive changes (cache, state machine, IP
pool, metrics, networking, and pprof-related code).

For full recursive DNS coverage (functional, e2e, release gates), see
[`docs/testing/recursive-dns-test-plan.md`](recursive-dns-test-plan.md).

## 1) Preconditions

- Build in the same environment when comparing before/after runs.
- Keep `config.yaml` stable across comparison runs.
- Record Go version (`go version`) and CPU model in the PR/change note.
- Rebuild perf tools before each run (do not trust stale binaries):

```bash
go build -o tools/dnsperf/dnsperf ./tools/dnsperf
```

## 1.1) Tooling in `tools/`

- `tools/dnsperf`:
  - Primary load tool for this repository.
  - Modes:
    - file replay: `-f tools/dnsperf/queries-sample.txt`
    - cache-miss stress: `-random-prefix example.com`
  - Supports UDP/TCP (`-proto`), duration mode (`-d`), count mode (`-n`), and
    optional rate limit (`-qps`).
- `tools/validate-perf.sh`:
  - One-command script for the dual-metric gate (dnsperf + pprof).
  - Output directory: `/tmp/rec53-perf-validation`.
  - Prerequisites: `dig`, `curl`, `go tool pprof`, GNU `grep` with `-P`.
  - Intended for dev/CI validation on Linux; if your environment lacks these
    dependencies, run the manual commands in this document instead.

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

Optional cache-miss stress profile:

```bash
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -random-prefix example.com -c 32 -d 20s -proto udp
```

Required review points:

- Report QPS, P50, P95, P99, errors, and timeouts.
- No unexplained error/timeout increase compared to baseline.
- If changing concurrency/network logic, include at least two concurrency levels
  (for example `c=64` and `c=128`).

## 4.1) Dual-Host Direct-Link Profile

Use this profile when loopback results are no longer representative, especially
for:

- `SO_REUSEPORT` / multi-listener evaluation
- UDP socket / syscall bottleneck investigation
- XDP disabled, Go-path throughput validation on physical NICs
- any change where client/server CPU contention on one machine would distort results

### Topology

- server host runs `rec53`
- client host runs `tools/dnsperf`
- hosts are connected directly or through a simple L2 path
- test addresses are the direct-link IPs (example: `192.168.53.1 <-> 192.168.53.2`)

Reference lab example used in current exploration:

- server: local host, direct-link IP `192.168.53.1`
- client: remote host `10.15.18.22`, direct-link IP `192.168.53.2`

### Link validation

Before comparing runs, validate the link from both sides:

```bash
# server
ping -c 2 192.168.53.2
ethtool <server-nic> | sed -n '1,40p'

# client
ping -c 2 192.168.53.1
ethtool <client-nic> | sed -n '1,40p'
```

Record at least:

- negotiated speed
- duplex
- whether packet loss is zero

### Stable comparison rules

- keep the query file fixed: `tools/dnsperf/queries-sample.txt`
- keep the server config fixed across runs except for the factor under test
- prefer a dedicated benchmark port such as `192.168.53.1:5353`
- disable unrelated features unless they are part of the comparison:
  - `warmup.enabled: false`
  - `snapshot.enabled: false`
  - `xdp.enabled: false`
- use a fresh server process before each `pprof`/heap comparison to avoid
  cumulative heap noise

### Recommended server config template

```yaml
dns:
  listen: "192.168.53.1:5353"
  metric: "127.0.0.1:9901"
  log_level: "error"
  listeners: 4

warmup:
  enabled: false
  timeout: 5s
  duration: 5s
  concurrency: 0
  tlds: []

snapshot:
  enabled: false
  file: ""

xdp:
  enabled: false
  interface: ""

debug:
  pprof_enabled: true
  pprof_listen: "127.0.0.1:6061"

hosts: []
forwarding: []
```

### Recommended load shape

Use multiple `dnsperf` workers from the client host to avoid the client itself
becoming the bottleneck:

```bash
# client warmup
./tools/dnsperf/dnsperf -server 192.168.53.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 8 -d 5s -proto udp

# client benchmark: 4 workers, each c=50, total concurrency 200
for i in 1 2 3 4; do
  ./tools/dnsperf/dnsperf -server 192.168.53.1:5353 \
    -f tools/dnsperf/queries-sample.txt -c 50 -d 20s -proto udp \
    > /tmp/rec53-run-$i.out &
done
wait
```

Aggregate QPS and timeouts:

```bash
grep '^  QPS:' /tmp/rec53-run-[1-4].out | awk '{sum+=$3} END {printf "TOTAL_QPS %.1f\n", sum}'
grep '^  Timeouts:' /tmp/rec53-run-[1-4].out | awk '{sum+=$3} END {printf "TOTAL_TIMEOUTS %d\n", sum}'
```

### Step-by-step execution protocol

Treat the following as the reusable dual-host baseline workflow:

1. On the server host, build `rec53`, prepare the benchmark config, and start a
   fresh process bound to the direct-link IP.
2. On the client host, build `tools/dnsperf`, run the 5-second warmup once, then
   launch the 4-worker benchmark.
3. While load is active, optionally collect `pprof` from the server host.
4. After the run, preserve the raw client outputs and aggregate QPS/timeouts.

Server-side example:

```bash
go build -o rec53 ./cmd
./rec53 --config ./config.perf.yaml
```

Client-side example:

```bash
go build -o tools/dnsperf/dnsperf ./tools/dnsperf

./tools/dnsperf/dnsperf -server 192.168.53.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 8 -d 5s -proto udp

for i in 1 2 3 4; do
  ./tools/dnsperf/dnsperf -server 192.168.53.1:5353 \
    -f tools/dnsperf/queries-sample.txt -c 50 -d 20s -proto udp \
    > /tmp/rec53-run-$i.out &
done
wait
```

### Example use: listeners comparison

For `SO_REUSEPORT` evaluation, keep everything else identical and compare:

- `listeners=1`
- `listeners=4`
- optionally `listeners=8`

Interpretation rules:

- if `1 -> 4` improves QPS materially and removes timeouts, multi-listener is effective
- if `4 -> 8` is flat, the machine has likely reached a good listener count already
- prefer the smallest listener count that reaches the plateau

### Profiling during direct-link load

Collect profiles on the server host while client load is active:

```bash
go tool pprof -top -nodecount=20 \
  http://127.0.0.1:6061/debug/pprof/profile?seconds=10

go tool pprof -top -sample_index=alloc_space -nodecount=20 \
  http://127.0.0.1:6061/debug/pprof/heap

go tool pprof -top -sample_index=alloc_objects -nodecount=20 \
  http://127.0.0.1:6061/debug/pprof/heap
```

Review points for this profile:

- whether `internal/runtime/syscall.Syscall6` remains a dominant CPU flat hotspot
- whether cache-read hotspots (`shallowCopyMsg`, `cacheLookupState.handle`) move materially
- whether `parseDstFromOOB` / `correctSource` become significant in physical NIC runs

### Evidence to preserve

For reusable dual-host runs, keep:

- server host metadata: CPU, kernel, NIC name
- client host metadata: CPU, kernel, NIC name
- negotiated link speed / duplex
- exact server config used
- raw `/tmp/rec53-run-*.out`
- aggregated QPS / timeout summary
- pprof outputs when profiling was part of the run

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

Use [`docs/testing/benchmarks.md`](benchmarks.md) as the baseline metrics snapshot.
If new measurements materially change expected ranges, update that file in the
same commit.

## 7) Sustainability Rules

- Keep command sets and query corpus stable across comparisons.
- Update methodology docs in the same commit when changing test parameters.
- Record "not run" items explicitly when environment limitations exist.
- Keep baseline snapshots in [`docs/testing/benchmarks.md`](benchmarks.md), and use
  this file as the authoritative reproducibility protocol.
- When updating perf docs, include the exact tool mode (`-f` vs
  `-random-prefix`), protocol, and command line so results can be replayed.
