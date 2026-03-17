## 1. 配置结构与解析

- [x] 1.1 在 `server/` 包中新增 `hosts_config.go`，定义 `HostEntry` 和 `HostsConfig` 类型
- [x] 1.2 在 `server/` 包中新增 `forward_config.go`，定义 `ForwardZone` 和 `ForwardingConfig` 类型
- [x] 1.3 在 `cmd/rec53.go` 的 `Config` struct 中添加 `Hosts []server.HostEntry` 和 `Forwarding []server.ForwardZone` YAML 字段
- [x] 1.4 在 `cmd/rec53.go` 的 `validateConfig` 中添加 hosts 条目校验（IP 格式、记录类型、非空 name/value）
- [x] 1.5 在 `cmd/rec53.go` 的 `validateConfig` 中添加 forwarding 条目校验（非空 zone、非空 upstreams、upstream 地址格式）
- [x] 1.6 更新 `generate-config.sh` 和 `config.yaml`，添加 `hosts` / `forwarding` 注释示例节

## 2. Server 注入与初始化

- [x] 2.1 扩展 `server.go` 中的 `server` struct，添加 `hostsConfig` 和 `forwardingConfig` 字段
- [x] 2.2 新增 `NewServerWithFullConfig(listen string, warmupCfg WarmupConfig, hosts []HostEntry, forwarding []ForwardZone) *server` 构造函数
- [x] 2.3 在 `server` 初始化时将 hosts 预编译为 `map[string]*dns.Msg`（key = `"fqdn. qtype"`）
- [x] 2.4 在 `server` 初始化时将 forwarding zones 按 zone 字符串长度降序排列
- [x] 2.5 在 `cmd/rec53.go` 的 `main()` 中改用 `NewServerWithFullConfig`，传入 `cfg.Hosts` 和 `cfg.Forwarding`

## 3. Hosts 状态实现

- [x] 3.1 在 `server/state.go`（或 `state_define.go`）中新增 `HOSTS_LOOKUP` 状态常量及返回码
- [x] 3.2 新增 `server/state_hosts.go`，实现 `hostsLookupState` 及其 `handle()` 方法
- [x] 3.3 `handle()` 对命中的条目构造 AA=true 的 DNS 响应，对类型不匹配的条目返回 NODATA（NOERROR + 空 answer）
- [x] 3.4 在 `state_machine.go` 的 `Change()` 中：`STATE_INIT` 成功后跳转到 `HOSTS_LOOKUP` 而非 `CACHE_LOOKUP`
- [x] 3.5 `HOSTS_LOOKUP` 命中时跳转到 `RETURN_RESP`；未命中时跳转到下一状态（`FORWARD_LOOKUP`）

## 4. Forwarding 状态实现

- [x] 4.1 在 `server/state.go` 中新增 `FORWARD_LOOKUP` 状态常量及返回码
- [x] 4.2 新增 `server/state_forward.go`，实现 `forwardLookupState` 及其 `handle()` 方法
- [x] 4.3 实现最长后缀匹配逻辑，从降序 zone 列表中找到第一个匹配的 `ForwardZone`
- [x] 4.4 向匹配的上游顺序发送标准转发查询（`rd=1`），复用 `UpstreamTimeout`；全部失败时返回 SERVFAIL 返回码
- [x] 4.5 转发结果**不**写入 `globalDnsCache`
- [x] 4.6 在 `state_machine.go` 的 `Change()` 中：`HOSTS_LOOKUP` 未命中时跳转到 `FORWARD_LOOKUP`；命中时直接构造响应，跳转到 `RETURN_RESP`；未命中时跳转到 `CACHE_LOOKUP`

## 5. 单元测试

- [x] 5.1 新增 `server/state_hosts_test.go`：测试 A/AAAA/CNAME 命中、类型不匹配 NODATA、未命中穿透
- [x] 5.2 新增 `server/state_forward_test.go`：测试最长后缀匹配、上游成功返回、全部上游失败 SERVFAIL
- [x] 5.3 在 `cmd/` 或 `server/` 中添加 `validateConfig` hosts/forwarding 校验的单元测试（无效 IP、不支持的类型、空上游）
- [x] 5.4 运行 `go test -race ./...` 确保无竞争条件

## 6. E2E 集成测试

- [x] 6.1 在 `e2e/` 中新增 `hosts_forward_test.go`，使用 `MockAuthorityServer` 验证：hosts 优先级 > forwarding > cache > iter
- [x] 6.2 新增场景：hosts A 记录命中，AA 标志正确
- [x] 6.3 新增场景：forwarding zone 匹配，响应来自 mock 上游
- [x] 6.4 新增场景：forwarding 上游全部不可达，客户端收到 SERVFAIL
- [x] 6.5 运行 `go test -race -timeout 120s ./e2e/...` 确保所有 E2E 测试通过

## 7. 文档更新

- [x] 7.1 更新 `README.md` 和 `README.zh.md`，添加 `hosts` / `forwarding` 配置说明和示例
- [x] 7.2 更新 `docs/architecture.md`，记录新状态节点和查询处理链变化
- [x] 7.3 更新 `CHANGELOG.md`，在对应版本节下记录新功能
