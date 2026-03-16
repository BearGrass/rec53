## 1. 创建对比文档

- [x] 1.1 创建 `docs/sdns-comparison.md`，包含元数据头部（sdns 版本 v1.6.1，对比日期 2026-03-16）
- [x] 1.2 编写 DNS 解析能力对比章节（递归解析、记录类型、CNAME、QNAME 最小化、空区域等）
- [x] 1.3 编写传输协议支持对比章节（UDP/TCP、DoT、DoH、DoQ）
- [x] 1.4 编写缓存系统对比章节（缓存策略、预取、缓存清除 API）
- [x] 1.5 编写安全功能对比章节（DNSSEC、反射攻击防御、域名封锁、EDNS Cookie）
- [x] 1.6 编写访问控制与速率限制对比章节
- [x] 1.7 编写监控与可观测性对比章节（Prometheus 指标、Dnstap、HTTP 管理 API）
- [x] 1.8 编写 Kubernetes 集成对比章节
- [x] 1.9 编写扩展架构对比章节（中间件、插件系统）
- [x] 1.10 为每个 rec53 缺失的功能标注建议优先级（High / Medium / Low / Out-of-scope）
- [x] 1.11 在文档末尾添加"总结：rec53 的定位与差距"章节

## 2. 更新 README 文档索引

- [x] 2.1 在 `README.md` 的文档索引章节添加 `docs/sdns-comparison.md` 链接
- [x] 2.2 在 `README.zh.md` 的文档索引章节添加 `docs/sdns-comparison.md` 链接
