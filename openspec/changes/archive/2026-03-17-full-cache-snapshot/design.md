## Context

rec53 当前已实现 NS 委派缓存快照持久化（`server/snapshot.go`），关机时保存、启动时恢复。
`SaveSnapshot` 遍历 `globalDnsCache.Items()`，仅保存 `msg.Ns` 中含有 `*dns.NS` 记录的条目。
`remainingTTL` 只检查 `msg.Ns` 和 `msg.Extra` 两个 section 的 TTL。

这意味着 A/AAAA 答案记录、CNAME 链等缓存条目在重启后丢失，每个域名首次查询仍需 1-2 次上游往返。
对于单机生产部署（应用 + Docker + 垂类爬虫），重启后的冷启动惩罚可达数分钟。

快照的 JSON 结构（`snapshotEntry`：key / msg_b64 / saved_at）通用性良好，
无需修改即可承载任意类型的 `dns.Msg` 条目。

## Goals / Non-Goals

**Goals:**

- 快照保存全部缓存条目，重启后热门域名首次查询即缓存命中
- 保持向后兼容：新版本可读取旧版本快照文件
- 改动量最小化：仅修改过滤逻辑和 TTL 计算范围

**Non-Goals:**

- 不做访问频次（hit_count）统计或热度排序
- 不做快照条目数截断或容量限制
- 不做 IP 质量池（RTT / 健康状态）持久化
- 不引入新的配置项

## Decisions

### D1：移除 NS-only 过滤，保存全部条目

**选择**：删除 `SaveSnapshot` 中的 `hasNS` 过滤循环，对 `globalDnsCache.Items()` 中所有可序列化的 `*dns.Msg` 条目执行保存。

**替代方案**：仅新增 A/AAAA 类型白名单过滤（ChatGPT 建议）。

**理由**：白名单需要显式枚举记录类型并维护过滤条件，而 `remainingTTL` 的 TTL 过期机制已天然过滤短寿命条目（负缓存 60s TTL 在重启后几乎必定过期）。全量保存代码更简单，且不会遗漏 CNAME 等有价值的中间记录。

### D2：`remainingTTL` 扩展检查 `msg.Answer` section

**选择**：在现有 `msg.Ns` + `msg.Extra` 基础上，增加对 `msg.Answer` section 的 TTL 遍历。取三个 section 中最小的剩余 TTL 作为缓存过期时间。

**理由**：纯答案记录（如 A/AAAA）的 RR 仅存在于 `msg.Answer` 中，`msg.Ns` 和 `msg.Extra` 可能为空。不扩展此函数会导致这些条目的 `remainingTTL` 返回 0，被误判为过期而跳过恢复。

### D3：快照 JSON 结构不变

**选择**：复用现有 `snapshotEntry`（key / msg_b64 / saved_at）和 `snapshotFile`（entries 数组），不新增字段。

**理由**：`dns.Msg.Pack()` 已经是通用的 wire-format 序列化，不区分记录类型。JSON 结构天然兼容全量条目，无需扩展。旧版本快照文件可被新版本正常读取（内容是旧版本保存的 NS 子集）。

## Risks / Trade-offs

**快照文件体积增长** → 从 KB 级增长到 1-10 MB（万级条目场景）。对于单机部署完全可接受。若未来出现十万级条目场景，可在此基础上追加截断逻辑，但当前不需要。

**关机时 `Items()` 拷贝开销** → `go-cache.Items()` 在 RLock 下拷贝整个 map。万级条目时耗时 ~1-5 ms，远低于 5 秒关机超时。

**过期负缓存条目被保存到磁盘** → 浪费少量磁盘空间（负缓存条目体积小），恢复时被 TTL 过期丢弃，无功能影响。
