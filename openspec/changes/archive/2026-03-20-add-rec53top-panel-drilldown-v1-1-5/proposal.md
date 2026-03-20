## Why

`rec53top` 已经具备 overview 和 detail 两层信息，但 `Cache`、`Upstream`、`XDP` 这类面板仍然把多种诊断信号混在同一页里。用户已经能进入 detail，但还不能沿着一个面板继续向下拆开看“当前 breakdown”和“累计 totals”，这让 TUI 的阅读深度还停在半成品。

## What Changes

- 为 `rec53top` 的 `Cache`、`Upstream`、`XDP` detail 增加轻量 drill-down 子视图，让用户可以在同一面板内继续切到更细的 breakdown 页面。
- 为 detail 增加统一的子视图导航、当前子视图标识和返回语义，但不引入新的全局页面层级。
- 更新 TUI 文档和 roadmap，把 `v1.1.5` 明确收敛到 drill-down 和 UX polish 的当前阶段。

## Capabilities

### New Capabilities
- `rec53top-panel-drilldown`: 为 `Cache`、`Upstream`、`XDP` detail 提供可切换的子视图和更细的诊断层次。

### Modified Capabilities
- `rec53top-navigation-ux`: detail 页面需要支持子视图切换和更明确的当前导航状态。
- `local-ops-tui`: TUI 使用文档需要说明 detail drill-down 的默认使用路径和键位。

## Impact

- 主要影响代码：`tui/app.go`、`tui/dashboard.go`、`tui/*_test.go`
- 主要影响文档：`docs/user/local-ops-tui.md`、`docs/user/rec53top.md`、`docs/roadmap.md`
- 不改变 metrics 采集方式、不引入历史存储、不扩展为多 target TUI
