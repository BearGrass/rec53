## Why

`rec53top` 的信息层已经比 `v1.1.2` 明显更完整，但交互仍偏“知道热键的人才能顺手用”。overview 没有明确焦点，detail 进入方式仍然主要依赖记忆数字键，这让 TUI 还没有到“完成开发”的状态。

## What Changes

- 为 `rec53top` 增加稳定的 overview 焦点导航，让用户能用方向键、`j/k/l`、`Tab`、`Enter` 在面板间移动和进入 detail，同时保留 `h` 作为 help 兼容键。
- 在 overview 和 detail 中补足更清晰的焦点提示、键位提示和返回路径，减少对数字键记忆的依赖。
- 收敛 `v1.1` roadmap：明确 `v1.1.4` 继续承担 TUI 导航与 UX 完成度，而不是现在切去 `v1.2.0`。
- 更新 TUI 使用文档，让新的导航方式和交互语义成为默认说明路径。

## Capabilities

### New Capabilities
- `rec53top-navigation-ux`: 为 `rec53top` 提供 overview 焦点导航、进入 detail 的统一操作，以及更明确的交互提示。

### Modified Capabilities
- `local-ops-tui`: 更新本地运维 TUI 的交互说明和默认使用路径。

## Impact

- 主要影响代码：`tui/app.go`、可能补充 `tui/*_test.go`
- 主要影响文档：`docs/roadmap.md`、`docs/user/local-ops-tui.md`、README 中的相关入口说明
- 不改变 rec53 主解析逻辑、指标采集模型或 `rec53top` 的六面板信息架构
