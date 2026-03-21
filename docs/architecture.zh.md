# 架构说明

[English](architecture.md) | 中文

`docs/architecture.md` 面向开发者，说明 rec53 如何解析 DNS 请求、核心模块如何协作，以及修改代码时需要关注的约束。部署和运维指南请查看 `docs/user/` 下的文档。

## 范围

rec53 是一个节点本地递归 DNS 解析器，目标是在每台机器或每个节点上运行一个解析器。项目重点是：

- 可预测的递归解析行为
- 低运维开销的本地部署
- 保守的并发与缓存安全
- 清晰的失败与关闭行为

它不是中心化递归 DNS 集群管理器。

## 仓库映射

| 路径 | 作用 |
|---|---|
| `cmd/rec53.go` | CLI 参数、配置加载、日志/指标/服务启动、信号处理 |
| `server/` | DNS 请求处理、状态机、缓存、IP 池、warmup、snapshot、XDP |
| `monitor/` | 日志、Prometheus 指标、pprof 支持 |
| `utils/` | 根服务器数据和 zone 工具 |
| `e2e/` | 端到端与集成测试 |
| `tools/` | 内部基准和验证工具 |
| `docs/user/` | 面向运维者的文档 |
| `docs/dev/` | 面向开发者的工作流和发布文档 |

## 请求生命周期

整体流程：

```text
client query
  -> server.ServeDNS()
  -> Change(stateMachine)
  -> response classification / iterative steps
  -> response normalization
  -> optional UDP truncation
  -> metrics + write back to client
```

入口：

- `server.ServeDNS()` 创建回复消息，校验 `QDCOUNT`，记录指标，然后交给状态机。
- `server/state_machine.go` 中的 `Change()` 最多推进 50 次，避免无限循环。
- 最终响应会被规范化，确保在 CNAME 追踪或内部重写后仍保留原始 question。

## 解析流水线

状态机显式编码了解析顺序：

```text
STATE_INIT
  -> HOSTS_LOOKUP
  -> FORWARD_LOOKUP
  -> CACHE_LOOKUP
  -> CLASSIFY_RESP
  -> EXTRACT_GLUE
  -> LOOKUP_NS_CACHE
  -> QUERY_UPSTREAM
  -> RETURN_RESP
```

优先级：

1. `HOSTS_LOOKUP`
2. `FORWARD_LOOKUP`
3. `CACHE_LOOKUP`
4. 迭代解析

这样设计是为了：

- hosts 必须短路其他逻辑
- forwarding 必须绕过本地迭代行为
- cache 只负责降低延迟，不改变正确性
- 只有 miss 才支付完整迭代成本

## 状态职责

| 状态 | 职责 |
|---|---|
| `STATE_INIT` | 校验请求，初始化响应头 |
| `HOSTS_LOOKUP` | 返回本地静态记录，包括 name 命中但 type 不匹配时的 NODATA |
| `FORWARD_LOOKUP` | 将匹配 zone 的请求转发到指定上游 |
| `CACHE_LOOKUP` | 按 `fqdn:qtype` 读取缓存副本 |
| `CLASSIFY_RESP` | 区分最终答案、负响应、CNAME、委派 |
| `EXTRACT_GLUE` | 从 additional section 提取委派 NS 的 A 记录 |
| `LOOKUP_NS_CACHE` | 从缓存或根服务器恢复 NS IP |
| `QUERY_UPSTREAM` | 用类 happy-eyeballs 并发方式查询选定 nameserver |
| `RETURN_RESP` | 恢复原始 question 并补回 CNAME 链 |

两个循环最重要：

- 委派循环：referral -> glue/ns 解析 -> upstream 查询 -> 再次分类
- CNAME 循环：跟随 CNAME target，保留链，然后重新进入 cache/iterative 路径

保护措施：

- `MaxIterations = 50` 限制状态机总步数
- `visitedDomains` 防止 CNAME 环
- `contextKeyNSResolutionDepth` 防止 NS 解析死锁

## 缓存模型

DNS cache 是全局且按类型区分的。

- 实现：`server/cache.go`
- key 格式：`"fqdn:qtype"`
- 存储后端：`github.com/patrickmn/go-cache`

不变量：

- 读取返回 `dns.Msg` 的浅拷贝
- 调用方可以替换或追加 RR slice
- 调用方不能修改缓存中 RR 的单个字段
- 写入缓存前会剥离 OPT 记录，避免并发 `Pack()` 风险

行为规则：

- 正响应按 TTL 缓存
- 负响应按 SOA TTL 缓存，必要时回退到 `DefaultNegativeCacheTTL`
- forwarding 返回不缓存
- 读取前会先复制，避免请求本地修改污染共享状态

## IP 池与上游选择

IP 池负责追踪 nameserver 质量，并驱动上游选择。

- 实现：`server/ip_pool.go`、`server/ip_pool_quality_v2.go`
- 全局：`globalIPPool`
- 输入：根 IP、glue IP、递归解析得到的 NS IP

关键行为：

- `GetBestIPsV2` 返回首选和次选上游 IP
- `queryHappyEyeballs` 在可能时竞速这两个候选
- 失败会调用 `RecordFailure`
- 成功 RTT 会调用 `RecordLatency`
- 后台探测循环帮助退化的 NS 条目恢复

这个系统故意保持简单：选择是自适应的，但避免策略过重或分布式协调。

## Hosts 与 forwarding 快照

Hosts 和 forwarding 规则会被编译成不可变快照，并通过 `atomic.Pointer` 发布。

- 实现：`server/state_shared.go`
- 快照内容：`hostsMap`、`hostsNames`、`forwardZones`

原因：

- 读路径保持无锁
- 配置消费者不会看到半更新结构
- 测试可以安装合成快照而不重建服务

## Warmup、Snapshot 和 XDP

这些特性扩展默认路径，但不是基础部署所必需。

### Warmup

- `server/warmup.go`
- 启动时预热根和部分 TLD 的 NS 记录
- 后台运行，不能阻塞服务就绪

### Cache snapshot

- `server/snapshot.go`
- 在关闭时持久化缓存，并在启动时恢复
- 应视为运维优化，而不是正确性依赖

### XDP/eBPF 快速路径

- `server/xdp_loader.go`、`server/xdp_sync.go`、`server/xdp_metrics.go`、`server/xdp/`
- 仅在受支持的 Linux 环境上启用
- 在 DNS listener 启动前附加
- 如果加载或 attach 失败，会回退到 Go-only cache 路径

XDP 必须保持可选。Go 路径是发布基线。

## 启动与关闭约束

服务生命周期集中在 `server/server.go`。

启动规则：

- 先构建所有 UDP/TCP listener 结构体
- 在 DNS listener 之前初始化可选 XDP
- 异步启动 warmup
- 只有 listener 真正 bind 成功后才发布 ready 地址

关闭规则：

- 在拆共享服务前取消 warmup
- 确定性地关闭 listener
- 先停止后台循环，再关闭共享资源
- 只有在请求处理停止后再保存 cache snapshot

修改生命周期代码时，优先保留这些保证，而不是引入新抽象。

## 测试期望

仓库依赖单元、包级和端到端测试的组合。

- 包级行为在 `server/`、`cmd/`、`monitor/`、`utils/` 下
- 集成测试在 `e2e/` 下
- 并发敏感变更要用 `-race`

触碰热路径或生命周期逻辑时，应补充覆盖：

- 畸形请求
- 启动失败行为
- 关闭清理
- 缓存安全不变量
- forwarding / hosts 优先级

命令和测试建议见 `docs/dev/testing.md`。
