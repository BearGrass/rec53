## 1. 缓存 benchmark — `server/cache_bench_test.go`

- [x] 1.1 创建 `server/cache_bench_test.go`，添加 `init()` 调用 `monitor.Rec53Log = zap.NewNop().Sugar()` 和 `monitor.InitMetricForTest()`
- [x] 1.2 实现 `BenchmarkCacheKey`：测量 `getCacheKey("example.com.", dns.TypeA)` 耗时，回归阈值 300 ns
- [x] 1.3 实现 `BenchmarkCacheGetHit`：`FlushCacheForTest()` + 植入条目 + `b.ResetTimer()` + 循环 `getCacheCopyByType`，回归阈值 1500 ns
- [x] 1.4 实现 `BenchmarkCacheGetMiss`：`FlushCacheForTest()` + `b.ResetTimer()` + 循环查找不存在的键，回归阈值 300 ns
- [x] 1.5 实现 `BenchmarkCacheSet`：循环 `setCacheCopyByType`（每次写入不同键以避免 go-cache 短路），回归阈值 3000 ns
- [x] 1.6 实现 `BenchmarkCacheConcurrent`：`b.RunParallel` 混合读写，验证并发安全（配合 `-race` 运行）
- [x] 1.7 运行 `go test -bench=BenchmarkCache -benchmem -race ./server/...` 确认全部通过

## 2. 状态机 benchmark — `server/state_machine_bench_test.go`

- [x] 2.1 创建 `server/state_machine_bench_test.go`，添加 `init()` 调用 `monitor.Rec53Log = zap.NewNop().Sugar()` 和 `monitor.InitMetricForTest()`
- [x] 2.2 实现 `BenchmarkStateMachineCacheHit`：构造 A 查询请求，预先 `setCacheCopyByType` 植入缓存，`b.ResetTimer()` 后循环 `Change(newStateInitState(req, resp, ctx))`，回归阈值 15000 ns
- [x] 2.3 实现 `BenchmarkStateInitHandle`：构造 nil Question 请求触发 FORMERR 快速返回，测量 `stateInitState.handle` 单次耗时，回归阈值 500 ns
- [x] 2.4 实现 `BenchmarkCacheLookupHit`：预先植入缓存，测量 `cacheLookupState.handle` 命中路径耗时，回归阈值 2000 ns
- [x] 2.5 实现 `BenchmarkCacheLookupMiss`：`FlushCacheForTest()` 后测量 `cacheLookupState.handle` 缺失路径耗时，回归阈值 500 ns
- [x] 2.6 运行 `go test -bench=BenchmarkStateMachine -benchmem -race ./server/...` 确认全部通过

## 3. monitor benchmark — `monitor/metric_bench_test.go`

- [x] 3.1 创建 `monitor/metric_bench_test.go`，添加 `init()` 调用 `InitMetricForTest()`
- [x] 3.2 实现 `BenchmarkInCounterAdd`：循环 `Rec53Metric.InCounterAdd("iter", "example.com.", "A")`，回归阈值 1500 ns
- [x] 3.3 实现 `BenchmarkOutCounterAdd`：循环 `Rec53Metric.OutCounterAdd("iter", "example.com.", "A", "NOERROR")`，回归阈值 1500 ns
- [x] 3.4 实现 `BenchmarkLatencyHistogram`：循环 `Rec53Metric.LatencyHistogramObserve("iter", "example.com.", "A", "NOERROR", 1.5)`，回归阈值 3000 ns
- [x] 3.5 实现 `BenchmarkIPQualityV2GaugeSet`：循环 `Rec53Metric.IPQualityV2GaugeSet("1.2.3.4", 10, 20, 30)`，回归阈值 3000 ns
- [x] 3.6 运行 `go test -bench=. -benchmem ./monitor/...` 确认全部通过

## 4. 补充 IPQualityV2.RecordFailure benchmark

- [x] 4.1 在 `server/ip_pool_test.go` 中追加 `BenchmarkRecordFailure`：使用局部 `NewIPQualityV2()`，循环调用 `RecordFailure()`，回归阈值 500 ns
- [x] 4.2 运行 `go test -bench=BenchmarkRecordFailure -benchmem ./server/...` 确认通过

## 5. 验证

- [x] 5.1 运行 `go test -bench=. -benchmem ./server/... ./monitor/...` 全套 benchmark 通过，无 panic
- [x] 5.2 运行 `go test -race ./...` 确认普通测试套件未受影响
- [x] 5.3 运行 `gofmt -l ./server/ ./monitor/` 确认格式正确
