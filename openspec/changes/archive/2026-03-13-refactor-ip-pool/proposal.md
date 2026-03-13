## Why

`server/ip_pool.go` 存在两个可读性问题：一是文件单体过长（665 行，包含三个职责不同的数据结构），二是 V1 `IPQuality` 及其相关方法已被 V2 完全替代但仍滞留在代码库中，形成死代码，增加维护负担。现在是清理的最佳时机，因为分析已确认 V1 生产路径调用为零。

## What Changes

- 将 `ip_pool.go` 拆分为两个文件：`ip_pool_quality_v2.go`（IPQualityV2 及其方法）和 `ip_pool.go`（IPPool 及其方法）
- 删除 V1 `IPQuality` 类型及其全部方法（`NewIPQuality`、`Init`、`IsInit`、`GetLatency`、`SetLatency`、`SetLatencyAndState`）
- 删除 V1 IPPool 方法：`isTheIPInit`、`GetIPQuality`、`SetIPQuality`、`updateIPQuality`、`UpIPsQuality`、`getBestIPs`
- 删除 V1 prefetch 链：`GetPrefetchIPs`、`PrefetchIPs`、`prefetchIPQuality`（及 `IPPool.pool` 字段、`NewIPPool` 中的 V1 初始化）
- 删除 `state_query_upstream.go` 中对 `GetPrefetchIPs`/`PrefetchIPs` 的调用（实质 no-op）
- 删除 `monitor` 包中的 V1 Prometheus 指标：`IPQuality` GaugeVec 和 `IPQualityGaugeSet` 方法
- 删除所有 V1 相关测试代码（`ip_pool_test.go`、`state_query_upstream_test.go` 中的 V1 测试函数）

## Capabilities

### New Capabilities

无新能力引入，本次为纯重构。

### Modified Capabilities

无规格级别的行为变更，所有变更均为实现细节层面的删减与重组。

## Impact

- **`server/ip_pool.go`**：拆分为 `ip_pool.go` 和 `ip_pool_quality_v2.go`，删除全部 V1 代码
- **`server/state_query_upstream.go`**：删除 `getBestAddressAndPrefetchIPs` 内的 2 行 V1 prefetch 调用
- **`monitor/metric.go`**：删除 `IPQualityGaugeSet` 方法
- **`monitor/var.go`**（或等效文件）：删除 `IPQuality` GaugeVec 注册
- **`server/ip_pool_test.go`**：删除 V1 相关测试函数（约 200 行）
- **`server/state_query_upstream_test.go`**：删除直接操作 V1 pool 的测试辅助代码
- **`e2e/`**：不受影响（仅使用 V2 公开 API）
- **破坏性变更**：`GetIPQuality`、`SetIPQuality`、`UpIPsQuality`、`GetPrefetchIPs`、`PrefetchIPs` 这些导出函数将被删除；**BREAKING**（仅影响测试代码，无外部消费者）
- **Prometheus 指标**：`rec53_ip_quality` 指标将停止上报（已由 `rec53_ipv2_p50/p95/p99_latency_ms` 替代）
