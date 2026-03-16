## ADDED Requirements

### Requirement: metrics.md 包含所有 Prometheus 指标定义
`docs/metrics.md` SHALL 包含从 README.md 迁移的 Prometheus 指标表格（指标名称、类型、Labels、描述），以及指标端点地址（`http://localhost:9999/metric`）。

#### Scenario: 指标定义完整迁移
- **WHEN** `docs/metrics.md` 被创建
- **THEN** README 中 6 条指标（`rec53_in_total`、`rec53_out_total`、`rec53_latency_ms`、`rec53_ipv2_p50_latency_ms`、`rec53_ipv2_p95_latency_ms`、`rec53_ipv2_p99_latency_ms`）SHALL 全部出现在该文件中

### Requirement: metrics.md 包含 PromQL 示例
`docs/metrics.md` SHALL 包含从 README.md 迁移的所有 PromQL 查询示例（Query rate、Error rate、P99 latency、Degraded nameservers）。

#### Scenario: PromQL 示例完整迁移
- **WHEN** `docs/metrics.md` 被创建
- **THEN** README 中 4 条 PromQL 示例 SHALL 全部出现在该文件中，语法不被修改

#### Scenario: 指标端点可配置说明
- **WHEN** 读者查看 metrics.md
- **THEN** 文件 SHALL 说明指标端点地址可通过 `-metric` CLI flag 或配置文件的 `dns.metric` 字段修改
