## ADDED Requirements

### Requirement: XDP configuration struct
`cmd/rec53.go` SHALL 定义 `XDPConfig` 结构体，包含 `Enabled bool`（默认 false）和 `Interface string`（XDP attach 的网卡名称）。其余参数（map 大小、超时等）SHALL 使用硬编码常量。

#### Scenario: Default config — XDP disabled
- **WHEN** config.yaml 中不包含 `xdp:` 块
- **THEN** `XDPConfig.Enabled` SHALL 为 false
- **AND** XDP 相关代码 SHALL 不被初始化

#### Scenario: XDP enabled with interface
- **WHEN** config.yaml 包含 `xdp: { enabled: true, interface: "eth0" }`
- **THEN** `XDPConfig.Enabled` SHALL 为 true
- **AND** `XDPConfig.Interface` SHALL 为 `"eth0"`

#### Scenario: XDP enabled without interface
- **WHEN** config.yaml 包含 `xdp: { enabled: true }` 但无 `interface` 字段
- **THEN** 启动 SHALL 失败并报告错误：XDP 启用时 interface MUST 指定

### Requirement: Config file template update
`generate-config.sh` SHALL 在生成的 config.yaml 中包含 `xdp:` 配置块，默认 `enabled: false`，并附注释说明各字段含义和运行时要求。

#### Scenario: Generated config contains xdp block
- **WHEN** 执行 `./generate-config.sh`
- **THEN** 生成的 config.yaml SHALL 包含 `xdp:` 块
- **AND** `enabled` SHALL 为 `false`
- **AND** 注释 SHALL 说明需要 Linux kernel >= 5.15 和 CAP_BPF 权限
