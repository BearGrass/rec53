## Context

`rec53top` 已经有当前窗口 rate/ratio、since-start counters、detail drill-down 和 next checks。继续增强时，最容易失控的方向就是把 TUI 逐步做成第二套历史监控 UI，而这和现有 Prometheus/Grafana 能力重叠很大。

因此如果要做趋势相关能力，必须明确它只是“轻量趋势提示”：

1. 数据仅存在于 `rec53top` 进程内。
2. 只保留最近 `10-20` 个 scrape。
3. 只辅助回答“当前是否还在继续恶化/是否开始回落”，不承担历史查询、时间范围对比或长期诊断。

## Goals / Non-Goals

**Goals:**

- 给 TUI 增加极轻量的短历史趋势提示。
- 让趋势提示直接服务于当前 detail / drill-down 阅读，而不是变成独立历史页面。
- 明确与 Prometheus/Grafana 的职责分工。

**Non-Goals:**

- 不做持久化历史。
- 不做多分钟/多小时时间窗口查询。
- 不做复杂图表、导出、排序筛选联动或独立 trend 页面。

## Decisions

### 1. 趋势数据保存在 `rec53top` 进程内 ring buffer

- 每次 scrape 后把有限数量的已派生指标点写入 ring buffer。
- 只保留最近 `10-20` 个点。
- 进程退出即丢弃。

原因：

- 这是与 Prometheus 历史监控最清晰的边界。
- 实现成本低，也不会引入新的外部依赖。

### 2. 趋势提示优先出现在 detail / drill-down 中

- overview 继续以当前状态和摘要为主。
- detail 或 drill-down 中为少数核心指标附加微型趋势提示。

原因：

- overview 要求一眼可读，不适合塞太多微图。
- detail 本来就承担“判断下一步”的角色，趋势提示放这里更有解释力。

## Risks / Trade-offs

- [风险] 趋势提示做重了，会和 Prometheus 的职责重叠
  → 严格限制点数、作用范围和展示目标

- [风险] 点数过少可能误导
  → 只把它作为辅助判断，不覆盖当前窗口 rate/ratio 的主语义

- [风险] 每个面板都加会让界面噪音上升
  → 只给最值得观察的少数指标加趋势提示

## Migration Plan

1. 定义短历史 ring buffer 和要采样的派生指标。
2. 选择最值得显示趋势的 detail 指标。
3. 在 detail 中渲染极轻量趋势提示并补充文档。
