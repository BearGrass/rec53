## Why

当前快照仅保存 NS 委派条目（`SaveSnapshot` 中 `hasNS` 过滤）。重启后 NS 委派链已恢复，但每个域名的最终 A/AAAA 答案仍需 1-2 次上游往返才能获得。对于单机生产部署（应用 + Docker + 垂类爬虫），重启后爬虫批量请求数千域名时全部缓存 miss，导致前几分钟吞吐量下降。将快照扩展为保存全部缓存条目，可消除这一冷启动惩罚。

## What Changes

- `server/snapshot.go` `SaveSnapshot` 移除 NS-only 过滤逻辑，保存所有可序列化的缓存条目（A/AAAA 答案、CNAME 链、NS 委派等）
- `server/snapshot.go` `remainingTTL` 扩展为同时检查 `msg.Answer`、`msg.Ns`、`msg.Extra` 三个 section 的 TTL，确保非 NS 条目（如纯 Answer 记录）也能正确计算剩余 TTL
- `server/snapshot.go` 日志与注释更新，反映"全量缓存快照"语义
- 现有恢复逻辑（TTL 过期丢弃）不变，无新配置项

## Capabilities

### New Capabilities

- `full-cache-snapshot`: 快照从 NS-only 扩展到全量缓存条目的保存与恢复

### Modified Capabilities

（无——现有 spec 无需求变更，仅实现层扩展过滤范围）

## Impact

- **代码**：`server/snapshot.go`（~20 行改动）、`server/snapshot_test.go`（新增测试用例）
- **快照文件体积**：从 ~50 条 NS 条目（KB 级）增长到 ~1,000-10,000 条全量条目（1-10 MB），仍在可接受范围
- **恢复耗时**：从 < 5 ms 增长到 < 100 ms（万级条目），不影响启动速度
- **依赖**：无新依赖
- **兼容性**：快照 JSON 结构不变（`snapshotEntry` 字段不变），新版本可读取旧版本快照文件
