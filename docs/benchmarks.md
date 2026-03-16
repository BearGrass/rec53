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
