## Context

rec53 启动时执行 Round 1 预热：并发查询 30 个内置高流量 TLD 的 NS 记录，将结果写入 `globalIPPool` 和 `globalDnsCache`。这使 TLD 级解析的首次查询延迟从迭代级（300–800ms）降到缓存级（<5ms）。

Round 1 的局限：它只覆盖 TLD（`com.`、`net.`、`org.` 等），不覆盖二级域名（如 `github.com`）。用户实际热点往往集中在少数几十个注册域名，重启后这些域名的 NS 仍需完整迭代解析。

目标是在 Round 1 之后加入 Round 2：从持久化学习文件读取用户历史热点域名，并发预热其 NS 记录。

**现有相关代码：**
- `server/warmup.go`：`WarmupNSRecords(ctx, cfg)` 驱动 Round 1，`queryNSRecords(ctx, domain)` 执行单次合成查询
- `server/state_machine.go`：`Change(stm)` 状态机入口，查询结果自动写缓存和 IP 池
- `cmd/rec53.go`：`Config.Warmup WarmupConfig` 控制预热参数

## Goals / Non-Goals

**Goals:**
- 运行时统计每个查询的 eTLD+1，用衰减 LFU 维护热度分
- 定期将 top-N 热点域名持久化到 JSON 文件（覆写，防止无限增长）
- 启动时从学习文件并发预热热点域名的 NS 记录（Round 2）
- 功能完全可选（`enabled: false` 时无任何影响），向后兼容

**Non-Goals:**
- 不缓存 A/AAAA 应答记录，只预热 NS 层（NS 层命中即可消除 300ms+ 的迭代延迟）
- 不提供 HTTP API 或管理界面查看学习数据
- 不做跨节点同步
- 不处理 DNSSEC 相关记录

## Decisions

### 决策 1：统计粒度为 eTLD+1（注册域名），不是 FQDN

**选择**：提取每次成功查询的 eTLD+1（如 `github.com`），而非完整 FQDN（如 `api.github.com`）。

**原因**：
- NS 记录属于注册域名，不属于子域名；预热 `github.com` 的 NS 等同于覆盖所有 `*.github.com`
- eTLD+1 大幅减少条目数量，top-200 即可覆盖绝大多数用户热点
- 使用 `golang.org/x/net/publicsuffix` 提取，无需自维护公共后缀表

**替代方案（放弃）**：按 FQDN 统计 — 条目爆炸，且同一注册域名下的子域名条目分散无法聚合

### 决策 2：衰减 LFU，每次查询 score += 1，后台每小时 score × 0.9

**选择**：指数衰减 LFU，而非纯 LFU 或 LRU。

**原因**：
- 纯 LFU 无法遗忘老旧热点（域名停用后永久占据排名）
- LRU 忽略频率，单次访问与千次访问等价
- 衰减 LFU 兼顾频率和时效：活跃域名分数持续高，不活跃域名分数缓慢归零

**参数**：`decay_factor: 0.9`（默认），每小时衰减约 10%，约 21 小时衰减至 10%，约 2 天趋近于 0；可通过配置调整。

### 决策 3：学习文件覆写而非追加，存储 top-N 条目

**选择**：每次 flush 时将内存中 top-N 条目序列化为 JSON 覆写文件，不追加。

**原因**：
- 追加模式会导致文件无限增长，需要额外 GC 逻辑
- 覆写天然限制文件大小（top-200 条目约 20–50KB）
- 内存中全量维护，flush 只是快照：不丢历史，只截断长尾

### 决策 4：Round 2 在 Round 1 完成后、`server.Run()` 返回前异步启动

**选择**：Round 2 在 Round 1 结束后同步执行，共享同一个 warmup deadline context。

**原因**：
- 共享 deadline 保证启动总时间有上限，不阻塞服务监听
- Round 1 结束后 TLD 层已预热，Round 2 的迭代解析成本更低
- 实现简单，无需单独 goroutine 和 context 管理

**替代方案（放弃）**：完全异步后台 goroutine — 服务已开始接受请求时预热尚未完成，效果与不预热相当

### 决策 5：查询统计在状态机成功返回路径上记录，不在各 state 内记录

**选择**：在 `server/server.go` 的 `ServeDNS` 方法中，于状态机 `Change(stm)` 成功返回后调用 `globalLearnedWarmup.Record(qname)`。

**原因**：
- 集中一处，不散布在各 state 中
- `ServeDNS` 已持有原始请求和响应，context 完整
- 不影响状态机逻辑

## Risks / Trade-offs

- **[风险] publicsuffix 提取失败** → 对无法解析 eTLD+1 的域名（如单标签域名、纯 IP）直接跳过记录，不影响查询本身
- **[风险] 学习文件损坏** → 启动时解析失败时降级为空学习集，打印 warning，不 fatal
- **[风险] 磁盘写失败** → flush 失败只记录 error log，不影响 DNS 服务；内存数据保留到下次 flush 重试
- **[Trade-off] Round 2 延长启动时间** → Round 2 共享 warmup deadline（默认 5s），超时自动截止；用户可通过 `top_n` 减少预热域名数
- **[Trade-off] 额外依赖** → `golang.org/x/net/publicsuffix` 是 Go 官方 extended 库，维护风险极低

## Migration Plan

1. 新增 `server/learned_warmup.go`，独立模块，不改变现有接口
2. `server/warmup.go` 新增 `WarmupLearnedDomains(ctx, cfg, lw)` 函数，Round 1 完成后调用
3. `server/server.go` `ServeDNS` 成功路径增加 `globalLearnedWarmup.Record(qname)` 调用
4. `cmd/rec53.go` 扩展 Config，新增 `LearnedWarmup` 字段（`enabled: false` 默认关闭）
5. 全程可 feature-flag 控制，`enabled: false` 时 `Record()`/`WarmupLearnedDomains()` 均为 no-op

## Open Questions

- 学习文件默认路径：`~/.rec53/learned.json`（用户家目录）还是 `/var/lib/rec53/learned.json`（systemd 服务更合适）？暂定 `~/.rec53/learned.json`，通过配置项覆盖。
- `flush_interval` 默认 300s（5分钟）是否合适？服务崩溃最多丢失 5 分钟的学习数据，可接受。
