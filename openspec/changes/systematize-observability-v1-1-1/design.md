## Context

rec53 当前已经具备基础 Prometheus 指标、IPQualityV2 延迟分位和 XDP 基础状态，但这些信号主要回答“流量有没有进来、总延迟高不高”。对于 `cache / snapshot / upstream / XDP / state machine` 这几条真正影响运行质量的路径，系统还缺少统一、低基数、可聚合的解释面。

这会同时伤害两类使用者：

- 开发者看不到“行为变化发生在哪一层”，排查回归时容易退回临时日志和一次性插桩。
- 运维或部署使用者看不到“现在是正常、退化还是异常”，只能从 SERVFAIL 或超时结果倒推问题。

这次设计是一个跨 `monitor/`、`server/` 和文档面的观测系统化收敛，约束如下：

- 不引入 tracing 平台、新时序后端或新的外部依赖。
- 不改变请求处理、缓存、snapshot、XDP 的核心业务语义。
- 指标必须保持低基数，不能为了“更细”牺牲可运营性。
- 文档输出必须按受众区分，明确开发者与运维/用户各自首先该看什么。

## Goals / Non-Goals

**Goals:**

- 建立 `cache / snapshot / upstream / XDP / state machine` 五类运行时指标契约。
- 明确技术目标：关键路径能以 Prometheus 指标回答“是否正常、哪里退化、退化原因是什么”。
- 明确给开发者的业务目标：出现行为回归时，可以先从稳定指标判断问题层次，而不是先打补丁式日志。
- 明确给运维/用户的业务目标：出现缓存失效、snapshot 异常、上游波动或 XDP 退化时，可以快速定位优先检查项。
- 将输出分成两层：运行时指标本身，以及面向开发/运维的 dashboard、PromQL 入口和 operator checklist。

**Non-Goals:**

- 不引入按 domain、原始 upstream 列表、request ID 等高基数 labels。
- 不把 logging、profiling、metrics 一次性平台化重做。
- 不在 `v1.1.1` 中承诺 readiness/liveness/degraded 的完整运行韧性实现。
- 不以这次变更直接解决所有历史 backlog 问题，只为后续韧性与排障提供观测基础。

## Decisions

### 1. 用“问题域”组织指标，而不是继续只按通用 request/response 暴露

决策：

- 新增观测面以五个问题域组织：`cache`、`snapshot`、`upstream`、`XDP`、`state machine`。
- 每个问题域都要求至少有“总量/结果/失败原因”三类基础信号，避免只有总量没有解释维度。

原因：

- 现在的 request/response/latency 只能说明系统外部表现，无法说明内部哪一段导致退化。
- 按问题域组织更适合开发排障和运维排查，也更容易映射到文档与 dashboard。

备选方案：

- 继续只补几个零散 counter。
  放弃原因：会继续积累命名不一致、语义不清和文档难维护的问题。

### 2. 先建立低基数指标预算，再决定每个事件是否值得做成 metric

决策：

- 所有新指标默认只允许使用有限枚举标签，如 `result`、`rcode`、`reason`、`path`、`stage`。
- 原始域名、完整 upstream 列表、任意字符串错误消息不进入 labels。
- 单个 upstream IP 只在已有 IPQualityV2 或明确受控的 upstream 维度中出现，不在所有错误计数上泛化。

原因：

- rec53 是 node-local resolver，观测必须可长期运行，不能把指标系统做成新的资源风险。
- 对排障最有价值的是“分类”，不是把所有原始输入搬到 label 上。

备选方案：

- 允许更细粒度标签，再由运维自行约束。
  放弃原因：一旦进入发布版本，约束会失效，后续收口成本更高。

### 3. 指标、文档、dashboard 一起定义，但实现按批次推进

决策：

- 第一批先落地 `cache / snapshot / upstream` 基础计数与失败原因。
- 第二批补 `XDP` 深水区指标和 `state machine` 阶段聚合。
- 文档与 dashboard/checklist 以首批指标为基线同步产出，避免代码有指标但无人会读。

原因：

- cache、snapshot、upstream 是当前最直接影响“是否可用”的路径，优先级最高。
- XDP 和状态机聚合更偏深度诊断，适合在基础信号稳定后再补。

备选方案：

- 一次性把五类指标全部实现完再补文档。
  放弃原因：范围过大，且容易出现埋点完成但运维不可用的结果。

### 4. 把“给开发看”和“给运维/用户看”明确拆成两套入口

决策：

- `docs/metrics.md` 承担指标定义、标签约束、PromQL 示例和解释基线。
- 运维/用户入口放在 `docs/user/operations.md` 与 `docs/user/troubleshooting.md`，侧重“先看什么、出现什么现象意味着什么”。
- dashboard/checklist 的组织方式必须显式区分开发者诊断视角和运维健康视角。

原因：

- 开发需要的是定位层次和变化归因，运维需要的是健康判断和动作优先级，这两者不应混在同一套表述里。

备选方案：

- 只更新 `docs/metrics.md`，不区分受众。
  放弃原因：无法满足用户提出的目标拆解要求，也会让观测文档再次变成“大而混”的总表。

### 5. 保持日志与 pprof 为二级信号，指标负责“一眼看出哪里不对”

决策：

- Prometheus 指标负责暴露结果分类与趋势。
- 结构化日志继续承接细节上下文。
- pprof 继续只用于性能与资源深挖，不承担一线运行状态判断。

原因：

- 这符合当前代码结构，也避免把 `v1.1.1` 扩成 tracing/diagnostics 平台化项目。

备选方案：

- 用更细的日志或 profiling 替代新增指标。
  放弃原因：不利于持续运营，也不便于 dashboard/告警聚合。

## Risks / Trade-offs

- [指标过多但无人会读] → 与指标定义同时产出 PromQL 示例、dashboard 视图和 operator checklist。
- [标签失控造成高基数] → 在 spec 中显式禁止原始 domain 和自由文本 labels，并在实现阶段优先复用有限枚举。
- [代码埋点分散导致维护成本上升] → 由 `monitor/` 统一承接指标定义和 helper，业务代码只传结果与原因。
- [第一批指标不足以覆盖所有问题] → 接受分批推进，先回答最高频运行问题，再补深度诊断面。
- [文档按受众拆分后维护成本增加] → 用 `docs/metrics.md` 做指标事实源，其他文档只做入口和解释，不重复定义细节。

## Migration Plan

1. 先定义 spec 和 design，冻结指标边界与标签预算。
2. 第一批实现 `cache / snapshot / upstream` 指标与对应测试。
3. 同步更新 `docs/metrics.md`、`docs/user/operations.md`、`docs/user/troubleshooting.md`。
4. 第二批补 `XDP / state machine` 指标、dashboard 与 operator checklist。
5. 发布时按“新增观测能力”说明，不将其描述为协议或功能语义变更。

回滚策略：

- 新指标全部为附加能力；若某一批实现引入噪音或成本过高，可单独回退对应 metric family，而不影响主功能路径。

## Open Questions

- `state machine` 阶段计数是否只统计进入次数，还是需要同时区分成功返回与失败返回。
- XDP occupancy 应暴露绝对条目数、使用率百分比，还是两者都暴露。
- dashboard/checklist 最终放在仓库文档中维护，还是额外提供导出的 Grafana JSON 模板。
