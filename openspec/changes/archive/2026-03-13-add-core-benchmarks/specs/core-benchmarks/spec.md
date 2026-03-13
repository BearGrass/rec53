## ADDED Requirements

### Requirement: 缓存热路径有 benchmark 覆盖
`server/` 包 SHALL 提供独立文件 `server/cache_bench_test.go`，覆盖 `getCacheKey`、`getCacheCopyByType`（命中与缺失）、`setCacheCopyByType` 以及并发读写场景的 benchmark 函数。每个 benchmark SHALL 调用 `b.ResetTimer()` 排除 Setup 开销，并通过 `b.Errorf` 内置性能回归阈值断言。

#### Scenario: 缓存键生成基准
- **WHEN** 运行 `go test -bench=BenchmarkCacheKey ./server/...`
- **THEN** `BenchmarkCacheKey` 函数 SHALL 存在于 `server/cache_bench_test.go` 中，测量 `getCacheKey` 的每次调用耗时，回归阈值 SHALL 为 300 ns

#### Scenario: 缓存命中读取基准
- **WHEN** 运行 `go test -bench=BenchmarkCacheGetHit ./server/...`
- **THEN** `BenchmarkCacheGetHit` SHALL 预先植入缓存条目，测量 `getCacheCopyByType` 命中路径（含 `msg.Copy()`）耗时，回归阈值 SHALL 为 1500 ns

#### Scenario: 缓存缺失读取基准
- **WHEN** 运行 `go test -bench=BenchmarkCacheGetMiss ./server/...`
- **THEN** `BenchmarkCacheGetMiss` SHALL 测量 `getCacheCopyByType` 在键不存在时的耗时，回归阈值 SHALL 为 300 ns

#### Scenario: 缓存写入基准
- **WHEN** 运行 `go test -bench=BenchmarkCacheSet ./server/...`
- **THEN** `BenchmarkCacheSet` SHALL 测量 `setCacheCopyByType`（含 `msg.Copy()`）的耗时，回归阈值 SHALL 为 3000 ns

#### Scenario: 并发读写基准
- **WHEN** 运行 `go test -bench=BenchmarkCacheConcurrent ./server/...`
- **THEN** `BenchmarkCacheConcurrent` SHALL 使用 `b.RunParallel` 模拟多协程混合读写，验证 `sync.RWMutex` 并发下无竞争（配合 `-race` 运行）

---

### Requirement: 状态机热路径有 benchmark 覆盖
`server/` 包 SHALL 提供独立文件 `server/state_machine_bench_test.go`，覆盖端到端 `Change()` 缓存命中路径、`stateInitState.handle` 快速失败路径、`cacheLookupState.handle` 命中与缺失路径、`classifyRespState.handle` 各响应分支。文件 `init()` SHALL 调用 `monitor.InitMetricForTest()` 防止 nil panic。

#### Scenario: 端到端缓存命中基准
- **WHEN** 运行 `go test -bench=BenchmarkStateMachineCacheHit ./server/...`
- **THEN** `BenchmarkStateMachineCacheHit` SHALL 预先植入缓存，测量完整 `Change(stm)` 调用（STATE_INIT → CACHE_LOOKUP → RETURN_RESP）耗时，回归阈值 SHALL 为 15000 ns

#### Scenario: 请求验证快速失败基准
- **WHEN** 运行 `go test -bench=BenchmarkStateInitHandle ./server/...`
- **THEN** `BenchmarkStateInitHandle` SHALL 构造 nil request 触发 FORMERR 返回，测量 `stateInitState.handle` 的最短路径耗时，回归阈值 SHALL 为 500 ns

#### Scenario: 缓存查找命中基准
- **WHEN** 运行 `go test -bench=BenchmarkCacheLookupHit ./server/...`
- **THEN** `BenchmarkCacheLookupHit` SHALL 预先植入缓存，测量 `cacheLookupState.handle` 命中路径耗时，回归阈值 SHALL 为 2000 ns

#### Scenario: monitor 初始化防 nil panic
- **WHEN** 在 `server/state_machine_bench_test.go` 的 `init()` 中未调用 `monitor.InitMetricForTest()`，且 benchmark 触发 metrics 路径
- **THEN** 运行时 SHALL 产生 nil pointer panic — 因此该文件的 `init()` 中 SHALL 调用 `monitor.InitMetricForTest()`

---

### Requirement: monitor 指标上报有 benchmark 覆盖
`monitor/` 包 SHALL 提供独立文件 `monitor/metric_bench_test.go`，覆盖 `InCounterAdd`、`OutCounterAdd`、`LatencyHistogramObserve`、`IPQualityV2GaugeSet` 四个核心方法的 benchmark。文件 `init()` SHALL 调用 `InitMetricForTest()`。

#### Scenario: 入站计数器上报基准
- **WHEN** 运行 `go test -bench=BenchmarkInCounterAdd ./monitor/...`
- **THEN** `BenchmarkInCounterAdd` SHALL 测量 `Rec53Metric.InCounterAdd` 每次调用耗时，回归阈值 SHALL 为 1500 ns

#### Scenario: 出站计数器上报基准
- **WHEN** 运行 `go test -bench=BenchmarkOutCounterAdd ./monitor/...`
- **THEN** `BenchmarkOutCounterAdd` SHALL 测量 `Rec53Metric.OutCounterAdd`（4 个 label）每次调用耗时，回归阈值 SHALL 为 1500 ns

#### Scenario: 延迟直方图上报基准
- **WHEN** 运行 `go test -bench=BenchmarkLatencyHistogram ./monitor/...`
- **THEN** `BenchmarkLatencyHistogram` SHALL 测量 `Rec53Metric.LatencyHistogramObserve` 每次调用耗时，回归阈值 SHALL 为 3000 ns

#### Scenario: IP 质量 Gauge 上报基准
- **WHEN** 运行 `go test -bench=BenchmarkIPQualityV2GaugeSet ./monitor/...`
- **THEN** `BenchmarkIPQualityV2GaugeSet` SHALL 测量 `Rec53Metric.IPQualityV2GaugeSet`（设置 p50/p95/p99 三个 gauge）每次调用耗时，回归阈值 SHALL 为 3000 ns

---

### Requirement: IPQualityV2.RecordFailure 有 benchmark 覆盖
`server/ip_pool_test.go` SHALL 补充 `BenchmarkRecordFailure` benchmark，覆盖 `IPQualityV2.RecordFailure` 的三阶段状态机路径（healthy → degraded → failed）。

#### Scenario: 故障记录基准
- **WHEN** 运行 `go test -bench=BenchmarkRecordFailure ./server/...`
- **THEN** `BenchmarkRecordFailure` SHALL 存在于 `server/ip_pool_test.go`，使用局部 `NewIPQualityV2()` 实例，测量连续调用 `RecordFailure` 的均摊耗时，回归阈值 SHALL 为 500 ns
