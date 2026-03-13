## Why

递归解析器在处理 glueless NS 委托（即上游只返回 Ns section、没有 Extra/glue 的情况）时，既不缓存这些 NS 映射关系，也无法从缓存中读取之前解析过的 NS IP。导致 www.baidu.com（a.shifen.com 委托）和 www.huawei.com（多跳 CNAME 链 + 跨 TLD）每次解析都要从根重新迭代，产生大量冗余查询和额外延迟（100-400ms/次）。

## What Changes

- **写侧修复**：`QUERY_UPSTREAM` 在缓存带 glue 的 NS referral 的同时，也缓存 glueless NS 委托（仅有 Ns section、无 Extra 的情况），使用同一 `setCacheCopy` 接口写入独立 key。
- **读侧修复**：`LOOKUP_NS_CACHE` 放开"必须有 Extra"的限制，允许命中仅含 Ns section 的缓存条目，当 Extra 为空时将 glueless NS 委托传给后续状态，触发 `resolveNSIPsConcurrently` 解析 NS IP。
- **NS IP 缓存复用**：`resolveNSIPsConcurrently` 已将解析出的 NS IP 写入缓存（`updateNSIPsCache`）。在 CNAME 跳跃场景中，确保 `resolveNSIPs`（缓存查询路径）能优先命中这些记录，避免重复运行完整子状态机。
- **`EXTRACT_GLUE` 兼容**：允许仅含 Ns section（无 Extra）的 response 经 `EXTRACT_GLUE` 直接进入 `QUERY_UPSTREAM`，不强制要求同时存在 Extra。

## Capabilities

### New Capabilities

- `glueless-ns-cache`: 缓存无 glue 的 NS 委托，并在 LOOKUP_NS_CACHE 和 EXTRACT_GLUE 中正确读取，减少迭代解析中的重复 NS 查询

### Modified Capabilities

（无需求层面变更，只修改实现行为）

## Impact

- `server/state_query_upstream.go`：写侧，增加 glueless NS 的缓存写入逻辑
- `server/state_lookup_ns_cache.go`：读侧，放开 Extra 非空限制
- `server/state_extract_glue.go`：允许仅有 Ns 无 Extra 的 response 进入 QUERY_UPSTREAM
- `server/state_query_upstream_test.go`、`server/state_lookup_ns_cache_test.go`（如存在）：新增覆盖 glueless 路径的测试
- 不涉及 API 变更或新依赖
