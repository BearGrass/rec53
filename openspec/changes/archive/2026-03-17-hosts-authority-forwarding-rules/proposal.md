## Why

rec53 目前仅支持纯递归解析模式：所有查询都走迭代解析流程，无法为内部域名返回自定义答案，也无法将特定域名的查询转发到指定上游。当用户在内网环境中部署 rec53 时，需要能够回答内部主机名（如 `db.internal`）并将企业域名的查询转发到内部 DNS 服务器，而不走公网迭代解析。

## What Changes

- **新增 `hosts` 本地权威功能**：在 `config.yaml` 中声明静态 A/AAAA/CNAME 映射，rec53 直接从本地配置返回权威答案，绕过缓存和迭代。
- **新增 `forwarding` 转发规则功能**：为特定域名后缀配置一组上游 DNS 服务器，匹配的查询直接转发（类似 `stub-zone`），不走递归解析。
- 两个功能均在 `STATE_INIT` 之后、`CACHE_LOOKUP` 之前插入新状态，使优先级链为：**hosts → forwarding → cache → iterative**。
- **BREAKING**：`config.yaml` 新增顶层 `hosts` 和 `forwarding` 节；无默认值，省略时行为不变。

## Capabilities

### New Capabilities

- `hosts-authority`: 在配置文件中声明静态 DNS 记录（A、AAAA、CNAME），对匹配查询直接返回权威答案，不走缓存或迭代。
- `forwarding-rules`: 为一组域名后缀配置专属上游 DNS 服务器列表，匹配查询以标准 UDP/TCP 转发（非迭代），结果不写入全局缓存。

### Modified Capabilities

（无现有 spec 需要修改）

## Impact

- **`cmd/rec53.go`**：`Config` struct 新增 `Hosts []HostEntry` 和 `Forwarding []ForwardZone` 字段；`loadConfig` 传递给 server。
- **`server/` 包**：新增 `state_hosts.go`、`state_forward.go`；`state_machine.go` 更新状态跳转表；`server.go` 接收并注入配置。
- **`config.yaml` / `generate-config.sh`**：新增注释示例节。
- **测试**：新增 `server/state_hosts_test.go`、`server/state_forward_test.go`，`e2e/` 新增集成场景。
- **无外部新依赖**（使用 `github.com/miekg/dns` 已有转发原语）。
