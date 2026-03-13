# Design: 核心模块 Benchmark 套件

## Context

rec53 目前在 `server/ip_pool_test.go` 中有 5 个 IP Pool benchmark，但缓存操作（`getCacheCopyByType` / `setCacheCopyByType`）、状态机热路径（`Change()` / 各 state `handle()`）、monitor 指标上报均无 benchmark 覆盖。本次设计目标是在不引入新依赖、不修改生产代码的前提下，为三个核心 package 补全 benchmark 套件。

现有 benchmark 规范（`server/ip_pool_test.go`）已建立清晰的模式：局部实例、`b.ResetTimer()`/`b.StopTimer()`、性能回归阈值断言。新 benchmark 将严格遵循该模式。

## Goals / Non-Goals

**Goals:**

- 覆盖 `server/cache` 的单线程与并发读写热路径
- 覆盖 `server/` 状态机的缓存命中端到端路径（无网络 I/O）
- 覆盖 `monitor/` Prometheus 指标上报（`InCounterAdd`、`OutCounterAdd`、`LatencyHistogramObserve`、`IPQualityV2GaugeSet`）
- 补充 `IPQualityV2.RecordFailure` benchmark（现有 ip_pool_test.go 缺失）
- 所有 benchmark 内置性能回归阈值断言（`b.Errorf`），可作为 CI 回归门禁

**Non-Goals:**

- 不覆盖网络 I/O 路径（`queryUpstreamState` 需要真实 DNS 上游，属于集成测试范畴）
- 不引入新依赖（无 testify/bench、无第三方 benchmark 框架）
- 不修改任何生产代码
- 不强制要求 CI 中 `-bench` 默认开启（可选配置）

## Decisions

### 决策 1：在 `server/` 包创建独立 benchmark 文件，而非追加到现有测试文件

**选择**：新建 `server/cache_bench_test.go` 和 `server/state_machine_bench_test.go`。

**理由**：现有 `server/ip_pool_test.go` 已超过 400 行，`server/state_machine_test.go` 超过 500 行。将 benchmark 混入会降低可读性。独立文件便于 `go test -bench=Cache` 等精确过滤。

**备选方案**：追加到现有文件 — 否决，文件体积膨胀。

---

### 决策 2：状态机 benchmark 只覆盖无网络路径

**选择**：`Change()` 只测试缓存命中路径（预先 `setCacheCopyByType` 植入缓存）；`cacheLookupState.handle` 测试命中与缺失两个子基准。

**理由**：迭代解析路径（`queryUpstreamState`）涉及真实 UDP 连接，不适合 benchmark（不稳定、依赖网络）。缓存命中路径是生产中最高频的路径（warmup 后命中率 >95%）。

**备选方案**：mock 上游 DNS — 过度复杂，属于集成测试设计。

---

### 决策 3：`monitor/` benchmark 使用 `InitMetricForTest()` + 真实 Prometheus 注册表

**选择**：在 `monitor/metric_bench_test.go` 的 `init()` 中调用 `monitor.InitMetricForTest()`，使用真实 `Metric{}` 实例（无 HTTP 监听，但有 in-memory Prometheus registry）。

**理由**：`InitMetricForTest()` 创建 `Metric{}` 但不注册任何 collector，因此 `CounterVec.With().Inc()` 等调用在 benchmark 中是 no-op（注册为空时直接返回），可正确测量调用本身的开销。如果改用 mock，则无法反映真实 Prometheus 客户端的调用成本。

**备选方案**：使用 `prometheus/testutil` — 需要额外注册，复杂度不必要。

---

### 决策 4：性能回归阈值基于当前实测值 ×3 作为上限

**选择**：每个 benchmark 的 `b.Errorf` 阈值设为「合理预期 × 3」（宽松上限），避免硬件差异导致误报。

**理由**：过紧的阈值在不同 CPU/负载环境下误报率高，失去意义。3× 倍数足以捕捉真实性能回归（>300% 劣化必然是实质问题）。

**具体阈值参考**：

| Benchmark | 预期 p50 | 回归阈值 |
|-----------|----------|----------|
| `getCacheKey` | <100 ns | 300 ns |
| `getCacheCopyByType` 命中 | <500 ns | 1500 ns |
| `setCacheCopyByType` | <1 µs | 3 µs |
| `Change` 缓存命中 | <5 µs | 15 µs |
| `InCounterAdd` | <500 ns | 1500 ns |
| `LatencyHistogramObserve` | <1 µs | 3 µs |

## Risks / Trade-offs

- **[风险] `globalDnsCache` 全局状态污染** → 缓存 benchmark 使用 `FlushCacheForTest()` 在 `Setup` 阶段清理，或直接调用底层 `setCacheCopyByType` 植入固定数据，`b.ResetTimer()` 后再执行读取，隔离 Setup 开销。
- **[风险] `monitor.Rec53Metric` nil panic** → `server/` 包现有 `init()` 仅初始化 Logger，不调用 `InitMetricForTest()`。`state_machine_bench_test.go` 需要在 `init()` 中补充 `monitor.InitMetricForTest()`，防止状态机路径触发 metrics 时 nil panic。
- **[权衡] benchmark 文件不参与普通 `go test ./...`** → benchmark 只在 `-bench` 标志时运行，普通 CI 不受影响，不会拖慢日常测试。

## Migration Plan

无生产代码变更，无迁移步骤。新增文件可随时合并，不影响现有测试。

运行方式：

```bash
# 运行所有新 benchmark
go test -bench=. -benchmem ./server/... ./monitor/...

# 只运行缓存 benchmark
go test -bench=BenchmarkCache -benchmem ./server/...

# 只运行状态机 benchmark
go test -bench=BenchmarkStateMachine -benchmem ./server/...
```
