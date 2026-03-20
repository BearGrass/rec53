## Why

`v1.1.0` 已经完成性能热点复核，当前更紧迫的问题不再是“是否还能继续提速”，而是当缓存命中率下降、snapshot 恢复异常、上游抖动或 XDP 退化时，开发和运维都缺少一套能快速解释现象的统一观测面。`v1.1.1` 需要先建立低基数、可运营、可定位问题的观测基线，再继续推进运行韧性与后续特性。

## What Changes

- 明确 `v1.1.1` 的技术目标：为 `cache / snapshot / upstream / XDP / state machine` 五类关键路径建立统一、低基数、可解释的 Prometheus 指标契约。
- 明确给开发者的业务目标：让开发在排查回归、分析行为变化、比较优化前后效果时，不必先依赖临时日志或手动插桩。
- 明确给运维/用户的业务目标：让运维或部署者能回答“现在是否正常、哪里退化了、应该先看哪里”，而不是只看到总请求量和延迟。
- 为 `v1.1.1` 拆解分批范围，先补基础计数和失败原因，再补 XDP 深水区指标、状态机阶段聚合、dashboard 与 operator checklist。
- 约束标签策略与文档输出，禁止引入按 domain、完整 upstream 列表等高基数 labels，避免把观测系统本身变成新的负担。

## Capabilities

### New Capabilities
- `runtime-observability`: 为 cache、snapshot、upstream、XDP、state machine 提供低基数、可聚合、可解释的运行时指标。
- `operator-observability`: 为开发者、运维与部署使用者提供基于这些指标的 dashboard、问题定位入口和操作检查清单。

### Modified Capabilities
- `metrics-doc`: 指标文档需要从“列出已有指标”升级为“解释指标语义、标签约束、PromQL 入口和排障含义”。

## Impact

- 主要影响代码：`monitor/metric.go`、`monitor/var.go`、`server/cache.go`、`server/snapshot.go`、`server/state_query_upstream.go`、`server/xdp_metrics.go` 及相关测试。
- 主要影响文档：指标说明文档、运维文档、dashboard/operator checklist。
- 不引入新外部依赖，继续基于当前 Prometheus 指标模型与现有日志/pprof 能力扩展。
- 该变更以观测能力增强为主，不改变 rec53 递归解析、缓存、XDP fast path 的核心对外语义。
