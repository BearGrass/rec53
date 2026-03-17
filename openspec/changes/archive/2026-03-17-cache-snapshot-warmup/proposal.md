## Why

rec53 作为 Ubuntu 桌面的 systemd-resolved 替代品，重启后冷启动首包延迟达 309ms（实测），而 systemd-resolved 转发模式通常 < 50ms。根本原因是迭代解析器重启后缓存清空，每次查询需要 3-4 次网络往返重新建立 NS 委托链。

## What Changes

- 新增**停机快照**：graceful shutdown 时将当前 NS 委托缓存序列化为 JSON 文件（路径由 `snapshot.file` 配置，留空等价于禁用）
- 新增**启动恢复**：在 `cmd/main()` 的 `rec53.Run()` 之前同步调用 `server.LoadSnapshot()`，将 NS 条目直接写入 `globalDnsCache`，保证首包到达前缓存已就绪
- 快照**完全可选**（`snapshot.enabled: false` 默认关闭），向后兼容
- 快照文件不存在或损坏时降级为仅 Round 1 预热，不影响服务启动

## Capabilities

### New Capabilities

- `ns-cache-snapshot`: 停机时持久化 NS 委托缓存，启动时恢复，消除重启后的冷启动迭代代价

### Modified Capabilities

- `dns-warmup`: 无变更（Round 1 TLD 预热行为不变；快照恢复在 warmup 之外、Run() 之前同步完成）

## Impact

- **新增文件**：`server/snapshot.go`（快照读写逻辑）
- **修改文件**：
  - `server/server.go`：`Shutdown()` 中触发快照写入；`NewServerWithFullConfig` 增加 `SnapshotConfig` 参数
  - `cmd/rec53.go`：`Config` 增加 `Snapshot SnapshotConfig` 字段；`main()` 在 `Run()` 前调用 `LoadSnapshot()`
  - `generate-config.sh`：新增注释示例块
- **新增依赖**：无（仅使用标准库 `encoding/json`、`os`、`path/filepath`）
- **不影响**：查询路径、状态机、IP 池、转发、hosts 逻辑
