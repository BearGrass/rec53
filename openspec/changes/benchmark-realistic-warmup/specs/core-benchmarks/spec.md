## MODIFIED Requirements

### Requirement: E2E 首包 benchmark 场景语义
`e2e/first_packet_bench_test.go` SHALL 提供以下四个独立场景的 benchmark，每个场景的前置状态 SHALL 与其名称和注释准确对应：
1. `BenchmarkFirstPacketNoWarmup`：IPPool 冷 + zone 缓存空（冷启动最坏情况）
2. `BenchmarkFirstPacketIPPoolOnly`：IPPool 热（warmup 完成）+ zone 缓存空（通过 `FlushCacheForTest()` 清空）——此场景在生产中不存在，仅用于量化 IPPool 单独贡献
3. `BenchmarkFirstPacketWithWarmup`：IPPool 热 + zone 缓存热（不调用 `FlushCacheForTest()`）——生产典型首包场景
4. `BenchmarkFirstPacketCacheHit`：完整缓存命中（基线对比）

#### Scenario: WithWarmup 不调用 FlushCacheForTest
- **WHEN** 阅读 `BenchmarkFirstPacketWithWarmup` 的实现代码
- **THEN** 代码 SHALL 不包含 `FlushCacheForTest()` 调用

#### Scenario: IPPoolOnly 调用 FlushCacheForTest
- **WHEN** 阅读 `BenchmarkFirstPacketIPPoolOnly` 的实现代码
- **THEN** 代码 SHALL 在 warmup 完成后调用 `FlushCacheForTest()`

#### Scenario: 四个 benchmark 均存在
- **WHEN** 运行 `go test -list='BenchmarkFirstPacket' ./e2e/...`
- **THEN** 输出 SHALL 包含 `BenchmarkFirstPacketNoWarmup`、`BenchmarkFirstPacketIPPoolOnly`、`BenchmarkFirstPacketWithWarmup`、`BenchmarkFirstPacketCacheHit` 四个名称
