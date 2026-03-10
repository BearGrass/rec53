# E2E Integration Tests

端到端集成测试，验证 rec53 DNS 解析器的完整系统行为。

## 运行测试

```bash
# 运行所有 E2E 测试
go test -v ./e2e/...

# 运行简短模式（跳过集成测试）
go test -v -short ./e2e/...

# 运行特定测试
go test -v ./e2e/... -run TestCacheBehavior

# 运行带超时的测试
go test -v ./e2e/... -timeout 5m
```

## 测试文件说明

### cache_test.go

缓存功能测试套件。

| 测试名称 | 说明 | 验证点 |
|---------|------|--------|
| `TestCacheBehavior` | 基本缓存行为测试 | 首次查询与缓存命中后的响应一致性 |
| `TestCacheConcurrentAccess` | 并发缓存访问测试 | 10 个并发 worker，每个 20 次查询，验证缓存线程安全性 |
| `TestCacheDifferentTypes` | 不同记录类型缓存测试 | A、AAAA、MX、TXT 记录的缓存行为 |
| `TestCacheHitRate` | 缓存命中率测试 | 多域名二次查询性能提升验证 |

---

### error_test.go

错误处理与边界条件测试套件。

| 测试名称 | 说明 | 验证点 |
|---------|------|--------|
| `TestMalformedQueries` | 格式错误查询处理 | 空消息、无问题段、有效 A 查询的响应 |
| `TestNXDomainHandling` | NXDOMAIN 响应处理 | 不存在域名的返回码验证 |
| `TestTimeoutHandling` | 超时处理测试 | 1ms、100ms、5s 不同超时设置的响应 |
| `TestUnsupportedRecordTypes` | 不支持记录类型处理 | HINFO、MINFO、LOC、RP、AFSDB、X25、ISDN、RT、SRV、DNAME |
| `TestQueryWithEDNS` | EDNS 扩展处理 | EDNS0 支持、DO 位处理 |
| `TestMultipleQuestions` | 多问题查询处理 | 单个消息中多个问题的处理 |
| `TestTruncatedResponse` | 截断响应处理 | UDP 截断标志、TCP 回退 |
| `TestReverseLookup` | 反向 DNS 查询 | 8.8.8.8、1.1.1.1、OpenDNS 的 PTR 记录 |
| `TestLocalhostQueries` | 本地主机查询 | localhost A/AAAA、127.0.0.1 PTR |

---

### resolver_test.go

DNS 解析器集成测试套件。

| 测试名称 | 说明 | 验证点 |
|---------|------|--------|
| `TestResolverIntegration` | 解析器集成测试 | A、AAAA、MX、TXT、NS 记录的真实 DNS 解析 |
| `TestCNAMEResolution` | CNAME 链解析 | www.github.com、www.cloudflare.com 的 CNAME 跟随 |
| `TestNonExistentDomain` | 不存在域名处理 | NXDOMAIN/SERVFAIL 返回码验证 |
| `TestMultipleRecordTypes` | 多记录类型查询 | 同一域名的 A、AAAA、MX、TXT、NS、SOA 查询 |
| `TestLargeResponse` | 大响应处理 | UDP 与 TCP 大响应处理对比 |
| `TestQueryTimeout` | 查询超时测试 | 极短超时 (1ms) 下的服务器响应 |
| `TestIDNResolution` | 国际化域名 (IDN) | Punycode 编码域名解析 |
| `TestReverseDNS` | 反向 DNS 查询 | 8.8.8.8、1.1.1.1 的 PTR 记录解析 |

---

### server_test.go

服务器生命周期与并发测试套件。

| 测试名称 | 说明 | 验证点 |
|---------|------|--------|
| `TestServerLifecycle` | 服务器生命周期 | 随机端口与指定端口的启动/关闭 |
| `TestServerUDPAndTCP` | UDP/TCP 协议测试 | 同一服务器的 UDP 和 TCP 查询 |
| `TestServerGracefulShutdown` | 优雅关闭测试 | 关闭期间查询完成验证 |
| `TestServerMultipleStarts` | 多服务器实例测试 | 不同端口的多服务器实例 |
| `TestServerConcurrentQueries` | 并发查询测试 | 5 个 worker，每个 50 次查询 (250 总计) |
| `TestMockServerIntegration` | Mock 服务器集成 | 本地模拟权威服务器的查询验证 |

---

### helpers.go

测试辅助工具与 Mock 服务器实现。

| 组件 | 说明 |
|-----|------|
| `MockAuthorityServer` | 模拟 DNS 权威服务器，支持自定义 zone 数据 |
| `TestResolver` | 测试用 DNS 解析器封装 |
| `Zone` | DNS zone 数据结构，支持 referral 模式 |
| 记录构造函数 | `A()`, `AAAA()`, `CNAME()`, `MX()`, `TXT()`, `NS()`, `SOA()` |

---

## 测试环境

- **测试隔离**: 每个测试用例使用独立的服务器实例 (`127.0.0.1:0`)
- **超时设置**: 默认 5-10 秒查询超时
- **EDNS 支持**: 客户端配置 4096 字节 UDP 缓冲区
- **日志禁用**: 测试中使用 `zap.NewNop()` 禁用日志输出

## 已知问题

参考 `bug_record/` 目录下的记录：

- `2026.3.9.001` - CNAME 循环、缓存共享问题 (已修复)
- `2026.3.9.002` - 非 A 记录类型返回 SERVFAIL、截断响应处理 (已修复)
- `2026.3.10.001` - Question Section mismatch (已修复)

## 基准测试

```bash
# 运行基准测试
go test -bench=. ./e2e/...

# 基准测试：BenchmarkIntegrationQuery
# 测量端到端查询延迟
```

## 添加新测试

1. 在对应的 `*_test.go` 文件中添加测试函数
2. 使用 `server.NewServer("127.0.0.1:0")` 创建测试服务器
3. 使用 `defer s.Shutdown(ctx)` 确保清理
4. 使用 `t.Skip()` 跳过短模式下的集成测试
5. 使用 `go test -short` 可跳过长时间运行的测试