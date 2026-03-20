## Why

`rec53top` 现在已经有 detail 视图，但 detail 面板的大部分内容仍然只是 summary 指标的重复展开，再加几条静态 `Reading guide`。当用户按下 `1-6` 进入 detail 时，常见感受是“没有新增判断价值”，这会削弱 detail 交互本身的存在意义。

现在需要把 detail 从“更长的说明文本”提升为“当前这次 scrape 值得看的诊断层”，让用户在进入 detail 后能更快回答：哪一类异常最突出、最近变化集中在哪一组 breakdown、下一步更该去看哪个面板或日志。

## What Changes

- 为 `rec53top` detail 视图增加更强的诊断表达，而不是只重复 summary 行。
- 让各 detail 面板优先展示“当前异常点 / 主导因素 / 最近最值得关注的 breakdown”，减少静态说明文字占比。
- 为 detail 面板补充更明确的排查引导，例如“当前退化由 timeout 主导”“当前 cache miss 高于 hit”“当前 state-machine 失败主要集中在某类 reason”。
- 统一 detail 面板的结构，使用户进入任一 detail 后都能看到一致的层次：状态结论、关键指标、主要 breakdown、解释性提示、下一步建议。
- 补充针对 detail 视图的测试和用户文档，确保增强后的内容可回归、可理解。

## Capabilities

### New Capabilities
- `rec53top-detail-panels`: 定义 `rec53top` detail 视图必须提供的诊断信息、异常聚焦和排查引导，而不是仅重复 summary 面板内容。

### Modified Capabilities

## Impact

- 主要影响代码：`tui/dashboard.go` 的 detail 所需 view-model 信息、`tui/app.go` 的 detail render 逻辑，以及相关测试。
- 主要影响体验：`rec53top` 的 `1-6` detail 交互会从“说明型扩展”变成“诊断型扩展”。
- 主要影响文档：本地运维 TUI 的使用说明、detail 阅读方式和排查路径需要同步更新。
- 不改变 `rec53` 主服务、metrics 暴露路径或 `rec53ctl` 生命周期行为。
