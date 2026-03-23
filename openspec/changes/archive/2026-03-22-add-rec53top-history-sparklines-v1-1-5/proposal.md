## Why

`rec53top` 现在更适合做“当下状态”的快速诊断，而不是承担完整历史监控。既然已经有 Prometheus 提供长期趋势和时间窗口对比，那么 TUI 如果还要补一点“趋势感”，就应该只提供非常轻的进程内短历史，用来判断当前信号是在继续变坏还是已经回落。

## What Changes

- 为 `rec53top` 增加轻量趋势提示，只保留最近 `10-20` 个 scrape 的内存序列，不落盘、不导出、不接外部时序库。
- 趋势提示只服务于当前窗口阅读，帮助判断某个指标是在上升、回落还是横盘，而不是替代 Prometheus 的历史监控职责。
- 更新 TUI 文档和 roadmap，明确 `rec53top` 与 Prometheus/Grafana 的职责边界。

## Capabilities

### New Capabilities
- `rec53top-lightweight-trends`: 为 `rec53top` 提供进程内短历史趋势提示，辅助当下状态判断。

### Modified Capabilities
- `local-ops-tui`: TUI 文档需要说明轻量趋势提示的边界和使用方式。

## Impact

- 主要影响代码：`tui/app.go`、`tui/dashboard.go`、可能新增历史序列辅助结构
- 主要影响文档：`docs/user/local-ops-tui.md`、`docs/user/rec53top.md`、`docs/roadmap.md`
- 不改变 metrics 抓取源，不引入持久化历史，也不替代 Prometheus/Grafana
