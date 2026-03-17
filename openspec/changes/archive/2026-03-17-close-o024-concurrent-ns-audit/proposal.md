## Why

O-024（并发查询 NS IP）在代码与测试层面已经实现，但 backlog 状态仍停留在 `Planned`，导致“实现状态”和“项目文档状态”不一致。当前需要一次收尾审计，把实现证据、测试证据和状态变更统一到可追溯的结项结果。

## What Changes

- 新增一个 O-024 收尾审计变更，明确本次仅做“结项收敛”，不新增解析功能、不调整并发算法。
- 归档 O-024 已实现证据：并发 NS 解析、首个成功优先返回、后台缓存更新、context 取消与防挂死保护。
- 归档 O-024 已覆盖证据：E2E 并发场景与 server 侧并发/取消回归测试。
- 将 `.rec53/BACKLOG.md` 中 O-024 从 `Planned` 迁移到 `Completed`，并补充完成说明和验证命令。
- 在完成说明中声明性能验收沿用现有基准（`e2e/first_packet_bench_test.go`），不新增专项 benchmark。

## Capabilities

### New Capabilities

- `o024-closure-audit`: 定义 O-024 结项收敛的标准，包括实现/测试证据归档、backlog 状态迁移、以及基于现有基准的性能验收记录。

### Modified Capabilities

- （无）

## Impact

- `openspec/changes/close-o024-concurrent-ns-audit/`: 新增 proposal/design/specs/tasks 工件。
- `.rec53/BACKLOG.md`: O-024 条目由 `Planned` 迁移至 `Completed` 并补充完成摘要。
- 不影响 `server/state_query_upstream.go` 当前行为与常量 `maxConcurrentNSQueries`。
- 不引入新的配置字段、外部依赖或对外 API 变更。
