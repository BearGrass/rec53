## Context

`rec53top` 在 `v1.1.2` 已经完成了本地运维 TUI MVP：单 target、固定六面板、短窗口 rate/ratio、detail 诊断摘要和 `-plain` fallback 都已具备。当前版本已经足够做第一轮 triage，但仍然有两个明显缺口：

1. detail 更偏当前状态解释，仍然缺少“从启动以来哪些计数器持续增长”的开发排障视角。
2. 现有 TUI 文档更偏使用说明，还缺少一份适合发布说明、README 引用和对外介绍的聚焦文档。

roadmap 现在已经把 `v1.1` 明确为“可观测性与本地运维”版本线，因此 `v1.1.3` 需要优先补这两个缺口，而不是马上切到运行韧性或 DNSSEC。

## Goals / Non-Goals

**Goals:**

- 在 detail 中同时提供短窗口判断和 bounded since-start counter 视图。
- 让开发者能够从 detail 快速判断“当前异常”和“长期累计热点”是否一致。
- 为各 detail 面板建立更稳定的语义骨架，避免每个面板随增强继续分裂。
- 增加一份面向发布/用户介绍的 TUI 文档，可直接被 README 或 release notes 引用。

**Non-Goals:**

- 不引入历史时序存储或任意时间窗口查询。
- 不在 `v1.1.3` 里把 TUI 扩成多 target 或可配置 dashboard。
- 不把发布介绍文档做成新的独立产品站或 Web 页面。
- 不在这一版解决全部 UX polish 项。

## Decisions

### 1. detail 采用“双视角”而不是用累计计数器替换短窗口视图

决策：

- detail 顶部继续保留当前的 standout / next-check 判断。
- 在其下新增 bounded 的累计计数器区块，展示 since-start totals 或 top labels 的累计值。
- 短窗口判断仍然是第一层，累计计数器作为第二层补充，而不是替代。

原因：

- 只看累计计数器会掩盖“现在是否仍在出问题”。
- 只看短窗口比率又不够适合开发定位“哪条代码路径长期在增长”。
- 两者并排最符合运维和开发共用一个 TUI 的目标。

备选方案：

- 直接把 detail 改成纯累计计数器页。
  放弃原因：会削弱当前 detail 已经建立起来的实时诊断价值。

### 2. 累计计数器只展示 bounded top-N，不做全量标签枚举

决策：

- 对 response code、cache result、upstream failure reason、state-machine failure/stage 等标签类 counter，只显示 top-N 累计项。
- 对总量类指标，显示稳定的累计字段，不做无限展开。

原因：

- TUI 的终端空间是稀缺的，全量 label 展开会立即失控。
- 当前目标是“帮助定位热点”，不是“复制 Prometheus 全部原始输出”。

备选方案：

- 在 detail 里列出所有累计 label。
  放弃原因：界面噪声过高，且随数据增长可读性会明显下降。

### 3. 发布介绍文档单独成页，而不是继续向使用说明文档堆内容

决策：

- 新增一份面向发布和用户介绍的文档，聚焦：
  - `rec53top` 是什么
  - 适合什么场景
  - overview/detail 分别能回答什么问题
  - 边界和已知不做项
- `docs/user/local-ops-tui.md` 继续保留为操作说明与自测文档。

原因：

- 发布介绍和操作说明服务的是不同阅读场景。
- 如果继续把所有内容都堆在 `local-ops-tui.md`，README 和 release notes 很难有一个短而稳定的引用目标。

备选方案：

- 继续只维护一份用户文档。
  放弃原因：会让“怎么用”和“为什么值得用”混在一起，复用性差。

### 4. 先稳住信息结构，再做 UX polish

决策：

- 本次设计优先固化信息结构和文档定位。
- UX polish 只做必要配合，不追求在这一版内彻底打磨完文案密度、布局和键位体验。

原因：

- 当前最大不确定性仍在“detail 最终该展示哪些信息”，不是界面装饰。
- 在语义没定前做大量 polish 容易返工。

## Risks / Trade-offs

- [累计计数器和短窗口信息混在一起会增加认知负担] → 用明确标题区分 “current window” 和 “since start”。
- [bounded top-N 可能漏掉次要但关键的问题] → 保留日志和 raw metrics 作为深挖路径，不让 TUI 伪装成完整调试台。
- [新文档和现有文档职责再次重叠] → 明确新文档是“介绍/发布页”，现有文档是“操作/自测页”。
- [版本范围继续膨胀] → 限制本次只做累计计数器、文档和信息结构，不并入 drill-down 或 history。

## Migration Plan

1. 为 detail 视图补充累计计数器所需的 view-model 字段和 bounded render helper。
2. 在各面板 detail 中加入 since-start 区块，并补测试覆盖。
3. 增加发布介绍文档，并在 README / roadmap / 相关用户文档中接好入口。
4. 将 UX polish 保持在后续候选项，避免当前版本继续扩版。

回滚策略：

- 如果累计计数器视图的可读性不达标，可以只回退新增的 since-start 区块，保留当前 detail 诊断层不变。

## Open Questions

- 是否需要在 full-screen 和 `-plain` 之间共享一部分累计计数器语义，还是先只增强 full-screen detail。
- 发布介绍文档是否需要配套一张固定截图/ASCII 示意，还是先以纯文本介绍为主。
