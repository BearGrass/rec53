# 性能回归规则

[English](perf-regression.md) | 中文

本文定义 rec53 的标准性能回归流程。它适用于性能敏感变更（缓存、状态机、IP 池、指标、网络和 pprof 相关代码）。

完整递归 DNS 覆盖（功能、e2e、release gate）见 [`docs/testing/recursive-dns-test-plan.md`](recursive-dns-test-plan.md)。

## 1）前置条件

- 比较前后结果时，尽量在相同环境中构建。
- 比较期间保持 `config.yaml` 不变。
- 在 PR / change note 里记录 Go 版本（`go version`）和 CPU 型号。
- 每次运行前都重新构建 perf 工具（不要相信旧二进制）：

```bash
go build -o tools/dnsperf/dnsperf ./tools/dnsperf
```

## 1.1）`tools/` 工具标准

- `tools/dnsperf`：
  - 本仓库的主负载工具。
  - 模式：
    - 回放模式：`-f tools/dnsperf/queries-sample.txt`
    - cache-miss 压测：`-random-prefix example.com`
  - 支持 UDP/TCP（`-proto`）、时长模式（`-d`）、次数模式（`-n`）以及可选速率限制（`-qps`）。
- `tools/validate-perf.sh`：
  - 双指标门槛的一键脚本（dnsperf + pprof）。
  - 输出目录：`/tmp/rec53-perf-validation`。
  - 前置依赖：`dig`、`curl`、`go tool pprof`、支持 `-P` 的 GNU `grep`。
  - 适合 Linux 上的开发/CI 验证；如果环境缺少这些依赖，请改用本文中的手动命令。

## 2）正确性门槛（必须先通过）

```bash
go test -race ./...
```

如果 race 测试失败，不要接受性能数据。

## 3）基准门槛（micro）

运行带 allocation 指标的 benchmark：

```bash
go test -run '^$' -bench . -benchmem ./server/...
go test -run '^$' -bench . -benchmem ./monitor/...
go test -run '^$' -bench . -benchmem ./e2e/...
```

必须关注：

- 热路径变更时，`allocs/op` 不应增加，除非有充分理由。
- 热路径变更时，`ns/op` 退化超过 10% 需要明确说明。
- 必须至少给出以下 benchmark 的前后数值：
  - `BenchmarkCacheGetHit`
  - `BenchmarkStateMachineCacheHit`
  - `BenchmarkRecordLatency`

## 4）负载门槛（macro，dnsperf）

使用 `tools/dnsperf` 做网络级回归检查：

```bash
go build -o tools/dnsperf/dnsperf ./tools/dnsperf
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -f tools/dnsperf/queries-sample.txt -c 128 -d 20s -proto udp
```

可选的 cache-miss 压测配置：

```bash
tools/dnsperf/dnsperf -server 127.0.0.1:5353 \
  -random-prefix example.com -c 32 -d 20s -proto udp
```

必须关注：

- 报告 QPS、P50、P95、P99、errors 和 timeouts。
- 不得出现无法解释的 error/timeout 增长。
- 如果改的是并发/网络逻辑，至少包括两个并发级别（例如 `c=64` 和 `c=128`）。

### 可复现上限 profile（release baseline）

更新性能基准文档时，不要只跑单次 load，而要跑下面的固定矩阵：

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

验收/汇报要求：

- 每个并发级别使用中位数 QPS 作为基线值。
- 记录 min/max QPS 范围（run-to-run jitter）。
- 记录 timeout/error 数量和 wall-time 漂移（例如目标 20s 的运行在过载时拉长到 24-25s）。
- 默认稳定负载级别是 `c=64`；如果更高并发级别增加 timeout 率，就不要把它们当基线。

## 5）pprof 门槛（性能 PR 必做）

在真实负载下采集去噪 CPU 和 allocation profile：

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

必须关注：

- 解释 `ServeDNS -> Change -> cacheLookup` 相关路径的热点变化。
- `alloc_space` 是累计分配，不是 RSS，解释时要按这个口径。
- 如果是分配优化 PR，要给出 hotspot 百分比的前后对比。

## 6）基线来源

把 [`docs/benchmarks.md`](docs/benchmarks.md) 作为基线快照。
如果新的测量显著改变了预期区间，请在同一个 commit 里更新该文件。

## 7）可持续规则

- 对比时保持命令集和 query corpus 稳定。
- 如果改了测试参数，也要在同一个 commit 里更新方法文档。
- 如果环境限制导致某些项没跑，必须明确写出 “not run”。
- 基线快照保存在 [`docs/benchmarks.md`](docs/benchmarks.md)，而本文档是可复现流程的权威来源。
- 更新 perf 文档时，必须明确工具模式（`-f` vs `-random-prefix`）、协议和完整命令行，方便他人复现。
