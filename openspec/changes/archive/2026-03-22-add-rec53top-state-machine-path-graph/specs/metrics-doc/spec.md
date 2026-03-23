## MODIFIED Requirements

### Requirement: metrics.md 包含所有 Prometheus 指标定义
`docs/metrics.md` SHALL 包含从 README.md 迁移的 Prometheus 指标表格（指标名称、类型、Labels、描述），以及指标端点地址（`http://localhost:9999/metric`）。当系统新增 Prometheus 指标时，该文件 SHALL 同步描述新增指标的名称、标签和含义，包括状态机转移指标。

#### Scenario: 指标定义完整迁移
- **WHEN** `docs/metrics.md` 被创建
- **THEN** README 中 6 条指标（`rec53_in_total`、`rec53_out_total`、`rec53_latency_ms`、`rec53_ipv2_p50_latency_ms`、`rec53_ipv2_p95_latency_ms`、`rec53_ipv2_p99_latency_ms`）SHALL 全部出现在该文件中

#### Scenario: 新增状态机转移指标被记录
- **WHEN** 系统引入 `rec53_state_machine_transition_total`
- **THEN** `docs/metrics.md` SHALL 说明该指标的名称、`from`/`to` 标签以及它表示真实状态转移边而非 stage 热度

#### Scenario: 指标端点可配置说明
- **WHEN** 读者查看 metrics.md
- **THEN** 文件 SHALL 说明指标端点地址可通过 `-metric` CLI flag 或配置文件的 `dns.metric` 字段修改
