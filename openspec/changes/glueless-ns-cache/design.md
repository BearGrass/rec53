## Context

rec53 是一个迭代递归 DNS 解析器。在处理 NS referral 时，状态机流程如下：

```
LOOKUP_NS_CACHE → EXTRACT_GLUE → QUERY_UPSTREAM → CLASSIFY_RESP → ...
```

**当前问题：** 上游权威服务器有时会返回"glueless"NS referral——`Ns` section 有 NS 记录，但 `Extra` section 为空（没有对应的 A/AAAA glue 记录）。这在多 TLD 跨域委托（如 `a.shifen.com`、`nsall.huawei.com`）中非常普遍。

当前代码在三处都要求 `len(Extra) != 0`，导致：
1. glueless referral **无法写入缓存**（`state_query_upstream.go:572`）
2. 即使写入了，`LOOKUP_NS_CACHE` 也**无法命中**（`state_lookup_ns_cache.go:40`）
3. `EXTRACT_GLUE` 对 glueless response **直接返回 `NOT_EXIST`**（`state_extract_glue.go:32`），绕过了后续 `resolveNSIPs` 路径

结果：每次解析 `www.baidu.com` 或 `www.huawei.com` 都要从根重新迭代，产生 100-400ms 额外延迟。

**现有的正确路径：** `QUERY_UPSTREAM.handle` 中已有处理 glueless 的逻辑（`state_query_upstream.go:409-430`）——当 `len(ipList) == 0 && len(Ns) > 0` 时，调用 `resolveNSIPs`（缓存查询）或 `resolveNSIPsConcurrently`（全量解析）获取 NS IP。问题在于这个路径必须先到达 `QUERY_UPSTREAM`，但 `EXTRACT_GLUE` 把 glueless response 直接挡在了外面。

## Goals / Non-Goals

**Goals:**
- 修复写侧：`QUERY_UPSTREAM` 同时缓存 glueless NS referral（仅有 Ns、无 Extra）
- 修复读侧：`LOOKUP_NS_CACHE` 允许命中仅含 Ns 的缓存条目
- 修复通路：`EXTRACT_GLUE` 允许 glueless NS response（有 Ns、无 Extra）通过，进入 `QUERY_UPSTREAM` 已有的 NS IP 解析路径
- 新增单元测试覆盖三处修复
- 确保 `go test -race -timeout 120s ./server/... ./e2e/...` 全部通过

**Non-Goals:**
- 不修改状态机主循环或状态常量
- 不引入新的缓存数据结构或新的缓存 key 格式
- 不处理 NS 解析深度限制（`contextKeyNSResolutionDepth`，现有防死锁机制保留不变）
- 不实现负响应缓存（NXDOMAIN/NODATA，RFC 2308）

## Decisions

### D1：glueless 缓存使用同一 `setCacheCopy` 写入

**方案：** 在 `state_query_upstream.go:572` 将现有条件 `len(Ns) != 0 && len(Extra) != 0` 拆分：有 Extra 时同原来写入带 glue 的条目，无 Extra 时也用同一个 `setCacheCopy` 写入仅含 Ns 的条目。

**理由：** `getCacheCopy` / `setCacheCopy` 使用 NS zone 名称作为 key，`LOOKUP_NS_CACHE` 也用同一 key 查询，无需引入新接口。两种 referral 类型统一走同一个缓存路径，保持一致性。

**备选方案（已否决）：** 用 `setCacheCopyByType(name, dns.TypeNS, ...)` 单独存。否决原因：`LOOKUP_NS_CACHE` 读侧使用 `getCacheCopy`，改变写侧 key 格式会破坏读侧逻辑，需要额外适配。

### D2：`LOOKUP_NS_CACHE` 读侧放开 Extra 限制

**方案：** 将 `state_lookup_ns_cache.go:40` 的条件从 `len(Ns) != 0 && len(Extra) != 0` 改为 `len(Ns) != 0`，当命中时直接拷贝 Ns（Extra 可能为空）并返回 `LOOKUP_NS_CACHE_HIT`。

**理由：** glueless referral 合法，Ns 不为空即表示有委托信息。后续 `EXTRACT_GLUE` + `QUERY_UPSTREAM` 路径已经能处理无 Extra 的情况（见 D3）。

### D3：`EXTRACT_GLUE` 对 glueless NS 返回 `EXTRACT_GLUE_EXIST`

**方案：** 将 `state_extract_glue.go:32` 的条件拆分：`len(Ns) != 0 && len(Extra) != 0` 时走带 glue 路径（现有逻辑，包括 zone 匹配校验）；`len(Ns) != 0 && len(Extra) == 0` 时也返回 `EXTRACT_GLUE_EXIST`，让状态机进入 `QUERY_UPSTREAM`。

**理由：** `QUERY_UPSTREAM.handle` 中已有完整的 glueless 处理逻辑（`resolveNSIPs` → `resolveNSIPsConcurrently`），不需要 `EXTRACT_GLUE` 重复实现，只需让流程通过即可。

**zone 匹配校验：** 对 glueless 情况同样需要 `dns.IsSubDomain(nsZone, queryName)` 校验，防止使用不相关 NS 委托（如 CNAME 跳跃后残留的旧 NS）。

### D4：`LOOKUP_NS_CACHE` 命中时保留已有的 zone 匹配校验

`EXTRACT_GLUE` 已经负责 zone 匹配，`LOOKUP_NS_CACHE` 不需要额外校验——继续仅检查 `len(Ns) != 0` 即可，保持职责分离。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|----------|
| glueless 缓存条目被 `LOOKUP_NS_CACHE` 命中后，后续若无法解析 NS IP（NS 服务器不可达），可能导致查询失败 | 已有 `QUERY_UPSTREAM` 的超时和错误处理路径兜底；此前 cache miss 路径同样面临相同问题 |
| `EXTRACT_GLUE` 放开限制后，glueless NS 的 zone 不匹配问题 | D3 保留了 `dns.IsSubDomain` 校验，不匹配时清空 Ns/Extra 并返回 `NOT_EXIST`，流程退回根重新解析 |
| NS 解析深度防死锁机制（`depth > 0` 返回 nil）在 glueless 场景下仍然有效，可能导致嵌套 glueless NS 无法解析 | 已知限制，这是现有防死锁设计，本次不改动；影响的是极少数两层 glueless 委托链 |

## Migration Plan

1. 修改三处代码（无需 schema/配置变更）
2. 新增单元测试
3. `go test -race -timeout 120s ./server/... ./e2e/...`
4. 运行 benchmark，更新 README

无需迁移或回滚策略——修改仅扩展了缓存写入和读取的覆盖范围，原有路径不受影响。若回滚，只需还原三处条件判断。

## Open Questions

无。所有技术决策已在上述 D1-D4 中明确。
