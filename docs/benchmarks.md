# Benchmarks

> All latency figures are measured on an Intel i7-1165G7 @ 2.80GHz running Linux.
> Network benchmarks reflect real iterative resolution over a typical home/office
> internet connection in China. Results on your hardware and network will differ —
> see [Running your own benchmark](#running-your-own-benchmark) to reproduce.

## First-Packet Resolution Latency (real network, 3-run average)

The four scenarios below show the progression from worst-case to best-case.
Results reflect the **Happy Eyeballs** optimization (concurrent dual-upstream
queries) and the **glueless NS delegation caching**:

| Domain | Cold start | IPPool only† | Full warmup | Cache hit |
|--------|-----------|-------------|-------------|-----------|
| `www.qq.com` | ~818 ms | ~663 ms | ~324 ms | ~0.05 ms |
| `www.baidu.com` | ~651 ms | ~465 ms | ~189 ms | ~0.06 ms |
| `www.taobao.com` | ~602 ms | ~680 ms | ~429 ms | ~0.15 ms |

† IPPool only: IP pool pre-seeded by warmup but zone cache flushed — this state
does not exist in production; included to isolate IP pool vs zone cache contributions.

- **Cold start** — IP pool is empty and zone cache is empty; the resolver has no
  prior RTT measurements or TLD NS information. This is the absolute worst case.
- **IPPool only†** — IP pool contains real RTT data from warmup, enabling better
  NS selection, but zone cache is empty so root → TLD traversal still required.
  This state is artificial (warmup always fills zone cache) and exists only to
  isolate the IP pool contribution.
- **Full warmup** — IP pool is pre-seeded AND zone cache contains TLD-level NS
  information (`.com`, `.net`, etc.) from warmup. This is the production
  steady-state: the server has been running long enough for warmup to complete,
  but the specific domain has not been queried before.
- **Cache hit** — A previously resolved domain is served entirely from memory.
  Latency drops to **< 0.2 ms**, a 1,000–10,000× improvement over iterative resolution.

## Cache Capacity (estimated, single A record per entry ≈ 450 bytes)

| Available memory | Estimated max cached domains |
|-----------------|------------------------------|
| 128 MB | ~280,000 |
| 256 MB | ~570,000 |
| 512 MB | ~1,130,000 |
| 1 GB | ~2,270,000 |

Complex responses (CNAME chains, multiple RRs) consume more memory per entry.

## Cache-Hit QPS (single core, in-process benchmark)

| Scenario | Throughput |
|----------|-----------|
| End-to-end cache hit (STATE_INIT → RETURN_RESP) | ~520,000 QPS |
| Cache layer read (hit) | ~1,500,000 QPS |
| 8-core concurrent mixed read/write | ~12,000,000 ops/s |

These figures are CPU-bound in-process measurements; real network QPS is limited
by connection handling and OS networking overhead.

## IP Pool Capacity (≈ 400 bytes per tracked NS IP)

| Available memory | Trackable NS IPs |
|-----------------|-----------------|
| 10 MB | ~25,000 |
| 50 MB | ~125,000 |
| 100 MB | ~250,000 |

## Profiling Findings (2026-03, dnsperf + pprof)

The project now includes a built-in load tool at `tools/dnsperf` and a controlled
pprof endpoint (`debug.pprof_enabled`, default off). Under local UDP load with
`dnsperf -c 128`, rec53 sustained about `~100k QPS` with zero timeouts/errors.

Profile capture method:

```bash
# 1) Run load (example)
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 128 -d 20s -proto udp

# 2) CPU profile (denoised view)
go tool pprof -top \
  -focus='rec53/server|github.com/miekg/dns' \
  -ignore='runtime/pprof|compress/flate|net/http/pprof|internal/runtime/syscall|runtime.futex' \
  http://127.0.0.1:6060/debug/pprof/profile?seconds=15

# 3) Allocation profile (denoised view)
go tool pprof -top -sample_index=alloc_space \
  -focus='rec53/server|github.com/miekg/dns' \
  -ignore='runtime/pprof|compress/flate|net/http/pprof' \
  http://127.0.0.1:6060/debug/pprof/heap
```

Using the denoised `alloc_space` view above, the main allocation hotspots were:

- Cache read-copy path (`getCacheCopy` / `getCacheCopyByType`): ~26-27% alloc_space
- DNS message copy path (`dns.Msg.Copy` / `CopyTo`): ~25% alloc_space
- Metrics reporting path (`InCounterAdd` / `OutCounterAdd` / `LatencyHistogramObserve`): ~24% alloc_space

Notes:

- `alloc_space` is cumulative allocated bytes, not resident memory (RSS).
- These percentages are workload-dependent and should be treated as directional
  guidance, not fixed SLO numbers.

CPU hotspots remained concentrated in the normal serving pipeline:

- `ServeDNS -> Change -> cacheLookup`

These findings define the v0.5.0 optimization order:

1. Reduce metrics-path allocations (`WithLabelValues`, lower label cardinality).
2. Evaluate cache COW with strict race-safety and mutation-audit guards.
3. Apply low-risk `getCacheKey` micro-optimization (`fmt.Sprintf` replacement).

## Concurrency Scaling (dnsperf, reproducible limit, 2026-03-18)

This section defines the reproducible "limit test" baseline on Intel i7-1165G7
(4C8T @ 2.80 GHz), Linux, UDP, cache-hit path.

Environment:

- rec53 on `127.0.0.1:5353` with perf config (`warmup=false`, `snapshot=false`,
  `log_level=error`, `pprof=false`)
- `tools/dnsperf` using `tools/dnsperf/queries-sample.txt` (13 pre-warmed domains)
- Matrix: `c=64/128/192`, each run `20s`, repeated `3` times

### Raw runs

| Concurrency | Run | Queries | Duration | QPS | P50 | P95 | P99 | Errors | Timeouts |
|------------|-----|---------|----------|-----|-----|-----|-----|--------|----------|
| 64 | 1 | 2,023,280 | 21.03 s | 96,225.1 | 507 us | 1.5 ms | 2.4 ms | 0 | 0 |
| 64 | 2 | 2,030,523 | 20.01 s | 101,488.9 | 505 us | 1.5 ms | 2.3 ms | 0 | 0 |
| 64 | 3 | 1,935,830 | 20.01 s | 96,755.4 | 543 us | 1.5 ms | 2.4 ms | 0 | 0 |
| 128 | 1 | 1,871,495 | 21.20 s | 88,283.2 | 1.1 ms | 3.3 ms | 5.2 ms | 0 | 0 |
| 128 | 2 | 1,997,139 | 20.01 s | 99,817.9 | 1.0 ms | 3.2 ms | 5.0 ms | 0 | 0 |
| 128 | 3 | 1,964,213 | 20.01 s | 98,167.6 | 1.1 ms | 3.2 ms | 5.1 ms | 0 | 0 |
| 192 | 1 | 1,951,932 | 22.29 s | 87,554.1 | 1.4 ms | 4.5 ms | 7.1 ms | 0 | 74 |
| 192 | 2 | 2,010,770 | 23.74 s | 84,700.7 | 1.4 ms | 4.6 ms | 7.3 ms | 0 | 72 |
| 192 | 3 | 2,029,032 | 24.43 s | 83,047.9 | 1.4 ms | 4.5 ms | 7.0 ms | 0 | 82 |

### Median summary (QPS)

| Concurrency | Median QPS | Min QPS | Max QPS |
|------------|------------|---------|---------|
| 64 | 96,755.4 | 96,225.1 | 101,488.9 |
| 128 | 98,167.6 | 88,283.2 | 99,817.9 |
| 192 | 84,700.7 | 83,047.9 | 87,554.1 |

### Conclusion

1. **`c=64` and `c=128` form the stable high-throughput plateau** (`~97-98k`
   median QPS), both with zero timeouts in this run.
2. **`c=192` is beyond stable operating range in this setup.** It consistently
   shows timeouts (`72-82`) and longer wall time (`22-24s`), so it should not
   be used as the default regression load level.
3. **Recommended default remains `c=64` for repeatability.** Throughput is near
   the plateau while latency and run-to-run jitter are lower than `c=128`.

Historical note: previous single-run 10s snapshots may report higher/lower
peaks, but release baselines should use this multi-run median method.

### SO_REUSEPORT projection

`miekg/dns` v1.1.52 natively supports `SO_REUSEPORT` via `dns.Server.ReusePort`.
Each listener pair (UDP+TCP) gets its own kernel receive queue, eliminating the
single-socket serialisation bottleneck.

Expected gains on 4C8T with `listeners: 4`:

| Estimate | QPS | Rationale |
|----------|-----|-----------|
| Conservative | 150–200 K | ~1.6–2.2× — kernel queue fanout, residual lock contention |
| Optimistic | 250–300 K | ~2.7–3.3× — near-linear scaling if cache lock is not hit |

Higher core-count servers (16C+) with proportional listener count should scale
further. Upgrading single-core frequency alone yields only ~15–25 % improvement.

Implementation cost: ~75 lines across 3 files, no handler or shared-state
changes required. Tracked in roadmap v0.4.1.

### Multi-Listener Benchmark (listeners=1 vs listeners=4, 2026-03-18)

Same hardware and test setup as above (i7-1165G7, UDP, c=64, 10 s, cache-hit path,
`tools/dnsperf` with 13-domain sample). rec53 running on `127.0.0.1:5353`.

| Metric | listeners=1 | listeners=4 | Delta |
|--------|------------|------------|-------|
| **QPS** | 93,927 | 94,718 | +0.8% |
| **P50** | 529 µs | 470 µs | −11% |
| **P95** | 1.7 ms | 1.9 ms | +12% |
| **P99** | 2.9 ms | 3.0 ms | +3% |
| **Max** | 12.6 ms | 1,514 ms | worse |
| **Timeouts** | 0 | 0 | — |
| **Errors** | 0 | 4 SERVFAIL | — |

**Observations:**

1. **Loopback neutralises SO_REUSEPORT gains.** On `127.0.0.1`, both client and
   server share the same CPU and memory bus. The kernel's per-socket receive queue
   is not the bottleneck — CPU contention between dnsperf and rec53 is. Real gains
   require separate client/server machines or high fan-in from many source IPs.

2. **P50 improved 11 % with listeners=4**, suggesting the kernel distributes
   packets more evenly across goroutines even on loopback. P95/P99/Max increased
   slightly, likely due to additional goroutine scheduling overhead from 4× more
   listener goroutines competing on the same cores.

3. **The 4 SERVFAIL errors with listeners=4** are transient cold-start noise
   (first queries hitting uncached domains before warmup pass completed), not a
   regression.

4. **Expected real-world impact**: On a dedicated server with external client
   traffic, listeners=4 on 4-core hardware should deliver 1.5–2× QPS improvement
   by eliminating the single-socket `recvfrom`/`sendto` serialisation measured in
   the [Concurrency Scaling](#concurrency-scaling-dnsperf-reproducible-limit-2026-03-18) section.

### Reproduce

```bash
# Build the tool
go build -o tools/dnsperf/dnsperf ./tools/dnsperf

# Start rec53
./rec53 --config ./config.yaml

# Warmup cache
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 4 -n 100 -proto udp

# Run reproducible limit matrix (20s x 3 per level)
for c in 64 128 192; do
  for i in 1 2 3; do
    echo "=== c=$c run=$i ==="
    tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
      -f tools/dnsperf/queries-sample.txt -c $c -d 20s -proto udp
  done
done
```

For quick daily smoke (non-baseline), a single run is still acceptable:

```bash
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 64 -d 10s -proto udp
```

To generate the same matrix file used above:

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

And compute medians:

```bash
awk 'NR>1 {print $1, $5}' /tmp/dnsperf_matrix.tsv \
  | sort -k1,1n -k2,2n \
  | awk '
    {
      c=$1; q[c,++n[c]]=$2
    }
    END {
      for (c in n) {
        m=(n[c]+1)/2
        printf "c=%s median_qps=%.1f\n", c, q[c,m]
      }
    }
  ' | sort -n
```

Legacy one-shot scaling command (kept for reference):

```bash
for c in 32 64 128 256; do
  echo "=== c=$c ==="
  tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
    -f tools/dnsperf/queries-sample.txt -c $c -d 10s -proto udp
done
```

## Regression Smoke Snapshot (2026-03-18)

This snapshot is a quick sanity sample used for day-to-day regression checks
in development (not a release-grade baseline replacement).

### Micro bench (server, benchmem)

```bash
go test -run '^$' -bench 'BenchmarkCacheGetHit|BenchmarkStateMachineCacheHit|BenchmarkRecordLatency' -benchmem ./server/...
```

| Benchmark | Result |
|----------|--------|
| `BenchmarkCacheGetHit` | `856.0 ns/op`, `296 B/op`, `7 allocs/op` |
| `BenchmarkStateMachineCacheHit` | `4204 ns/op`, `1074 B/op`, `33 allocs/op` |
| `BenchmarkRecordLatency` | `945.3 ns/op`, `0 B/op`, `0 allocs/op` |

### Macro load (`dnsperf`, UDP, c=64, 10s)

```bash
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 64 -d 10s -proto udp
```

- QPS: `96,941.6`
- Latency: `P50 499us`, `P95 1.6ms`, `P99 2.5ms`
- Reliability: `Timeouts 0`, `Errors 0`, `SERVFAIL 3`

## Running Your Own Benchmark

Use the built-in benchmark to measure first-packet latency on your own
infrastructure with domains relevant to your workload:

```bash
# Use default domain list (www.qq.com, www.baidu.com, www.taobao.com)
go test -v -run='^$' -bench='BenchmarkFirstPacket' \
    -benchtime=5x -timeout=300s ./e2e/...

# Override with your own domains
REC53_BENCH_DOMAINS="www.example.com,api.myservice.net" \
    go test -v -run='^$' -bench='BenchmarkFirstPacket' \
    -benchtime=5x -timeout=300s ./e2e/...

# Quick one-shot comparison table (all four scenarios side by side)
REC53_BENCH_DOMAINS="www.example.com,api.myservice.net" \
    go test -v -run='^$' -bench=BenchmarkFirstPacketComparison \
    -benchtime=1x -timeout=180s ./e2e/...
```

`REC53_BENCH_DOMAINS` accepts a comma-separated list of hostnames. The trailing
dot is added automatically. Separate multiple domains with commas and no spaces.

For standardized performance regression rules (bench, load, pprof, acceptance
criteria), see [`docs/perf-regression.md`](perf-regression.md).
For the complete recursive DNS test strategy (correctness + performance +
release gates), see [`docs/recursive-dns-test-plan.md`](recursive-dns-test-plan.md).
