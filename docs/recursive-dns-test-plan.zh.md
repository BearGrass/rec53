# 递归 DNS 测试计划

[English](recursive-dns-test-plan.md) | 中文

本文定义了 rec53 作为递归 DNS 解析器时的一套完整、可重复的测试策略。它适合日常研发持续使用，而不只是一次性的 release 验证。

## 1. 目标

- 验证解析正确性（答案、委派、CNAME、负响应）。
- 发现并发安全（`-race`）和稳定性回归。
- 使用可复现的方法跟踪性能趋势（延迟/QPS/分配）。
- 为 PR 和 release 决策提供可比证据。

## 1.1 测试文档与工具映射（单一事实源）

| Item | Role | Canonical Scope |
|------|------|-----------------|
| `docs/recursive-dns-test-plan.md` | Master test strategy | 分层、release gate、产物、职责边界 |
| `docs/testing/perf-regression.md` | Performance execution protocol | 精确 perf 命令、验收阈值、可复现矩阵方法 |
| `docs/benchmarks.md` | Baseline snapshots | 只记录测量值（不做策略决策） |
| `tools/dnsperf` | Load generator | 宏观负载测试（`-f` replay / `-random-prefix` miss stress） |
| `tools/validate-perf.sh` | Dual-metric automation | 一键 dnsperf + pprof 验证流程 |

防漂移规则：

- 如果 perf 命令模板或阈值发生变化，先更新 `docs/testing/perf-regression.md`，再在同一 commit 里更新本文和 `docs/benchmarks.md`。
- 不要在多个文档里复制出不同版本的命令；尽量引用权威命令。

## 2. 测试层级

| Layer | Purpose | Command / Scope | Trigger |
|------|---------|------------------|---------|
| Unit | 状态逻辑和 helper 正确性 | `go test ./server/... ./utils/...` | 每个 PR |
| Race | 并发安全 | `go test -race ./...` | 每个性能/并发 PR；release 前 |
| Integration/E2E | 端到端解析行为 | `go test -v ./e2e/...` | 触及解析流程的每个 PR |
| Bench (micro) | 热路径延迟和分配 | `go test -run '^$' -bench . -benchmem ./server/... ./monitor/...` | 性能敏感 PR |
| Load (macro) | 服务级吞吐和尾延迟 | `tools/dnsperf/dnsperf ...` | 性能敏感 PR + release |
| Profiling | 热点归因 | `go tool pprof ...` 在负载下采集 | load 后若发现回归 |

## 2.1 工具标准（`tools/`）

- 每次做负载测试前先重新构建 perf 工具：

```bash
go build -o tools/dnsperf/dnsperf ./tools/dnsperf
```

- `tools/dnsperf` 模式：
  - Replay 模式（`-f tools/dnsperf/queries-sample.txt`）：适合稳定 cache-hit 和可复现回归比较。
  - Random-prefix 模式（`-random-prefix example.com`）：cache-miss / iterative 压测。
- `tools/validate-perf.sh`：
  - 面向 Linux 的双指标门槛辅助脚本。
  - 依赖 `dig`、`curl`、`go tool pprof`、GNU `grep -P`。
  - 输出写入 `/tmp/rec53-perf-validation`。

## 3. 功能覆盖清单

最低功能场景：

- 从 root 到 authoritative server 的迭代解析。
- A/AAAA/CNAME/NS 的 cache hit/miss 行为。
- CNAME 链处理（包括跨 zone 切换）。
- 负缓存行为（NXDOMAIN/NODATA + SOA TTL）。
- 无 glue 的 NS 委派处理。
- forwarding 与 hosts 的优先级（`hosts > forwarding > cache > iterative`）。
- UDP 截断和 TCP 路径行为。
- 优雅关闭 + snapshot 恢复行为。

## 4. 性能覆盖清单

性能 PR 必备 benchmark：

- `BenchmarkCacheGetHit`
- `BenchmarkStateMachineCacheHit`
- `BenchmarkRecordLatency`

性能 PR 必备 load profile：

- `dnsperf` replay profile：`c=64` 和 `c=128`，UDP，每个 case `20s`。
- release baseline 刷新时，运行固定矩阵 `c=64/128/192`，每级 `20s x 3`，使用各级中位数 QPS。
- 可选 miss-stress：`-random-prefix example.com`。
- 记录 QPS、P50、P95、P99、errors、timeouts。

必备 pprof：

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

## 5. 执行配置

### PR 快速门（开发循环）

```bash
go test ./server/... ./utils/...
go test -race ./server/...
go test -run '^$' -bench 'BenchmarkCacheGetHit|BenchmarkStateMachineCacheHit|BenchmarkRecordLatency' -benchmem ./server/...
go build -o tools/dnsperf/dnsperf ./tools/dnsperf
tools/dnsperf/dnsperf -server 127.0.0.1:5353 -f tools/dnsperf/queries-sample.txt -c 64 -d 10s -proto udp
```

### 性能 PR 门

```bash
go test -race ./...
go test -run '^$' -bench . -benchmem ./server/... ./monitor/... ./e2e/...
go build -o tools/dnsperf/dnsperf ./tools/dnsperf
tools/dnsperf/dnsperf -server 127.0.0.1:5353 -f tools/dnsperf/queries-sample.txt -c 64 -d 20s -proto udp
tools/dnsperf/dnsperf -server 127.0.0.1:5353 -f tools/dnsperf/queries-sample.txt -c 128 -d 20s -proto udp
# optional miss-stress
tools/dnsperf/dnsperf -server 127.0.0.1:5353 -random-prefix example.com -c 32 -d 20s -proto udp
```

### Release 门

- 跑完整 race suite：`go test -race ./...`。
- 跑完整 e2e suite：`go test -v ./e2e/...`。
- 跑性能门（bench + load + pprof），如有变化则更新基线。
- 如果使用自动化封装，允许 `tools/validate-perf.sh`，但要附上原始输出。

## 6. 通过/失败规则

- 正确性测试：零失败。
- Race 测试：零 race 报告。
- Load 测试：errors/timeouts 不应出现无法解释的增长。
- Benchmarks：热路径 `ns/op` 退化超过 10% 需要说明。
- 分配优化：目标 benchmark 的 `allocs/op` 不能回退。

任何例外都必须在 PR 说明里写清楚根因和后续动作。

## 7. 证据和产物

性能敏感变更需要附上：

- benchmark 输出（`-benchmem`）前后对比。
- 必须并发级别下的 `dnsperf` 汇总输出。
- 去噪后的 `pprof -top` 输出（CPU + alloc_space）。
- 环境元数据：Go 版本、CPU 型号、config profile。

基线快照保存在 [`docs/benchmarks.md`](benchmarks.md)，执行规则保存在 [`docs/testing/perf-regression.md`](perf-regression.md)。

需要保留的测试输出（按需）：

- `dnsperf` 运行输出（`/tmp/dnsperf-runs/*.txt` 或 CI artifact）。
- 可复现矩阵表（`/tmp/dnsperf_matrix.tsv`）。
- 使用脚本时的 v0.5.0 产物（`/tmp/rec53-v050-validation/*`）。

## 8. 可持续规则（研发流程）

- 迭代期间尽量保持相同的命令集和 query 文件，除非有意修改。
- 如果方法变了（query set、duration、filters），要在同一个 commit 里更新文档。
- 绝不把未经验证的数字写进 benchmark 表。
- 如果本地环境跑不了某项，要明确标记为未运行。
- 报告里要明确工具模式：replay（`-f`）还是 miss-stress（`-random-prefix`）、协议、duration、concurrency。
