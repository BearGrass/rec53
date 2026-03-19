## ADDED Requirements

### Requirement: 用户文档与开发者文档分离
The project SHALL provide a dedicated user documentation space under `docs/user/` for deployment and operations guidance, separate from developer-facing architecture and contribution docs.

#### Scenario: 使用者寻找部署文档
- **WHEN** 使用者从 README 进入详细文档
- **THEN** 其 SHALL 能在 `docs/user/` 中找到与部署、配置、运维和排障相关的文档，而不需要阅读开发者实现细节

#### Scenario: 开发者文档不再承担用户部署说明
- **WHEN** 使用者或开发者浏览开发文档
- **THEN** 开发文档 SHALL 以实现与维护为主，不重复完整的用户安装与运行步骤

### Requirement: 用户文档覆盖默认部署路径
The user documentation SHALL cover the default deployable path for rec53, including first-run setup, minimum configuration, foreground validation, and service deployment.

#### Scenario: 首次部署路径完整
- **WHEN** 新用户按用户文档操作
- **THEN** 其 SHALL 能完成生成配置、启动服务、验证解析结果和查看日志的最小闭环

### Requirement: 用户文档覆盖运维与排障
The user documentation SHALL document operational topics that affect first deployment and day-2 usage, including metrics, logs, optional pprof, and common startup or resolution failures.

#### Scenario: 用户排查启动失败
- **WHEN** 服务因为配置错误、端口冲突或权限问题无法启动
- **THEN** 用户文档 SHALL 提供对应的排查入口和推荐检查项
