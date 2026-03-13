# Proposal: 为核心模块添加 Benchmark 套件

## Why

rec53 的核心热路径（缓存读写、状态机、IP 选路、Prometheus 指标上报）目前缺乏系统性的性能基准覆盖。现有 `server/ip_pool_test.go` 中已有 5 个 IP Pool benchmark，但缓存操作、状态机路径和 monitor 包均无 benchmark，无法在重构后量化性能回归或提升。

## What Changes

- **新增** `server/cache_bench_test.go`：覆盖 `getCacheCopyByType`（缓存命中/缺失）、`setCacheCopyByType`、`getCacheKey`，含并发读写场景
- **新增** `server/state_machine_bench_test.go`：覆盖端到端 `Change()` 缓存命中路径、`stateInitState.handle`（FORMERR 快速路径）、`cacheLookupState.handle`（命中/缺失）、`classifyRespState.handle`（各分支）
- **新增** `monitor/metric_bench_test.go`：覆盖 `InCounterAdd`、`OutCounterAdd`、`LatencyHistogramObserve`、`IPQualityV2GaugeSet`
- **补充** `server/ip_pool_test.go`：为 `IPQualityV2.RecordFailure` 添加缺失的 benchmark

所有 benchmark 遵循现有规范：局部实例隔离、`b.ResetTimer()`/`b.StopTimer()` 精确计时、内置性能回归断言（`b.Errorf`）。

## Capabilities

### New Capabilities

- `core-benchmarks`：为 cache、状态机、monitor 核心热路径建立可重复运行的性能基准，并配套性能回归阈值断言

### Modified Capabilities

（无——本变更不修改任何生产行为，仅新增测试文件）

## Impact

- 影响代码：`server/cache_bench_test.go`（新增）、`server/state_machine_bench_test.go`（新增）、`monitor/metric_bench_test.go`（新增）、`server/ip_pool_test.go`（补充）
- 无 API 变更、无依赖变更、无生产代码修改
- CI 可选择性运行：`go test -bench=. -benchmem ./server/... ./monitor/...`
