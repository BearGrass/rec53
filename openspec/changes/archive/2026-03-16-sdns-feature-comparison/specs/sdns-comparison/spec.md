## ADDED Requirements

### Requirement: sdns 功能对比文档存在于 docs/ 目录
项目 SHALL 在 `docs/sdns-comparison.md` 中维护一份与 sdns 项目（github.com/semihalev/sdns）的逐维度功能对比文档。

#### Scenario: 文档文件存在
- **WHEN** 查看项目 `docs/` 目录
- **THEN** 应存在 `docs/sdns-comparison.md` 文件

### Requirement: 对比文档包含元数据头部
文档 SHALL 在顶部注明对比时所参照的 sdns 版本号和对比日期，以便读者判断数据时效性。

#### Scenario: 文档头部有版本和日期信息
- **WHEN** 打开 `docs/sdns-comparison.md`
- **THEN** 文档顶部 SHALL 包含 sdns 版本号（如 v1.6.1）和对比日期

### Requirement: 对比文档覆盖核心功能维度
文档 SHALL 按以下维度对 rec53 与 sdns 进行逐项对比：DNS 解析能力、传输协议支持、缓存系统、安全功能、访问控制与速率限制、监控与可观测性、Kubernetes 集成、扩展架构。

#### Scenario: 文档包含所有对比维度
- **WHEN** 查看文档内容
- **THEN** 文档 SHALL 包含上述 8 个维度的对比内容，每个维度以表格或列表形式呈现

### Requirement: 每个差距项标注建议优先级
对于 sdns 已有而 rec53 尚未实现的功能，文档 SHALL 为每项差距标注建议优先级（High / Medium / Low / Out-of-scope），并简要说明判断依据。

#### Scenario: 差距项包含优先级标注
- **WHEN** 查看 rec53 与 sdns 的差距项
- **THEN** 每个差距项 SHALL 标注优先级，且优先级值为 High、Medium、Low、Out-of-scope 之一

### Requirement: 文档在 README 中被引用
`README.md` 和 `README.zh.md` SHALL 在文档索引中包含指向 `docs/sdns-comparison.md` 的链接，使用户能够发现该对比文档。

#### Scenario: README 中存在对比文档链接
- **WHEN** 查看 README.md 的文档索引章节
- **THEN** 应包含指向 `docs/sdns-comparison.md` 的相对路径链接
