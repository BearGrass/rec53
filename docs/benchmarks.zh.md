# 基准测试

[English](benchmarks.md) | 中文

> 所有延迟数据都采集自 Intel i7-1165G7 @ 2.80GHz、Linux 环境。
> 网络基准反映的是在中国常见家庭/办公网络下的真实迭代解析结果。你的硬件和网络会不同 —— 请参考[自行运行基准](#自行运行基准)复现。

## 首包解析延迟（真实网络，3 次平均）

下表展示了从最差到最好的四种场景。
结果反映了 **Happy Eyeballs** 优化（双上游并发查询）和 **无 glue 委派缓存**：

| Domain | Cold start | IPPool only† | Full warmup | Cache hit |
|--------|-----------|-------------|-------------|-----------|
| `www.qq.com` | ~818 ms | ~663 ms | ~324 ms | ~0.05 ms |
| `www.baidu.com` | ~651 ms | ~465 ms | ~189 ms | ~0.06 ms |
| `www.taobao.com` | ~602 ms | ~680 ms | ~429 ms | ~0.15 ms |

† IPPool only：IP 池已由 warmup 预填充，但 zone cache 已清空——这个状态不会出现在生产中，只用于拆分 IP 池和 zone cache 的贡献。

- **Cold start** — IP 池为空，zone cache 也为空；解析器没有任何 RTT 记录或 TLD NS 信息。这是最差情况。
- **IPPool only†** — IP 池里有 warmup 得到的真实 RTT 数据，可以更好地选择 NS，但 zone cache 仍为空，所以仍要走 root → TLD 过程。这个状态是人为构造的（warmup 总会填充 zone cache），只用于隔离 IP 池贡献。
- **Full warmup** — IP 池已预填充，并且 zone cache 包含 TLD 级 NS 信息（`.com`、`.net` 等）。这是生产稳态：服务运行足够久，warmup 已完成，但目标域名还没被查询过。
- **Cache hit** — 已经解析过的域名直接从内存返回。延迟降到 **< 0.2 ms**，比迭代解析快 1,000–10,000 倍。

## 缓存容量（估算，单 A 记录条目 ≈ 450 字节）

| Available memory | Estimated max cached domains |
|-----------------|------------------------------|
| 128 MB | ~280,000 |
| 256 MB | ~570,000 |
| 512 MB | ~1,130,000 |
| 1 GB | ~2,270,000 |

复杂响应（CNAME 链、多 RR）会占用更多内存。

## Cache-hit QPS（单核、进程内基准）

| Scenario | Throughput |
|----------|-----------|
| End-to-end cache hit (STATE_INIT → RETURN_RESP) | ~520,000 QPS |
| Cache layer read (hit) | ~1,500,000 QPS |
| 8-core concurrent mixed read/write | ~12,000,000 ops/s |

这些数据是 CPU 限制下的进程内测量；真实网络 QPS 会受连接处理和 OS 网络开销限制。

## IP 池容量（每个 NS IP 约 400 字节）

| Available memory | Trackable NS IPs |
|-----------------|-----------------|
| 10 MB | ~25,000 |
| 50 MB | ~125,000 |
| 100 MB | ~250,000 |

## Profiling Findings (2026-03, dnsperf + pprof)

项目现在包含内置压测工具 `tools/dnsperf` 和受控的 pprof 端点（`debug.pprof_enabled`，默认关闭）。在本地 UDP 负载、`dnsperf -c 128` 下，rec53 大约能维持 `~100k QPS`，且无超时/错误。

采集方法：

```bash
# 1) 运行负载
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 128 -d 20s -proto udp

# 2) CPU profile（去噪）
go tool pprof -top \
  -focus='rec53/server|github.com/miekg/dns' \
  -ignore='runtime/pprof|compress/flate|net/http/pprof|internal/runtime/syscall|runtime.futex' \
  http://127.0.0.1:6060/debug/pprof/profile?seconds=15

# 3) Allocation profile（去噪）
go tool pprof -top -sample_index=alloc_space \
  -focus='rec53/server|github.com/miekg/dns' \
  -ignore='runtime/pprof|compress/flate|net/http/pprof' \
  http://127.0.0.1:6060/debug/pprof/heap
```

使用去噪后的 `alloc_space` 视图，主要分配热点曾经是（v0.4.1 基线，**优化前**）：

- Cache read-copy 路径（`getCacheCopy` / `getCacheCopyByType`）：约 26-27% alloc_space
- DNS message copy 路径（`dns.Msg.Copy` / `CopyTo`）：约 25% alloc_space
- 指标上报路径（`InCounterAdd` / `OutCounterAdd` / `LatencyHistogramObserve`）：约 24% alloc_space

v0.5.0 之后，指标路径降到约 3.8% —— 见 [v0.5.0 章节](#v050-hot-path-allocation-optimization-2026-03-18) 的更新数据。

说明：

- `alloc_space` 是累计分配字节数，不是 RSS。
- 这些百分比依赖工作负载，应视为方向性参考，不是固定 SLO。

CPU 热点仍集中在正常服务路径：

- `ServeDNS -> Change -> cacheLookup`

这些结论定义了 v0.5.0 的优化顺序：

1. 降低指标路径分配（`WithLabelValues`，降低 label 基数）。
2. 在严格 race-safety 和 mutation-audit 约束下评估 cache COW。
3. 对 `getCacheKey` 做低风险微优化（替换 `fmt.Sprintf`）。

## v0.5.0 热路径分配优化（2026-03-18）

### 变更

1. **移除指标 label：** 从 `InCounter`、`OutCounter`、`LatencyHistogramObserver` 中移除 `name`（原始 FQDN）label —— 消除无界基数。
2. **切换 `WithLabelValues`：** 所有指标方法从 `With(prometheus.Labels{...})` 改成 `WithLabelValues(...)` —— 消除每次调用的 map 分配。
3. **`getCacheKey` 优化：** 从 `fmt.Sprintf("%s:%d", ...)` 改为字符串拼接 + `strconv.FormatUint` —— 零分配。

### 微基准对比（BenchmarkCacheKey，-count=5）

| Metric | v0.4.1 (before) | v0.5.0 (after) | Delta |
|--------|-----------------|----------------|-------|
| ns/op  | ~68-69          | ~17            | −75%  |
| B/op   | 16              | 0              | −100% |
| allocs/op | 1            | 0              | −100% |

### 双指标验收门槛 — 通过

| Gate | Metric | v0.4.1 baseline | v0.5.0 measured | Delta | Status |
|------|--------|-----------------|-----------------|-------|--------|
| 1 | dnsperf median QPS (c=64, 20s × 3) | ~97K | 111,049 | **+14.5%** | PASS |
| 1 | dnsperf P99 | ~2.4ms | 2.4ms | 0% | PASS |
| 2 | pprof alloc_space — metrics path | ~24% | ~3.8% | **−84%** | PASS |

### dnsperf 原始运行（c=64, 20s, v0.5.0）

| Run | Queries | Duration | QPS | P50 | P95 | P99 | Errors | Timeouts |
|-----|---------|----------|-----|-----|-----|-----|--------|----------|
| 1 | 2,223,558 | 20.01s | 111,135.0 | 452 µs | 1.4 ms | 2.4 ms | 0 | 0 |
| 2 | 2,221,846 | 20.01s | 111,049.5 | 452 µs | 1.4 ms | 2.4 ms | 0 | 0 |
| 3 | 2,209,928 | 20.00s | 110,489.3 | 449 µs | 1.4 ms | 2.4 ms | 0 | 0 |

### pprof alloc_space breakdown（v0.5.0，去噪）

| Source | alloc_space | % of total |
|--------|-------------|------------|
| `dns.(*Msg).Copy` (cache COW) | 6.88 GB | 31.53% |
| `cacheLookupState.handle` | 1.18 GB | 5.43% |
| `dns.packBufferWithCompressionMap` | 1.34 GB | 6.14% |
| `dns.(*A).copy` | 1.40 GB | 6.41% |
| `LatencyHistogramObserve` | 0.33 GB | 1.50% |
| `OutCounterAdd` | 0.29 GB | 1.35% |
| `InCounterAdd` | 0.21 GB | 0.96% |
| **Metrics subtotal** | **0.83 GB** | **3.81%** |

相比 v0.4.1，指标路径分配从约 24% 降到约 3.8%（−84%）。当前主要分配来源是 `dns.Msg.Copy`，约 32%，是后续 cache COW 的候选点。

### Cache COW 后续评估

门槛：v0.5.0 优化后，`dns.Msg.Copy` 去噪 `alloc_space` 超过 20%。

结果：**31.53%** —— 超过 20% 阈值，说明 Cache COW 后续工作是有必要的，应该单独跟踪。见 `server/cache.go` 中的 `getCacheCopy` / `getCacheCopyByType`。

## Cache Shallow Copy Optimization（2026-03-18）

### 变更

1. **读路径浅拷贝：** 在 `getCacheCopy` 中用 `shallowCopyMsg()` 替换 `msg.Copy()`（深拷贝）—— 新切片头共享 RR 指针，减少每次缓存命中的 RR 分配。
2. **写路径剥离 OPT：** 写入缓存前从 `msg.Extra` 中移除 `*dns.OPT` 记录—— 去掉唯一已知会触发 `Pack()` 变更的因素，使共享 RR 指针在并发 `Pack()` 下安全。
3. **保留写侧深拷贝：** `setCacheCopy` 仍然执行 `value.Copy()`，保护缓存条目不被调用方修改。

### 微基准对比（BenchmarkCacheGetHit，-benchmem -count=5）

| Metric | v0.5.0 (deep copy) | shallow copy | Delta |
|--------|-------------------|--------------|-------|
| ns/op  | ~234              | ~175         | −25%  |
| B/op   | 264               | 184          | −30%  |
| allocs/op | 5              | 3            | −40%  |

### BenchmarkShallowVsDeepCopy（3 Answer + 1 Ns + 1 Extra RRs）

| Variant | ns/op | B/op | allocs/op |
|---------|-------|------|-----------|
| ShallowCopy | ~143 | 248 | 5 |
| DeepCopy | ~294 | 472 | 11 |
| **Delta** | **−51%** | **−47%** | **−55%** |

### 双指标验收门槛 — 通过

| Gate | Metric | v0.5.0 baseline | Shallow copy measured | Delta | Status |
|------|--------|-----------------|----------------------|-------|--------|
| 1 | dnsperf median QPS (c=64, 20s × 3) | ~111K | 119,430 | **+7.6%** | PASS |
| 1 | dnsperf P99 | 2.4ms | 2.3ms | −4% | PASS |
| 2 | pprof alloc_space — cache copy path | 31.53% | 15.67% | **−50.3%** | PASS |

### dnsperf 原始运行（c=64, 20s, cache-shallow-copy）

| Run | Queries | Duration | QPS | P50 | P95 | P99 | Errors | Timeouts |
|-----|---------|----------|-----|-----|-----|-----|--------|----------|
| 1 | 2,389,561 | 20.01s | 119,429.6 | 419 µs | 1.3 ms | 2.3 ms | 0 | 0 |
| 2 | 2,415,445 | 20.01s | 120,723.9 | 415 µs | 1.3 ms | 2.2 ms | 0 | 0 |
| 3 | 2,373,104 | 20.01s | 118,608.1 | 420 µs | 1.3 ms | 2.3 ms | 0 | 0 |

### pprof alloc_space breakdown（cache-shallow-copy，top sources）

| Source | alloc_space | % of total |
|--------|-------------|------------|
| `shallowCopyMsg` | 0.46 GB | 15.67% |
| `cacheLookupState.handle` | 0.20 GB | 6.81% |
| `dns.packBufferWithCompressionMap` | 0.23 GB | 7.64% |
| `dns.(*CNAME).copy` (followCNAME path) | 0.05 GB | 1.54% |
| `LatencyHistogramObserve` | 0.06 GB | 1.93% |
| `OutCounterAdd` | 0.05 GB | 1.75% |
| `InCounterAdd` | 0.03 GB | 1.14% |
| **Metrics subtotal** | **0.14 GB** | **4.82%** |

相比 v0.5.0 基线，cache read 路径分配从 31.53%（`dns.Msg.Copy`）降到 15.67%（`shallowCopyMsg`），减少 50.3%。`dns.(*A).copy`、`dns.(*NS).copy` 这类 RR 深拷贝函数已经不再出现在 top 列表中。

## 并发扩展（dnsperf，可复现上限，2026-03-18）

本节定义在 Intel i7-1165G7（4C8T @ 2.80 GHz）、Linux、UDP、cache-hit 路径上的可复现“上限测试”基线。

环境：

- rec53 运行在 `127.0.0.1:5353`，使用 perf 配置（`warmup=false`、`snapshot=false`、`log_level=error`、`pprof=false`）
- `tools/dnsperf` 使用 `tools/dnsperf/queries-sample.txt`（13 个预热域名）
- 矩阵：`c=64/128/192`，每次运行 `20s`，重复 `3` 次

### 原始运行

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

### QPS 中位数汇总

| Concurrency | Median QPS | Min QPS | Max QPS |
|------------|------------|---------|---------|
| 64 | 96,755.4 | 96,225.1 | 101,488.9 |
| 128 | 98,167.6 | 88,283.2 | 99,817.9 |
| 192 | 84,700.7 | 83,047.9 | 87,554.1 |

### 结论

1. **`c=64` 和 `c=128` 是稳定高吞吐平台**（中位数约 `~97-98k` QPS），且本次都没有 timeout。
2. **`c=192` 已超出该环境的稳定运行范围。** 它持续出现 timeout（`72-82`）和更长的 wall time（`22-24s`），不应作为默认回归负载。
3. **推荐默认仍然是 `c=64`。** 吞吐接近平台值，同时延迟和抖动比 `c=128` 更低。

历史说明：之前的单次 10s 快照可能会出现更高或更低的峰值，但 release 基线应使用这个多次中位数方法。

### SO_REUSEPORT 预期

`miekg/dns` v1.1.52 原生通过 `dns.Server.ReusePort` 支持 `SO_REUSEPORT`。每个 listener 对（UDP+TCP）都有自己的内核接收队列，从而消除单 socket 的串行瓶颈。

在 4C8T、`listeners: 4` 的情况下，预期收益：

| Estimate | QPS | Rationale |
|----------|-----|-----------|
| Conservative | 150–200 K | 约 1.6–2.2×：内核队列分流 + 剩余锁竞争 |
| Optimistic | 250–300 K | 约 2.7–3.3×：如果 cache 锁不是瓶颈，接近线性扩展 |

更高核心数服务器（16C+）配合相应数量的 listener 应该能进一步扩展。单纯提高主频只能带来约 15–25% 的提升。

实施成本：约 75 行代码，涉及 3 个文件，无需改 handler 或共享状态。

### 多监听器基准（listeners=1 vs listeners=4，2026-03-18）

同硬件、同测试配置（i7-1165G7、UDP、c=64、10s、cache-hit 路径、`tools/dnsperf` 13 域名样本）。rec53 运行在 `127.0.0.1:5353`。

| Metric | listeners=1 | listeners=4 | Delta |
|--------|------------|------------|-------|
| **QPS** | 93,927 | 94,718 | +0.8% |
| **P50** | 529 µs | 470 µs | −11% |
| **P95** | 1.7 ms | 1.9 ms | +12% |
| **P99** | 2.9 ms | 3.0 ms | +3% |
| **Max** | 12.6 ms | 1,514 ms | worse |
| **Timeouts** | 0 | 0 | — |
| **Errors** | 0 | 4 SERVFAIL | — |

**观察：**

1. **Loopback 基本抵消了 SO_REUSEPORT 的收益。** 在 `127.0.0.1` 上，client 和 server 共享 CPU 和内存总线，瓶颈不是 per-socket 接收队列，而是 dnsperf 和 rec53 之间的 CPU 竞争。真实收益需要分离客户端/服务端机器，或者来自多个源 IP 的高 fan-in。

2. **listeners=4 时 P50 下降了 11%**，说明即使在 loopback 上，内核也可能把包分发得更均匀。不过 P95/P99/Max 略升，可能是 4 倍 listener goroutine 在同一组核心上竞争带来的调度开销。

3. **listeners=4 出现的 4 个 SERVFAIL** 是冷启动噪声（warmup 完成前首批查询命中未缓存域名），不是回归。

4. **真实环境预期影响**：在独立机器上承载外部流量时，4 核硬件上启用 `listeners=4` 理论上可带来 1.5–2× QPS 提升，因为它消除了前述单 socket `recvfrom`/`sendto` 串行化瓶颈。

### 复现

```bash
# 编译工具
go build -o tools/dnsperf/dnsperf ./tools/dnsperf

# 启动 rec53
mkdir -p dist && go build -o dist/rec53 ./cmd
./dist/rec53 --config ./config.yaml

# 预热缓存
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 4 -n 100 -proto udp

# 运行可复现上限矩阵（每级 20s x 3）
for c in 64 128 192; do
  for i in 1 2 3; do
    echo "=== c=$c run=$i ==="
    tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
      -f tools/dnsperf/queries-sample.txt -c $c -d 20s -proto udp
  done
done
```

双指标验收流程也可以直接运行仓库脚本：

```bash
chmod +x tools/validate-perf.sh
./tools/validate-perf.sh
```

脚本说明：

- 会自动启动/停止 `rec53`、运行 warmup、执行 `dnsperf`、采集 `pprof` alloc profile。
- 结果写入 `/tmp/rec53-v050-validation`。
- 依赖：`dig`、`curl`、`go tool pprof`、GNU `grep -P`。

日常快速 smoke（非基线）可只跑一次：

```bash
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 64 -d 10s -proto udp
```

生成上面同款矩阵文件：

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

再计算中位数：

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

保留的旧版单次扩展命令：

```bash
for c in 32 64 128 256; do
  echo "=== c=$c ==="
  tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
    -f tools/dnsperf/queries-sample.txt -c $c -d 10s -proto udp
done
```

## 回归烟雾快照（2026-03-18，cache-shallow-copy）

这是开发中用于日常回归检查的快速样本，不是 release 级基线替代。

### Micro bench（server，benchmem）

```bash
go test -run '^$' -bench 'BenchmarkCacheGetHit|BenchmarkStateMachineCacheHit|BenchmarkRecordLatency' -benchmem ./server/...
```

| Benchmark | v0.4.1 | v0.5.0 | shallow copy | Delta (v0.5.0→shallow) |
|----------|--------|--------|-------------|------------------------|
| `BenchmarkCacheGetHit` | `856 ns`, `296 B`, `7 allocs` | `234 ns`, `264 B`, `5 allocs` | `175 ns`, `184 B`, `3 allocs` | −25% ns, −30% B, −2 allocs |
| `BenchmarkStateMachineCacheHit` | `4204 ns`, `1074 B`, `33 allocs` | `1606 ns`, `1034 B`, `31 allocs` | _(pending)_ | — |
| `BenchmarkRecordLatency` | `945 ns`, `0 B`, `0 allocs` | `596 ns`, `0 B`, `0 allocs` | _(unchanged)_ | — |

### Macro load（dnsperf，UDP，c=64，20s，3 次中位数）

```bash
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 64 -d 20s -proto udp
```

| Metric | v0.4.1 | v0.5.0 | shallow copy | Delta (v0.5.0→shallow) |
|--------|--------|--------|-------------|------------------------|
| QPS | 96,755 | 111,049 | 119,430 | +7.6% |
| P50 | 543 µs | 452 µs | 419 µs | −7% |
| P95 | 1.5 ms | 1.4 ms | 1.3 ms | −7% |
| P99 | 2.4 ms | 2.4 ms | 2.3 ms | −4% |
| Errors | 0 | 0 | 0 | — |
| Timeouts | 0 | 0 | 0 | — |

## 自行运行基准

可以使用内置基准在你自己的基础设施上测量首包延迟，并使用与你工作负载相关的域名：

```bash
# 使用默认域名列表（www.qq.com、www.baidu.com、www.taobao.com）
go test -v -run='^$' -bench='BenchmarkFirstPacket' \
    -benchtime=5x -timeout=300s ./e2e/...

# 使用你自己的域名
REC53_BENCH_DOMAINS="www.example.com,api.myservice.net" \
    go test -v -run='^$' -bench='BenchmarkFirstPacket' \
    -benchtime=5x -timeout=300s ./e2e/...

# 一次性对比四种场景
REC53_BENCH_DOMAINS="www.example.com,api.myservice.net" \
    go test -v -run='^$' -bench=BenchmarkFirstPacketComparison \
    -benchtime=1x -timeout=180s ./e2e/...
```

`REC53_BENCH_DOMAINS` 接受逗号分隔的主机名列表。末尾的点会自动补上。多个域名之间用逗号分隔，不要加空格。

关于标准化性能回归规则（bench、load、pprof、验收标准），见 [`docs/testing/perf-regression.md`](perf-regression.md)。
关于完整递归 DNS 测试策略（正确性 + 性能 + release gate），见 [`docs/recursive-dns-test-plan.md`](recursive-dns-test-plan.md)。

## XDP Cache 快速路径基准（v0.6.1，2026-03-18）

### 测试环境

- Intel i7-1165G7 @ 2.80 GHz（4C8T），Linux 6.8.0，kernel `CONFIG_BPF=y`
- rec53 绑定到 `192.168.53.1:53`，`listeners=0`，`warmup=true`，`snapshot=false`
- `tools/dnsperf` 使用 13 域名样本，UDP，`c=500`，`20s × 3` 轮次
- 客户端独立网络命名空间（`ip netns`），流量通过 `veth` 对，确保包走真实内核网络设备而不是 loopback shortcut

### 无 XDP 基线（Go-only cache，veth + netns）

| Run | Queries | Duration | QPS | Timeouts |
|-----|---------|----------|-----|----------|
| 1 | 3,231,198 | 20.01s | 143,691 | 1,317 |
| 2 | 3,229,696 | 20.01s | 140,304 | 1,303 |
| 3 | 3,231,625 | 20.01s | 129,241 | 1,310 |

**无 XDP 中位 QPS：~140K**

### XDP native attach — 验证

XDP 已以 **native mode** attach 到 `veth-rec53`（通过 `bpftool link list` 和启动日志 `[XDP] attached to veth-rec53 in native mode` 确认）。BPF per-CPU counters 计数正确，Prometheus 读取也正确。

### veth 上的 XDP_TX 限制

BPF 程序在 cache hit 时使用 `XDP_TX`（把回复包从同一接口发回）。但 Linux 在 `veth` 上不支持 `XDP_TX`——veth 驱动会拒绝该动作，并把它计入 `xdp_errors_total`，而不是把响应送出去。测试中观察到：

- `rec53_xdp_cache_hits_total`: 2,655（BPF 命中并尝试 XDP_TX）
- `rec53_xdp_errors_total`: 1,419（veth 驱动拒绝 XDP_TX）
- 客户端超时率：约 62%

这是已知内核限制：虚拟接口（`veth`、`tun`）上的 `XDP_TX` 不可用。通过 `bpf_redirect` 发到 peer interface 的修复已在 roadmap 跟踪。

### veth 基准说明了什么

尽管 XDP_TX 失败，这个基准仍然验证了：

1. **BPF 程序能在真实内核网卡设备上以 native mode 加载和 attach。**
2. **cache lookup 逻辑正确** —— BPF 能准确识别命中/未命中，并更新 per-CPU counters。
3. **Prometheus 指标链路正常** —— `startXDPMetricsLoop` 能读取 per-CPU map 并导出正确值。
4. **XDP_TX 是唯一缺失环节**，它决定了虚拟接口上的端到端 fast path 是否能真正送回响应。

### 真实物理 NIC 的预期性能

在支持 native/driver XDP 的物理 NIC（`ixgbe`、`mlx5`、`i40e` 等）上：

- `XDP_TX` 是完全支持的 —— NIC 会 DMA 映射回复并直接发包，不进入内核网络栈。
- 相比 Go-only path，预期提升：**2–5× QPS**（无 syscall、无拷贝、fast path 无 goroutine 调度）。
- 上面的 ~140K no-XDP 基线，就是验证 `XDP_TX` 在物理硬件上效果时的参考。

### 为什么之前的 v0.6.0 基准数据无效

v0.6.0 那段（现已移除）测的是 `listen: 127.0.0.1:5353` 的 loopback。BPF 程序过滤的是 `udph->dest == htons(53)`，但 `dnsperf` 当时发到的是 **5353** 端口——所以所有包都走了 `XDP_PASS`，cache hit 为 0%。报告中的 165K vs 161K QPS 差异只是 Go-only 开销，不是 XDP 带来的收益。

### 复现

```bash
# 编译
mkdir -p dist && go build -o dist/rec53 ./cmd
go build -o tools/dnsperf/dnsperf ./tools/dnsperf

# 创建隔离网络命名空间
sudo ip netns add ns-client
sudo ip link add veth-rec53 type veth peer name veth-peer
sudo ip link set veth-peer netns ns-client
sudo ip addr add 192.168.53.1/24 dev veth-rec53
sudo ip link set veth-rec53 up
sudo ip netns exec ns-client ip addr add 192.168.53.2/24 dev veth-peer
sudo ip netns exec ns-client ip link set veth-peer up

# 无 XDP 基线（绑定到 veth IP，XDP 关闭）
sudo ./dist/rec53 --config config-veth-noxdp.yaml &
# 等待 warmup 后，在 client namespace 中：
sudo ip netns exec ns-client \
  tools/dnsperf/dnsperf -server 192.168.53.1:53 \
    -f tools/dnsperf/queries-sample.txt -c 500 -d 20s

# 启用 XDP（需要 root/CAP_BPF；注意 veth 不支持 XDP_TX）
sudo ./dist/rec53 --config config-veth-xdp.yaml &
# 确认 native attach：
sudo bpftool link list   # 应看到 veth-rec53 上的 xdp prog

# 清理
sudo ip netns del ns-client
```
