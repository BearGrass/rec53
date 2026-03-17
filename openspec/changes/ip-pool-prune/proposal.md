## Why

`globalIPPool` 是一个只增不删的 `map[string]*IPQualityV2`。每次遇到新的权威服务器 IP 就创建条目，但从不清理。对于单机 ECS + 爬虫的部署场景（2000 个域名），IP 池在稳态下约 3000-4000 条目、~1.3 MB，内存本身不是紧迫问题。但真正的隐患是：

1. **陈旧路由信息** — CDN 节点退役或 anycast IP 漂移后，旧 IP 的历史质量数据残留，永远不会被清理。
2. **探测资源浪费** — SUSPECT 状态的死亡 IP 每 30s 被 `probeAllSuspiciousIPs` 遍历和探测，即使该 IP 已经数小时未被任何查询引用。
3. **无上界增长** — 虽然当前规模可控，但缺乏任何自我约束机制，属于工程缺陷。

## What Changes

- `IPQualityV2` 新增 `lastSeen` 时间戳，在每次 `RecordLatency` / `RecordFailure` 时刷新
- `IPPool` 新增 `PruneStaleIPs` 方法，删除超过 24h 未被引用的 IP 条目
- 根服务器 IP 豁免：通过依赖注入传入豁免集合，根服务器永不被清理
- 复用现有 `periodicProbeLoop` goroutine，每 ~30min 执行一次 prune，不新增 goroutine
- 阈值硬编码为常量 `STALE_IP_THRESHOLD = 24h`，不增加配置项
- `NewIPQualityV2()` 初始化 `lastSeen` 为 `time.Now()`，保护新建条目不被误删

## Capabilities

### New Capabilities
- `ip-pool-prune`: IP 池陈旧条目清理——定期删除长时间未引用的 IP 质量跟踪条目，根服务器豁免

### Modified Capabilities

（无既有 spec 的需求变更）

## Impact

- `server/ip_pool_quality_v2.go` — 新增字段，修改两个方法
- `server/ip_pool.go` — 新增方法，修改 `StartProbeLoop` 签名（增加参数）和 `periodicProbeLoop` 逻辑
- `utils/root.go` — 新增 `ExtractRootIPs()` 辅助函数
- `cmd/rec53.go` 或 `server/server.go` — 调用方适配新的 `StartProbeLoop` 签名
- 新增测试文件 `server/ip_pool_prune_test.go`
- `docs/architecture.md` — 更新 IP 池段落
