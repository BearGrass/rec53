## ADDED Requirements

### Requirement: README 面向快速上手，行数 ≤ 120 行
README.md SHALL 仅包含以下章节，总行数不超过 120 行：项目一句话描述（含语言切换链接）、Features 核心特性列表、Quick Start（Build / Run / Test resolution）、CLI Flags 表格、Configuration 基础示例、Docker 快速运行命令、文档索引（链接至 `docs/` 和 `.rec53/`）。

#### Scenario: 用户克隆后快速运行
- **WHEN** 用户打开 README.md
- **THEN** 在 120 行内能找到 build、run、test 的完整命令，无需滚动超过一屏

#### Scenario: 行数约束
- **WHEN** README.md 被提交
- **THEN** 文件总行数 SHALL 不超过 120 行

### Requirement: README 包含完整文档索引
README.md SHALL 在末尾包含文档索引，链接指向：`docs/architecture.md`、`docs/benchmarks.md`、`docs/metrics.md`、`.rec53/CONVENTIONS.md`、`.rec53/ROADMAP.md`。

#### Scenario: 文档索引链接可达
- **WHEN** README 中的文档索引链接被访问
- **THEN** 每个链接指向的文件 SHALL 存在于仓库中

#### Scenario: 不再链接 .rec53/ARCHITECTURE.md
- **WHEN** README.md 精简完成后
- **THEN** README.md 中 SHALL NOT 出现 `.rec53/ARCHITECTURE.md` 的链接（已迁移至 `docs/architecture.md`）

### Requirement: README 不包含系统设计细节
README.md SHALL NOT 包含状态机转换图、缓存 API 说明、IP Pool 算法伪代码、Prometheus 指标定义、PromQL 查询示例、性能基准数据表格。

#### Scenario: 移除系统设计章节
- **WHEN** README.md 被精简后
- **THEN** "System Design"、"Core Subsystem"、"Specifications"（性能数据）等章节 SHALL 不出现在 README.md 中
