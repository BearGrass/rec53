## 1. lastSeen 时间戳

- [x] 1.1 `server/ip_pool_quality_v2.go` — `IPQualityV2` 新增 `lastSeen time.Time` 字段，`NewIPQualityV2()` 中初始化为 `time.Now()`
- [x] 1.2 `server/ip_pool_quality_v2.go` — `RecordLatency` 和 `RecordFailure` 方法中更新 `lastSeen = time.Now()`
- [x] 1.3 `server/ip_pool_quality_v2.go` — 新增 `GetLastSeen() time.Time` 方法（内部 `RLock`），供 `PruneStaleIPs` 安全读取 `lastSeen`

## 2. 根服务器 IP 提取

- [x] 2.1 `utils/root.go` — 新增 `ExtractRootIPs() map[string]struct{}`，从 `GetRootGlue().Extra` 提取所有 A 记录 IP

## 3. PruneStaleIPs 核心方法

- [x] 3.1 `server/ip_pool.go` — `IPPool` 结构体新增 `exemptIPs map[string]struct{}` 和 `lastPruneAt time.Time` 字段
- [x] 3.2 `server/ip_pool.go` — `StartProbeLoop` 签名改为 `StartProbeLoop(exemptIPs map[string]struct{})`，保存 exemptIPs 到 IPPool
- [x] 3.3 `server/ip_pool.go` — 新增包级常量 `STALE_IP_THRESHOLD = 24 * time.Hour` 和 `PRUNE_INTERVAL = 30 * time.Minute`
- [x] 3.4 `server/ip_pool.go` — 新增 `PruneStaleIPs(threshold time.Duration)` 方法：写锁遍历 poolV2，通过 `GetLastSeen()` 读取时间戳，删除超阈值且不在 exemptIPs 中的条目；执行后 log `[PRUNE] pruned N stale IPs (pool size: M → K)`
- [x] 3.5 `server/ip_pool.go` — `periodicProbeLoop` 中基于 `lastPruneAt` 与 `PRUNE_INTERVAL` 的 wall-clock 判定触发 prune（不使用 tick 计数）

## 4. 调用方适配

- [x] 4.1 调用 `StartProbeLoop` 的默认运行路径适配新签名：传入 `utils.ExtractRootIPs()` 返回的豁免集合

## 5. 测试

- [x] 5.1 `server/ip_pool_prune_test.go` — 测试超过阈值的 IP 被清理
- [x] 5.2 `server/ip_pool_prune_test.go` — 测试未超过阈值的 IP 被保留
- [x] 5.3 `server/ip_pool_prune_test.go` — 测试豁免集合中的 IP 不被清理（即使超阈值）
- [x] 5.4 `server/ip_pool_prune_test.go` — 测试空池调用无 panic
- [x] 5.5 `server/ip_pool_prune_test.go` — 测试新建 IPQualityV2 的 lastSeen 不为零值
- [x] 5.6 `server/ip_pool_prune_test.go` — 测试 RecordLatency/RecordFailure 更新 lastSeen
- [x] 5.7 `utils/root_test.go` — 测试 ExtractRootIPs 返回 13 个根服务器 IP
- [x] 5.8 `server/ip_pool_prune_test.go` — 测试定期 prune 触发条件基于 elapsed time（而非 tick 计数）
- [x] 5.9 运行 `go test -race ./...` 全量通过

## 6. 文档

- [x] 6.1 `docs/architecture.md` — 更新 IP 池段落，说明 prune 机制
- [x] 6.2 `.rec53/ROADMAP.md` — 更新 v0.3.0 状态
