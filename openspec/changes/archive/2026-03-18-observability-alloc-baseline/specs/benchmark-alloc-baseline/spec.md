## ADDED Requirements

### Requirement: 全部 benchmark 报告内存分配指标
项目中所有 `Benchmark*` 函数 SHALL 在函数体内调用 `b.ReportAllocs()`，确保 `go test -bench` 输出包含 `allocs/op` 和 `B/op` 指标，无需依赖调用者传递 `-benchmem` flag。

#### Scenario: server/ip_pool_test.go 的 6 个 benchmark 报告分配
- **WHEN** 运行 `go test -bench=. ./server/... -run=^$` 并查看 `ip_pool_test.go` 中的 benchmark 输出
- **THEN** `BenchmarkRecordLatency`、`BenchmarkGetBestIPsV2`、`BenchmarkIPPoolSelection`、`BenchmarkRecordFailure`、`BenchmarkIPPoolPruneConcurrent`、`BenchmarkIPPoolProbeAndPrune` 的输出 SHALL 包含 `allocs/op` 和 `B/op` 列

#### Scenario: server/state_machine_bench_test.go 的 4 个 benchmark 报告分配
- **WHEN** 运行 `go test -bench=. ./server/... -run=^$` 并查看 `state_machine_bench_test.go` 中的 benchmark 输出
- **THEN** `BenchmarkStateMachineCacheHit`、`BenchmarkStateInitHandle`、`BenchmarkCacheLookupHit`、`BenchmarkClassifyRespHandle` 的输出 SHALL 包含 `allocs/op` 和 `B/op` 列

#### Scenario: server/cache_bench_test.go 的 6 个 benchmark 报告分配
- **WHEN** 运行 `go test -bench=. ./server/... -run=^$` 并查看 `cache_bench_test.go` 中的 benchmark 输出
- **THEN** `BenchmarkCacheKey`、`BenchmarkCacheGetHit`、`BenchmarkCacheGetMiss`、`BenchmarkCacheSet`、`BenchmarkCacheConcurrent`、`BenchmarkCacheTTLExpiry` 的输出 SHALL 包含 `allocs/op` 和 `B/op` 列

#### Scenario: e2e/first_packet_bench_test.go 的 5 个 benchmark 报告分配
- **WHEN** 运行 `go test -bench=. ./e2e/... -run=^$` 并查看 `first_packet_bench_test.go` 中的 benchmark 输出
- **THEN** 该文件中的全部 5 个 benchmark 函数的输出 SHALL 包含 `allocs/op` 和 `B/op` 列

#### Scenario: e2e/error_test.go 的 BenchmarkIntegrationQuery 报告分配
- **WHEN** 运行 `go test -bench=BenchmarkIntegrationQuery ./e2e/... -run=^$`
- **THEN** `BenchmarkIntegrationQuery` 的输出 SHALL 包含 `allocs/op` 和 `B/op` 列

#### Scenario: monitor/metric_bench_test.go 的 4 个 benchmark 报告分配
- **WHEN** 运行 `go test -bench=. ./monitor/... -run=^$` 并查看 `metric_bench_test.go` 中的 benchmark 输出
- **THEN** `BenchmarkInCounterAdd`、`BenchmarkOutCounterAdd`、`BenchmarkLatencyHistogram`、`BenchmarkIPQualityV2GaugeSet` 的输出 SHALL 包含 `allocs/op` 和 `B/op` 列
