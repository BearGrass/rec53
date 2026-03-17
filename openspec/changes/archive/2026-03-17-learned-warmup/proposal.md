## Why

rec53 目前每次重启都从零开始预热：仅查询 30 个内置高流量 TLD 的 NS 记录（Round 1）。对于真实用户的热点域名（如 `github.com`、`baidu.com`），冷启动后的首次查询仍需完整的迭代解析（300–800ms）。在运行过程中，这些热点域名的 NS 记录已被解析并缓存，但重启后全部丢失。

通过记录查询历史并在重启时回放，可以将热点域名的首次解析延迟从迭代级（300–800ms）降低到缓存级（<5ms）。

## What Changes

- 新增 `server/learned_warmup.go`：衰减 LFU 计数器，跟踪每个 eTLD+1 的查询频率
- 新增 `server/warmup.go` Round 2 阶段：启动时从学习文件并发预热热点域名的 NS 记录
- 在 DNS 查询成功返回路径上记录命中的 eTLD+1
- `cmd/rec53.go`：`Config` 新增 `learned_warmup:` 块
- `go.mod`：引入 `golang.org/x/net/publicsuffix`

## Capabilities

### New Capabilities

- `learned-warmup`：基于历史查询的衰减 LFU 学习型预热，跨重启保留热点域名 NS 记录，缩短冷启动延迟

### Modified Capabilities

（无现有 capability 的 requirement 发生变更）

## Impact

- **新依赖**：`golang.org/x/net/publicsuffix`（eTLD+1 提取）
- **新文件**：`server/learned_warmup.go`、`~/.rec53/learned.json`（运行时生成）
- **修改文件**：`server/warmup.go`、`cmd/rec53.go`、`server/state_machine.go`（或查询成功路径）
- **配置变更**：`config.yaml` 新增可选 `learned_warmup:` 块（默认 disabled，向后兼容）
- **磁盘**：学习文件每 5 分钟覆写一次，大小上限约 50KB（top_n=200 条目）
