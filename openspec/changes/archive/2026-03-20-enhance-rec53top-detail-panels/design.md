## Context

`rec53top` 当前已经有 overview 和 detail 两层视图，但 detail 的实现仍然偏 MVP：大部分面板只是把 overview 中已经出现的几个数字重新排版，再附上固定的 `Reading guide`。这对第一次看界面的用户可能有一点帮助，但对已经进入 detail 的用户来说，新增信息密度仍然不足。

这次增强的目标不是把 TUI 做成完整 observability 平台，而是让 detail 真正承担“诊断扩展层”的角色：进入 detail 后，用户应当先看到当前最值得关心的异常、主导 breakdown、状态解释和下一步排查方向，而不是先读静态说明段落。

约束如下：

- 不引入历史存储、sparklines 或多 target 聚合。
- 不改变 rec53 主服务暴露的 metrics 名称、标签和端点。
- 保持当前 overview 六面板布局与快捷键映射不变。
- 保持 `rec53top` 仍是单 target、只读、本地快速排查工具。

## Goals / Non-Goals

**Goals:**

- 让每个 detail 面板都提供区别于 overview 的新增诊断价值。
- 在 detail 层明确展示当前主导异常、主要 breakdown 和排查方向。
- 统一 detail 面板的结构，使不同面板具有稳定阅读顺序。
- 在 `WARMING`、`UNAVAILABLE`、`DISABLED`、`DISCONNECTED`、`STALE` 等状态下给出解释性而非空泛的内容。
- 为 detail 的新增逻辑补上可测试的 view-model 和 render 断言。

**Non-Goals:**

- 不引入历史趋势线、交互式 drill-down、面板排序或筛选。
- 不把 detail 改成多页导航流程或复杂键位系统。
- 不为了 detail 重构现有 metrics 采集链路或新增后台缓存。
- 不改变 summary 面板的总体布局和信息密度目标。

## Decisions

### 1. 为 detail 增加“诊断摘要层”，而不是继续堆更多静态说明

决策：

- 每个 detail 面板新增一个统一的诊断摘要区域，优先输出：
  - 当前状态结论
  - 当前最突出的风险或主导信号
  - 推荐继续查看的关联面板或日志方向
- 现有 `Reading guide` 不直接删除，但收敛为更短的“Next checks”或“Interpretation”内容。

原因：

- 当前问题不是 detail 太短，而是 detail 没有帮用户完成判断。
- 进入 detail 的时刻，用户想看的是“这次哪里异常”，不是通用教材。

备选方案：

- 继续追加更多固定说明文字。
  放弃原因：会增加字数，但不会提升当前场景下的判断效率。

### 2. 在 view-model 层显式建模“主导项”和“下一步建议”

决策：

- 在 `tui/dashboard.go` 中为各 detail 面板补充派生字段，例如：
  - 主导异常标签 / reason
  - Top breakdown 文本或解释摘要
  - 当前状态下的 next-step 建议
- 这些字段仍然完全由现有 metrics 派生，不额外依赖历史存储。

原因：

- 如果把所有判断逻辑写死在 `renderDetail` 文本拼接里，测试会很脆弱，后续增强也会很难维护。
- 当前已有 per-panel status 和 breakdown，继续在 view-model 层派生“主导因素”是自然扩展。

备选方案：

- 仅在 `renderDetail` 中基于已有字段做字符串拼接判断。
  放弃原因：会让渲染和诊断逻辑缠在一起，难以写稳定测试。

### 3. 统一 detail 面板阅读顺序，避免每块 detail 的表达风格分裂

决策：

- detail 面板尽量统一成以下结构：
  - `status`
  - `what stands out now`
  - `key metrics`
  - `top breakdowns`
  - `next checks`
- 对没有 breakdown 的面板，仍保留相同骨架，但让 “what stands out now” 承担更多解释。

原因：

- 当前不同 detail 面板的内容风格不一致，用户需要重新学习每块 detail 的阅读方式。
- 统一结构后，用户切换 detail 时能更快定位关键信息。

备选方案：

- 每个面板自由设计文本结构。
  放弃原因：局部可能更灵活，但整体可读性会下降。

### 4. 状态特异化文案优先于“no recent samples”式中性文案

决策：

- 对 `WARMING`、`UNAVAILABLE`、`DISABLED`、`DISCONNECTED`、`STALE` 状态使用专门解释文本。
- `detailBreakdownLines` 仍可保留，但 detail 顶部要先用状态特异化摘要解释“为什么现在没有值得看的 breakdown”。

原因：

- 用户对 detail 不满意的另一层原因是：在异常或无数据状态下，detail 看起来仍像普通空白说明页。
- 把状态语义前置，可以让 detail 即使在“无数据”场景下也仍然有信息价值。

备选方案：

- 仅复用现有状态行和占位 breakdown。
  放弃原因：无法解决“进入 detail 仍然没东西看”的核心问题。

### 5. 先做文本诊断增强，不在这次变更中追加更多交互

决策：

- 本次只增强 detail 的派生信息与排版结构。
- 快捷键仍保持 `1-6`、`0`、`h`、`r`、`q`，不新增多层面板内导航。

原因：

- 当前用户反馈聚焦在“detail 没内容”，不是“无法操作”。
- 保持交互面稳定，可以让变更集中在信息价值本身。

备选方案：

- 同时引入 detail 子页、分页或滚动区域。
  放弃原因：范围扩大，但不保证先解决信息质量问题。

## Risks / Trade-offs

- [detail 派生判断过强，误导用户] → 使用保守规则，只基于当前已有 status 和 breakdown 做解释，不编造额外因果链。
- [新增字段让 view-model 膨胀] → 只加入 detail 直接需要的派生字段，不做通用规则引擎。
- [detail 文本变长后小终端可读性下降] → 固定优先级，先展示结论和 top 项，压缩静态说明段。
- [不同面板的诊断粒度不一致] → 允许面板间细节多少不同，但统一外层结构和语气。

## Migration Plan

1. 在 `Dashboard` / per-panel view-model 中补充 detail 所需的派生字段或 helper。
2. 重写 `renderDetail` 的结构，使其优先输出诊断摘要和 next-step 引导。
3. 为主要状态组合补充 render 测试和 view-model 测试。
4. 更新 TUI 用户文档，说明 detail 视图能回答的问题和推荐阅读顺序。

回滚策略：

- 如果 detail 增强效果不稳定，可以仅回退新增的派生文案与 render 结构，不影响 overview、scrape 或主服务行为。

## Open Questions

- 是否要把 “Next checks” 明确写成面板编号引用，例如 “see 4 Upstream” / “see 6 State Machine”。
- 是否要在 detail 中增加最近 scrape 错误或 last-success 时间的局部回显，而不仅依赖顶部 header。
