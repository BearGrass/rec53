## MODIFIED Requirements

### Requirement: metrics.md 包含所有 Prometheus 指标定义

`docs/metrics.md` SHALL 作为 rec53 指标体系的事实来源，包含所有受支持的 Prometheus 指标定义。对每个指标族，文档 SHALL 说明指标名称、类型、Labels、用途、标签基数约束，以及“给开发者看”和“给运维/用户看”时各自应如何解读。文档 SHALL 覆盖现有 request/response/latency、IPQualityV2、XDP 基础指标，以及 `v1.1.1` 新增的 cache、snapshot、upstream、state machine 观测面。文档 SHALL 同时保留指标端点地址（`http://localhost:9999/metric`）及其通过 `-metric` CLI flag 或配置文件 `dns.metric` 字段修改的说明。

`docs/metrics.zh.md` SHALL 与 `docs/metrics.md` 同步维护，提供等价的中文指标说明、标签约束和排障入口，不得长期落后于英文版本。

#### Scenario: 指标定义覆盖新旧观测面
- **WHEN** 读者查看 `docs/metrics.md`
- **THEN** 文档 SHALL 同时列出已有指标与 `v1.1.1` 新增观测面的指标定义
- **AND** 每个指标族 SHALL 包含名称、类型、Labels、描述和基数约束

#### Scenario: 文档解释两类受众的阅读方式
- **WHEN** 读者查看某个指标族说明
- **THEN** 文档 SHALL 明确指出该指标更适合开发诊断、运维巡检，或同时服务两者

#### Scenario: 指标端点可配置说明
- **WHEN** 读者查看 `docs/metrics.md`
- **THEN** 文档 SHALL 说明指标端点地址可通过 `-metric` CLI flag 或配置文件的 `dns.metric` 字段修改

#### Scenario: 中文版指标文档同步存在
- **WHEN** `docs/metrics.md` 新增或修改指标定义、标签约束或解释分组
- **THEN** `docs/metrics.zh.md` SHALL 同步反映相同的指标范围与语义
- **AND** 中文版 SHALL 保持给开发者与给运维/用户的阅读分层

### Requirement: metrics.md 包含 PromQL 示例

`docs/metrics.md` SHALL 包含 PromQL 示例，并按“开发者诊断入口”和“运维/用户健康检查入口”分组。示例 SHALL 覆盖查询速率、错误率、P99 延迟、缓存效果、snapshot 失败、upstream 失败原因、fallback 活动、XDP 健康等核心问题，使读者能从文档直接进入排查流程。`docs/metrics.zh.md` SHALL 提供与英文版语义一致的中文 PromQL 说明。

#### Scenario: 开发者诊断示例完整
- **WHEN** 开发者查看 `docs/metrics.md`
- **THEN** 文档 SHALL 提供用于判断行为回归、失败原因变化和关键路径退化的 PromQL 示例

#### Scenario: 运维健康检查示例完整
- **WHEN** 运维或部署使用者查看 `docs/metrics.md`
- **THEN** 文档 SHALL 提供用于巡检系统健康、识别退化和确认优先排查方向的 PromQL 示例

#### Scenario: PromQL 示例不依赖高基数标签
- **WHEN** 读者复制 `docs/metrics.md` 中的 PromQL 示例
- **THEN** 示例 SHALL 只依赖受支持的低基数 labels
- **AND** SHALL NOT 假设存在 raw domain name 一类高基数标签

#### Scenario: 中文版 PromQL 说明同步
- **WHEN** 英文版指标文档补充新的 PromQL 示例或诊断入口
- **THEN** `docs/metrics.zh.md` SHALL 同步补充对应的中文说明
- **AND** PromQL 本身 SHALL 保持与英文版一致
