## MODIFIED Requirements

### Requirement: IPQualityV2.RecordFailure 有 benchmark 覆盖
`server/ip_pool_test.go` SHALL 补充 `BenchmarkRecordFailure` benchmark，覆盖 `IPQualityV2.RecordFailure` 的三阶段状态机路径（healthy → degraded → failed）。

#### Scenario: 故障记录基准
- **WHEN** 运行 `go test -bench=BenchmarkRecordFailure ./server/...`
- **THEN** `BenchmarkRecordFailure` SHALL 存在于 `server/ip_pool_test.go`，使用局部 `NewIPQualityV2()` 实例，测量连续调用 `RecordFailure` 的均摊耗时，回归阈值 SHALL 为 500 ns

#### Scenario: RecordLatency benchmark 分配基线验证
- **WHEN** 运行 `go test -bench=BenchmarkRecordLatency ./server/... -run=^$` 并查看输出
- **THEN** `BenchmarkRecordLatency` 的输出 SHALL 包含 `allocs/op` 和 `B/op` 列
- **AND** `updatePercentiles` 优化后，`allocs/op` SHALL 显著低于优化前基线（优化前基线：3 allocs/op）
