## ADDED Requirements

### Requirement: pprof HTTP 端点可通过配置启用
rec53 SHALL 支持通过配置项 `debug.pprof_enabled` 启用 pprof HTTP 端点。默认值 SHALL 为 `false`（关闭）。

#### Scenario: 默认配置不启动 pprof
- **WHEN** `config.yaml` 中未配置 `debug` 节或 `debug.pprof_enabled` 为 `false`
- **THEN** rec53 SHALL 不启动 pprof HTTP server，不监听 pprof 端口

#### Scenario: 启用 pprof 后可访问 profile 端点
- **WHEN** `config.yaml` 中配置 `debug.pprof_enabled: true`
- **THEN** rec53 SHALL 启动 pprof HTTP server，`/debug/pprof/` 路径 SHALL 返回 pprof 索引页

---

### Requirement: pprof 端点默认仅监听本地地址
pprof HTTP server 的监听地址 SHALL 通过配置项 `debug.pprof_listen` 指定，默认值 SHALL 为 `"127.0.0.1:6060"`。

#### Scenario: 默认监听地址为 127.0.0.1:6060
- **WHEN** `debug.pprof_enabled: true` 且未配置 `debug.pprof_listen`
- **THEN** pprof server SHALL 监听 `127.0.0.1:6060`

#### Scenario: 自定义监听地址
- **WHEN** `debug.pprof_enabled: true` 且 `debug.pprof_listen: "127.0.0.1:9999"`
- **THEN** pprof server SHALL 监听 `127.0.0.1:9999`

---

### Requirement: pprof server 纳入服务生命周期管理
pprof HTTP server SHALL 纳入 rec53 的服务生命周期：接收主 context 取消信号后优雅关闭，不泄漏 goroutine。

#### Scenario: 服务关闭时 pprof server 优雅退出
- **WHEN** rec53 收到关闭信号（context 取消）
- **THEN** pprof HTTP server SHALL 调用 `Shutdown` 方法优雅退出，所有 pprof 相关 goroutine SHALL 在关闭完成后终止

#### Scenario: pprof server 启动失败不影响 DNS 服务
- **WHEN** pprof 端口被占用导致 `ListenAndServe` 失败
- **THEN** rec53 SHALL 记录错误日志，DNS 服务 SHALL 继续正常运行

---

### Requirement: README 文档包含 pprof 使用说明
`README.md` 和 `README.zh.md` SHALL 同步更新，包含 pprof 功能的配置示例和使用方式。

#### Scenario: 中英文 README 同步更新
- **WHEN** pprof 功能实现完成
- **THEN** `README.md` SHALL 包含英文 pprof 配置示例和 `go tool pprof` 使用命令
- **AND** `README.zh.md` SHALL 包含对应的中文版本
- **AND** 两份文档的信息 SHALL 保持一致
