## MODIFIED Requirements

### Requirement: README 面向快速上手与文档分流
README.md and README.zh.md SHALL serve as concise entry documents that present project positioning, quick start, minimum configuration, and links to user and developer documentation. They SHALL NOT remain the primary location for detailed architecture or operations content.

#### Scenario: 用户从 README 进入部署文档
- **WHEN** 用户打开 README
- **THEN** 其 SHALL 能在首页快速看到项目定位、默认部署入口以及通往 `docs/user/` 的文档链接

#### Scenario: 开发者从 README 进入开发文档
- **WHEN** 开发者打开 README
- **THEN** 其 SHALL 能在首页快速看到通往 `docs/dev/` 和架构文档的入口，而无需在 README 中翻找实现细节

### Requirement: README 包含完整文档索引
README.md SHALL 在首页提供清晰的文档索引，至少区分“用户文档”和“开发者文档”两类入口，并确保链接可达。

#### Scenario: 文档入口可达
- **WHEN** README 中的用户文档或开发者文档链接被访问
- **THEN** 每个链接 SHALL 指向仓库中存在的文档文件

### Requirement: README 不包含系统设计细节
README.md SHALL NOT contain detailed state machine diagrams, cache implementation details, IP pool internals, metric definitions, or benchmark tables. Such material SHALL live in dedicated developer or benchmark documents.

#### Scenario: 系统设计内容已下沉
- **WHEN** README 精简完成
- **THEN** 详细实现说明 SHALL 被迁移到专门文档，README 仅保留高层介绍和导航
