## Why

`v1.1.2` 已经把 `rec53top` 做成了可用的本地运维 TUI，但它目前仍然更偏“运维首轮判断”而不是“开发排障定位”。detail 面板缺少累计计数器视角，用户文档也还没有一份更适合发布介绍与外部引用的单独说明。

现在需要把 `v1.1.3` 明确为 TUI 增强版本：先补足 detail 的开发诊断价值，再补一份可以被 README、release notes 和用户介绍直接引用的 TUI 文档，让这条版本线能按部就班推进。

## What Changes

- 为 `rec53top` detail 面板增加 bounded 的累计计数器视图，让用户同时看到短窗口信号和 since-start counters。
- 统一 detail 面板的语义层次，明确哪些内容属于当前判断、哪些属于累计路径热点、哪些属于 next checks。
- 增加一份面向发布和用户介绍的 TUI 文档，解释 `rec53top` 的定位、适用场景、阅读方式、边界和自测路径。
- 在 roadmap 上把 `v1.1` 明确收敛为“可观测性与本地运维”版本线，并将韧性和 DNSSEC 后移到 `v1.2.x`。

## Capabilities

### New Capabilities
- `rec53top-detail-counters`: 为 `rec53top` detail 视图增加累计计数器和更稳定的开发排障语义，帮助定位代码路径和长期热点。
- `rec53top-release-intro`: 提供一份面向用户/发布说明的 TUI 介绍文档，可被 README、release notes 和用户指南直接引用。

### Modified Capabilities

## Impact

- 主要影响代码：`tui/dashboard.go`、`tui/app.go` 及相关 detail 渲染/测试。
- 主要影响文档：TUI 用户文档、README 入口、roadmap，以及新增的发布介绍页。
- 不改变 rec53 主服务的解析路径、指标暴露端点或 `rec53ctl` 生命周期行为。
