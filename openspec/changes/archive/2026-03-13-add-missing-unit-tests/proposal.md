## Why

`server` 包整体覆盖率为 72%，但以下核心迭代逻辑函数覆盖率为 0% 或低于 55%：`getNSNamesFromResponse`、`resolveNSIPs`、`updateNSIPsCache`、`classifyRespState.handle`、`queryUpstreamState.handle`。这些函数处于 DNS 迭代解析的关键路径上，缺乏直接单元测试会导致逻辑 bug 难以早期发现。

## What Changes

- 在 `server/state_query_upstream_test.go` 中补充针对 `getNSNamesFromResponse`、`resolveNSIPs`、`updateNSIPsCache` 的单元测试
- 在新文件 `server/state_classify_resp_test.go` 中补充 `classifyRespState.handle` 的缺失分支测试（当前 62.5%）
- 在 `server/state_query_upstream_test.go` 中补充 `queryUpstreamState.handle` 的缺失分支测试（当前 50.9%）

## Capabilities

### New Capabilities

- `query-upstream-helpers-tests`: 对 `getNSNamesFromResponse`、`resolveNSIPs`、`updateNSIPsCache` 的单元测试
- `classify-resp-branch-tests`: 对 `classifyRespState.handle` 全分支覆盖测试
- `query-upstream-handle-tests`: 对 `queryUpstreamState.handle` 缺失分支（NXDOMAIN retry、bad rcode retry、question mismatch 等）的测试

### Modified Capabilities

## Impact

- 仅新增测试代码，不修改任何生产代码
- 覆盖率预期从 72% 提升至 ~82%
- 受影响文件：`server/state_query_upstream_test.go`（追加）、`server/state_classify_resp_test.go`（新建）
