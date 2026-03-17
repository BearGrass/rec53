## Context

O-024 的目标能力（并发解析 NS 名称、首个成功优先返回、后台缓存更新、context 取消与防挂死）已经在 `server/state_query_upstream.go` 落地，并且有 `e2e/concurrent_ns_test.go` 与 `server/state_query_upstream_test.go` 的覆盖。当前主要问题不是实现缺失，而是项目管理状态未收敛：`.rec53/BACKLOG.md` 仍将 O-024 置于 `Planned`。

本次变更属于收尾审计，目的是统一实现状态、测试证据和 backlog 状态，形成可追溯结项记录。

## Goals / Non-Goals

**Goals:**
- 形成 O-024 的收尾审计工件，明确“已实现/已验证/已收敛”。
- 将 O-024 从 `Planned` 迁移到 `Completed`，并附完成摘要与验证命令。
- 在完成说明中声明性能验收采用现有 benchmark（`BenchmarkFirstPacket`），不新增专项基准。

**Non-Goals:**
- 不修改 `resolveNSIPsConcurrently` 的并发与递归策略。
- 不调整 `maxConcurrentNSQueries` 常量和任何配置接口。
- 不新增解析功能、协议行为或外部 API。

## Decisions

### Decision 1: 本次按“审计收尾”处理，而非 O-024 二期功能开发

- Rationale: 代码与测试已覆盖 O-024 目标能力，当前缺口是文档/状态收敛；继续功能开发会扩大范围并混淆结项边界。
- Alternatives considered:
  - 直接做 O-024 二期增强（并发配置化、depth 策略升级）: 拒绝，因不符合本次“收尾”目标。
  - 保持现状不收敛: 拒绝，会持续造成 backlog 与实现状态不一致。

### Decision 2: 性能证据沿用现有 benchmark，不新增专项 benchmark

- Rationale: 用户明确选择沿用现有基准；本次重点是结项一致性，不引入新的性能工作项。
- Alternatives considered:
  - 新增 O-024 专项 benchmark: 拒绝，超出本次范围，且会引入额外实现与评审成本。

### Decision 3: 以“可追溯证据链”作为验收标准

- Rationale: 审计类变更需要可复核，必须能从 backlog 条目追踪到测试命令和对应代码位置。
- Alternatives considered:
  - 仅改状态字段（Planned -> Completed）: 拒绝，证据不足，不利于后续审计与回归。

## Risks / Trade-offs

- [风险] 仅做状态收敛可能掩盖未来网络抖动下的边缘退化。
  - Mitigation: 在完成说明中明确“本次不改行为”，未来性能/稳健性增强应单独立项（例如 O-024 v2）。
- [风险] 依赖现有 benchmark 可能无法隔离 O-024 的纯增益。
  - Mitigation: 在完成摘要中标注“采用现有基准作为佐证”，避免将其解读为专项性能证明。
- [权衡] 不引入新实现可降低回归风险，但也不解决潜在深层优化诉求。
  - Mitigation: 保持变更最小化，后续优化单独提案。
