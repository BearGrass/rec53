## ADDED Requirements

### Requirement: WithWarmup benchmark 保留 zone 缓存
`BenchmarkFirstPacketWithWarmup` SHALL 在每次迭代中，warmup 完成后**不**调用 `FlushCacheForTest()`，确保 zone 缓存（`.com`、`.net` 等 TLD 级 NS 信息）在测量期间有效，从而真实反映生产中"warmup 后首次查询新域名"的状态。

#### Scenario: WithWarmup 不清空 zone 缓存
- **WHEN** 运行 `go test -bench=BenchmarkFirstPacketWithWarmup ./e2e/...`
- **THEN** `BenchmarkFirstPacketWithWarmup` 的实现 SHALL 不包含 `FlushCacheForTest()` 调用，且 www.qq.com 的 ms/query 结果 SHALL 低于 500ms（生产网络环境下典型值 ~268ms）

#### Scenario: WithWarmup 每次迭代重置 IPPool
- **WHEN** 运行多次迭代（`-benchtime=3x`）
- **THEN** 每次迭代开始前 SHALL 调用 `ResetIPPoolForTest()`，确保 warmup 对 IPPool 的改善在每次迭代中都重新发生，结果可重现

---

### Requirement: IPPoolOnly benchmark 独立存在且有准确文档
`BenchmarkFirstPacketIPPoolOnly` SHALL 作为独立 benchmark 存在于 `e2e/first_packet_bench_test.go` 中，测量"IPPool 已热（warmup 完成）、zone 缓存全空"的场景，并在注释中明确说明此状态在生产中不会出现，该 benchmark 的价值在于量化 IPPool 单独对 NS 选择的贡献。

#### Scenario: IPPoolOnly 在 warmup 后 flush zone cache
- **WHEN** 运行 `go test -bench=BenchmarkFirstPacketIPPoolOnly ./e2e/...`
- **THEN** `BenchmarkFirstPacketIPPoolOnly` SHALL 在 warmup 完成后调用 `FlushCacheForTest()`，清空 zone 缓存，仅保留 IPPool 延迟数据

#### Scenario: IPPoolOnly 注释说明场景局限性
- **WHEN** 阅读 `BenchmarkFirstPacketIPPoolOnly` 的 godoc 注释
- **THEN** 注释 SHALL 包含说明"此场景在生产中不存在，仅用于量化 IPPool 延迟数据对 NS 选择的单独贡献"

---

### Requirement: Comparison benchmark 覆盖四个场景
`BenchmarkFirstPacketComparison` SHALL 在输出表格中包含四列：`cold (no warmup)`、`ippool-only (warmup+flush)`、`first-pkt (full warmup)`、`cache hit`，并修复 WithWarmup 场景（不再调用 `FlushCacheForTest()`）。

#### Scenario: Comparison 输出四列结果
- **WHEN** 运行 `go test -v -run='^$' -bench=BenchmarkFirstPacketComparison -benchtime=1x ./e2e/...`
- **THEN** 输出表格 SHALL 包含四列，WithWarmup 列（full warmup）的 www.qq.com 值 SHALL 低于 500ms
