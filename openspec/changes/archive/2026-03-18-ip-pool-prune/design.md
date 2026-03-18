## Context

`globalIPPool`（`map[string]*IPQualityV2`）跟踪所有接触过的权威服务器 IP 的质量信息。条目在 `GetBestIPsV2` 中按需创建，但从不删除。现有后台 `periodicProbeLoop` 每 30s 遍历池中 SUSPECT 状态的 IP 并发起探测，但对已不再被任何查询引用的陈旧 IP 没有清理机制。

当前部署：单台 ECS，运行网页爬虫（2000 个域名），IP 池稳态约 3000-4000 条目。

## Goals / Non-Goals

**Goals:**
- 定期清理长时间未被查询引用的 IP 条目，防止无上界增长
- 默认运行路径下根服务器 IP 不被清理（豁免机制）
- 零额外 goroutine：复用现有 `periodicProbeLoop`
- 零新配置项：阈值硬编码为常量
- 保持代码简单，主逻辑改动控制在 50 行以内

**Non-Goals:**
- IP 质量衰减（aging）机制——被清理后重新遇到时重置为初始值即可接受
- 配置化阈值——当前部署场景单一，无需暴露调参入口
- IP 池容量上限（cap）——prune 已足够约束增长，不需要 LRU 驱逐
- sync.Pool 或内存优化——属于 v0.4.0 范畴

## Decisions

### 1. 复用 `periodicProbeLoop`，并采用基于时间的 prune 调度

**选择**：在现有 probe loop 中维护 `lastPruneAt`，每个 tick 检查 `time.Since(lastPruneAt) >= PRUNE_INTERVAL (30m)` 时触发一次 `PruneStaleIPs`，随后更新 `lastPruneAt`。

**替代方案**：独立 `time.Ticker` goroutine。

**理由**：probe loop 已经持有 `IPPool` 的引用、已有 context 取消、已被 `sync.Once` 保护。新增 goroutine 带来的是更多 `wg.Add` / 更多 shutdown 路径、更多竞争风险，收益为零。相较 tick 计数，基于时间的调度不会因为单次 probe 过慢而造成 prune 周期漂移。

### 2. 依赖注入豁免集合，根服务器作为默认豁免

**选择**：`StartProbeLoop(exemptIPs map[string]struct{})` 签名，调用方默认传入 `utils.ExtractRootIPs()`；测试可传入自定义集合验证行为。

**替代方案 A**：`ip_pool.go` 直接 `import utils` 在 `init()` 中提取。

**理由**：B 方案使 `IPPool` 不依赖 `utils`，包依赖图不增加边。默认路径下根服务器被豁免；测试时可传入自定义豁免集合，隔离性更好。

### 3. `lastSeen` 初始化为 `time.Now()`

**选择**：`NewIPQualityV2()` 中将 `lastSeen` 设为 `time.Now()`。

**替代方案**：零值 + prune 时特判 `lastSeen.IsZero()`。

**理由**：零值的 `time.Since()` 返回 ~56 年，会导致刚创建但尚未收到响应的 IP 被误删。初始化为 `time.Now()` 语义清晰，无需特判。

### 6. 明确 `lastSeen` 的并发访问边界

**选择**：`lastSeen` 的写入仅发生在 `RecordLatency` / `RecordFailure`（持有 `IPQualityV2.mu`）；读取通过受保护的访问路径（例如 `GetLastSeen()`，内部 `RLock`）完成。`PruneStaleIPs` 不允许在未持有 `IPQualityV2.mu` 的情况下直接读取 `lastSeen`。

**理由**：`IPQualityV2` 是高并发热路径。将 `lastSeen` 纳入同一锁语义可避免 data race，并保持与现有状态字段一致的并发模型。

### 4. 阈值硬编码为常量而非配置项

**选择**：`STALE_IP_THRESHOLD = 24 * time.Hour`，包级常量。

**理由**：`lastSeen` 仅在迭代解析器实际联系权威服务器时刷新（`RecordLatency` / `RecordFailure`），缓存命中期间不更新。DNS 缓存 TTL 常见 300-3600s，爬虫调度周期可达数小时，2h 阈值会导致正常使用中的 IP 因缓存命中而恰好卡在边界被误删，造成不必要的 churn。24h 完全覆盖 DNS 缓存周期和爬虫调度周期，消除边界竞争。当前池规模 3-4K 条目 ~1.3MB，多保留 22h 的内存和探测开销可忽略。常量可在未来需要时一行改为配置项，成本极低。

### 5. 术语使用 `Prune` 而非 `GC`

**选择**：方法命名为 `PruneStaleIPs`，测试文件命名为 `ip_pool_prune_test.go`。

**理由**：`GC` 易与 Go runtime GC 混淆。`Prune`（修剪）在 DNS/网络代码中常见（CoreDNS 等），语义准确：主动清理不再需要的条目。

### 7. Prune 操作日志

**选择**：`PruneStaleIPs` 执行后输出一行 `Debugf` 日志，格式 `[PRUNE] pruned N stale IPs (pool size: M → K)`，包含删除数量和池大小变化。无条目被删除时不输出日志。

**替代方案**：每个被删除的 IP 单独输出一行日志。

**理由**：单行汇总日志在运维排查时可快速确认 prune 是否执行、规模多大。逐条日志在 3000+ 条目池中产生大量噪声。如需排查具体被删 IP，可临时提升日志级别或添加逐条输出，当前不需要。

> **Future work**：Prune 相关 Prometheus metric（`rec53_ip_pool_pruned_total` 计数器、`rec53_ip_pool_size` gauge）留待后续需要时添加，当前部署场景单一，日志足以覆盖可观测性需求。

## Risks / Trade-offs

- **[被清理 IP 重新出现时重置为初始延迟 1000ms]** → 可接受。爬虫场景下域名集合相对稳定，24h 未命中说明该 IP 确实已不活跃。重新学习的代价是 1-2 次查询的非最优选路，影响微乎其微。

- **[prune 持有写锁期间阻塞查询路径]** → 遍历 + 删除 3000 条目耗时微秒级，写锁持有时间远小于一次 DNS 往返（ms 级），不构成瓶颈。

- **[根服务器 IP 硬编码在 `utils/root.go`，变更时需同步]** → 根服务器 IP 数十年未变。即使变更，`ExtractRootIPs()` 从 `GetRootGlue()` 动态提取，只需更新 `root.go` 一处。
