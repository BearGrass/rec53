## Context

`rec53top` 目前已经有明确的 overview 焦点和 detail 页面，但 detail 仍然是一张固定长页。对于 `Cache`、`Upstream`、`XDP` 这类同时包含当前窗口 breakdown 和累计 totals 的面板，单页会把“先看什么、再看什么”重新压平。

当前版本线已经把 `panel drill-down` 排到 TUI 增强候选的第一优先级，因此这次实现需要满足两个约束：

1. 继续复用现有 overview -> detail 交互，不重新设计整套 TUI 页面栈。
2. 保持最小实现，只给最需要拆开的面板提供 drill-down，不把 history、排序、target 切换一起引入。

## Goals / Non-Goals

**Goals:**

- 为 `Cache`、`Upstream`、`XDP` 提供 detail 内部的可切换子视图。
- 保持一套统一的 detail drill-down 导航规则，避免每个面板各自定义按键。
- 在 footer/help 和 detail 标题中体现当前子视图，降低迷失感。
- 保持 overview 焦点导航、`Enter` 打开 detail、`0`/`Esc` 返回 overview 的既有语义不变。

**Non-Goals:**

- 不为 `Traffic`、`Snapshot`、`State Machine` 新增 drill-down 子视图。
- 不引入历史 sparkline、排序筛选或 target 切换。
- 不把 detail 改成多层树结构或独立全局页签系统。

## Decisions

### 1. 采用“detail 内子视图”而不是新页面栈

- overview 进入 detail 的方式保持不变。
- 对支持 drill-down 的面板，在 detail 内部维护一个当前子视图索引。
- 子视图作为同一 detail 页内的内容切换，不新增第三层 page stack。

原因：

- 能最大化复用现有 `detailPanel` 语义和页面切换逻辑。
- `Esc` / `0` 仍然只承担“返回 overview”，避免层级返回规则变复杂。

### 2. 用 `Tab` / `Shift-Tab` 和 `[` / `]` 作为 detail 子视图切换键

- overview 中 `Tab` / `Shift-Tab` 继续用于焦点移动。
- detail 中 `Tab` / `Shift-Tab` 改为切换当前面板的 drill-down 子视图。
- 同时提供 `[` / `]` 作为显式上一页 / 下一页快捷键。
- 左右箭头在 detail 中也可用于子视图切换。

原因：

- 这些键在 detail 中当前没有承担别的核心动作。
- 不占用 `h`、`q`、`r`、`0`、`Esc`、`1-6` 的既有兼容键位。

### 3. Drill-down 只覆盖三类最值得拆层的面板

- `Cache`：`Summary`、`Lookup Mix`、`Lifecycle Totals`
- `Upstream`：`Summary`、`Failure Reasons`、`Winner Paths`
- `XDP`：`Summary`、`Packet Paths`、`Sync And Cleanup`

原因：

- 这三类面板已经有可复用的 breakdown 和累计 totals，数据模型现成。
- 先做这三类能验证 drill-down 交互是否成立，再决定是否扩到其他面板。

## Risks / Trade-offs

- [风险] detail 中复用 `Tab` 可能和 overview 的焦点导航语义混淆
  → 通过 footer/help 和标题中的子视图标签明确“当前处于 detail drill-down”

- [风险] 子视图如果只是把原有长页拆开，价值不够明显
  → 保持 `Summary` 为诊断结论页，其他子视图只显示该主题相关 breakdown/totals

- [风险] 状态异常时 drill-down 可能显得空洞
  → 对 `WARMING`、`UNAVAILABLE`、`DISABLED`、`STALE`、`DISCONNECTED` 统一退化到 summary 解释页，不强行展示空 breakdown

## Migration Plan

1. 给 detail 状态增加每个支持面板的当前子视图索引。
2. 拆分 `Cache`、`Upstream`、`XDP` 的 detail 渲染为 summary 与主题子视图。
3. 更新输入捕获、标题和 footer/help，使 detail 中可发现子视图切换。
4. 更新文档和 roadmap，并补充 focused tests。
