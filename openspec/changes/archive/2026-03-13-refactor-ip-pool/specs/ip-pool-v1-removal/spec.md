## REMOVED Requirements

### Requirement: V1 IP 质量跟踪
IPPool 原使用简单原子变量（`IPQuality`）跟踪每个 IP 的延迟，并通过 `getBestIPs` 进行选路；通过 `GetPrefetchIPs`/`PrefetchIPs`/`prefetchIPQuality` 对候选 IP 进行主动探测预热；通过 Prometheus 指标 `rec53_ip_quality` 上报延迟数据。

**Reason**: V1 实现已被 `IPQualityV2`（滑动窗口百分位延迟 + 状态机）完全替代。`GetBestIPsV2` 为唯一生产选路路径；V1 prefetch 链因 `IPPool.pool`（V1 map）从未被 V2 写入而成为 no-op；`rec53_ip_quality` 指标已由 `rec53_ipv2_p50/p95/p99_latency_ms` 替代。

**Migration**: 无需迁移。`GetBestIPsV2` 已完整替代 V1 选路；IP 质量监控改用 `rec53_ipv2_p50_latency_ms`、`rec53_ipv2_p95_latency_ms`、`rec53_ipv2_p99_latency_ms`。

#### Scenario: V1 选路不再可用
- **WHEN** 代码尝试调用 `IPPool.getBestIPs`、`GetIPQuality`、`SetIPQuality`、`UpIPsQuality`
- **THEN** 编译失败，因为这些方法已被删除

#### Scenario: V1 prefetch 不再可用
- **WHEN** 代码尝试调用 `IPPool.GetPrefetchIPs` 或 `IPPool.PrefetchIPs`
- **THEN** 编译失败，因为这些方法已被删除

#### Scenario: V1 Prometheus 指标停止上报
- **WHEN** rec53 运行时
- **THEN** `rec53_ip_quality` 指标不再出现在 `/metrics` 端点；`rec53_ipv2_p50_latency_ms` 等 V2 指标继续正常上报
