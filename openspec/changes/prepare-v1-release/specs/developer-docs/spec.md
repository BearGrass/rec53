## ADDED Requirements

### Requirement: 开发者文档集中在 `docs/dev/`
The project SHALL provide a dedicated developer documentation space under `docs/dev/` for architecture, development workflow, testing guidance, code conventions, and release process.

#### Scenario: 维护者寻找开发入口
- **WHEN** 维护者从 README 或文档索引进入开发文档
- **THEN** 其 SHALL 能在 `docs/dev/` 中找到开发、测试、发布与代码维护相关入口

### Requirement: 开发者文档描述代码维护边界
The developer documentation SHALL document the project’s key implementation boundaries, including state machine flow, cache and IP pool constraints, concurrency expectations, and testing practices.

#### Scenario: 新贡献者准备修改代码
- **WHEN** 新贡献者准备修改 `server/` 核心逻辑
- **THEN** 开发者文档 SHALL 告知其相关模块职责、测试命令和需要遵守的并发/缓存约束

### Requirement: 开发者文档包含发布流程
The developer documentation SHALL include a release checklist for preparing a deployable version, including tests, README sync, changelog updates, and documentation consistency checks.

#### Scenario: 准备发布 v1.0.0
- **WHEN** 维护者执行发布前检查
- **THEN** 开发者文档 SHALL 提供可执行的发布检查项，而不是分散在多个零碎文档中
