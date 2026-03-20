## ADDED Requirements

### Requirement: docs/architecture.md 完整合并两处来源内容
`docs/architecture.md` SHALL 包含原 `.rec53/ARCHITECTURE.md` 的全部内容，并补充 README.md 中的以下章节：目录结构（Directory Structure）、请求生命周期（Request Lifecycle）、组件映射表（Component Map）、状态机完整说明（States 表格、转换图、Loop A/B 注释、CNAME 处理、NS 无 Glue 解析、Return Codes 表格）、缓存子系统（设计、负向缓存、Cache API、线程安全）、IP Pool 子系统（IPQualityV2 数据结构、生命周期、评分公式、Score 示例表、选择 API、并发访问、Warmup Bootstrap）。合并时 SHALL 去除重复段落，不重写原有内容。

#### Scenario: 内容无丢失
- **WHEN** `docs/architecture.md` 被创建后
- **THEN** 原 `.rec53/ARCHITECTURE.md` 和 README.md 系统设计章节中的每个二级/三级标题 SHALL 在新文件中有对应章节

#### Scenario: 原文件删除
- **WHEN** `docs/architecture.md` 内容验证完成
- **THEN** `.rec53/ARCHITECTURE.md` SHALL 被删除，避免内容重复

### Requirement: docs/architecture.md 文件路径使用小写
`docs/architecture.md` SHALL 使用全小写文件名，与 `docs/testing/benchmarks.md`、`docs/metrics.md` 命名风格一致。

#### Scenario: 文件名格式
- **WHEN** 在 `docs/` 目录下创建文件
- **THEN** 文件名 SHALL 全部小写，单词间用连字符分隔
