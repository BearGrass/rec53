## MODIFIED Requirements

### Requirement: docs/architecture.md 服务开发者维护
`docs/architecture.md` SHALL be a developer-facing architecture document that explains request flow, state machine behavior, component responsibilities, cache constraints, and concurrency boundaries without duplicating end-user deployment instructions.

#### Scenario: 开发者查阅架构实现
- **WHEN** 开发者阅读 `docs/architecture.md`
- **THEN** 文档 SHALL 聚焦于系统如何工作、模块如何协作以及修改时需要注意的边界

#### Scenario: 使用者寻找部署方法
- **WHEN** 使用者需要安装、运行或排障说明
- **THEN** 相关内容 SHALL 被引导至用户文档，而不是继续堆叠在架构文档中

### Requirement: docs/architecture.md 与当前代码结构保持一致
`docs/architecture.md` SHALL describe the current repository layout and major modules accurately enough for maintainers to locate the implementation described in the document.

#### Scenario: 目录说明与代码匹配
- **WHEN** 开发者根据架构文档查找 `cmd/`、`server/`、`monitor/`、`utils/` 或 `e2e/` 中的实现
- **THEN** 文档中的文件和职责描述 SHALL 与当前仓库结构保持一致，不包含明显过时的路径说明
