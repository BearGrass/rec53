## Context

`server` 包当前单元测试覆盖率 72%。本次拆分 `state_define.go` 后，新文件中有以下函数直接覆盖率为 0% 或明显不足：

| 函数 | 当前覆盖率 | 原因 |
|------|-----------|------|
| `getNSNamesFromResponse` | 0% | 无直接测试，仅通过集成路径间接调用 |
| `resolveNSIPs` | 0% | 同上 |
| `updateNSIPsCache` | 0% | 异步 fire-and-forget，集成测试不覆盖 |
| `classifyRespState.handle` | 62.5% | NODATA、优先级 4/5 分支未测 |
| `.handle` | 50.9% | bad rcode retry、question mismatch 等分支未测 |

现有测试基础设施（`MockDNSHandler`、`MockDNSServer`、`init()` logger）已在 `state_query_upstream_test.go` 中定义，可直接复用。

## Goals / Non-Goals

**Goals:**
- 对 `getNSNamesFromResponse`、`resolveNSIPs`、`updateNSIPsCache` 补充直接单元测试
- 对 `classifyRespState.handle` 覆盖所有 5 个优先级分支
- 对 `.handle` 覆盖 bad rcode retry、question mismatch、NXDOMAIN 等缺失分支
- `server` 包覆盖率提升至 ≥ 82%

**Non-Goals:**
- 不修改任何生产代码
- 不为接口方法（`getCurrentState` 等）补测试——已通过集成路径覆盖
- 不为 `resolveNSIPsRecursively`（wrapper）补测试

## Decisions

**新建 `state_classify_resp_test.go`**：`classifyRespState` 测试与 `state_query_upstream_test.go` 关联度低，单独文件更清晰，遵循"测试文件名 = 被测文件名"惯例。

**追加至 `state_query_upstream_test.go`**：`getNSNamesFromResponse`、`resolveNSIPs`、`updateNSIPsCache`、`queryUpstreamState.handle` 的新测试追加到现有文件，保持 mock 基础设施集中。

**table-driven 风格**：所有新测试遵循项目现有 `t.Run(tt.name, ...)` table-driven 惯例。

**`updateNSIPsCache` 测试策略**：函数是 fire-and-forget，通过调用后检查 cache 副作用（`getCacheCopyByType`）来验证，需在测试前后用 `FlushCacheForTest()` 隔离状态。

**`queryUpstreamState.handle` bad rcode 测试**：利用现有 `MockDNSServer` + `SetIterPort`/`ResetIterPort` 模式，构造返回 SERVFAIL/REFUSED 的 mock server，测试 primary→secondary 切换逻辑。

## Risks / Trade-offs

- `updateNSIPsCache` 是 goroutine 调用，测试需要短暂 sleep 或轮询等待副作用落地，存在少量 flakiness 风险——用足够短的 sleep（≤50ms）缓解
- `queryUpstreamState.handle` 测试依赖网络 mock，需确保端口绑定成功，继续复用现有 `NewMockDNSServer` 模式即可
