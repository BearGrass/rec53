## 1. 修复 inGlueState NS 域相关性校验

- [x] 1.1 修改 `server/state_define.go` 中 `inGlueState.handle`：在返回 `IN_GLUE_EXIST` 之前，使用 `dns.IsSubDomain(nsZone, queryName)` 校验 `response.Ns[0].Header().Name` 是否是当前查询域的祖先；若无关则清空 `response.Ns/Extra` 并返回 `IN_GLUE_NOT_EXIST`
- [x] 1.2 确保根区域（`"."`）NS 始终被视为有效（`dns.IsSubDomain(".", anyDomain)` 返回 true，无需特殊处理，验证一下即可）

## 2. 单元测试：inGlueState 校验逻辑

- [x] 2.1 在 `server/state_define_test.go` 中新增 `TestInGlueStateNSRelevance` 表格驱动测试，覆盖：NS 区域是查询域祖先（应返回 `IN_GLUE_EXIST`）、NS 区域与查询域无关（应返回 `IN_GLUE_NOT_EXIST` 且 Ns/Extra 被清空）、NS 为空（应返回 `IN_GLUE_NOT_EXIST`）、NS 区域为根 `"."`（应返回 `IN_GLUE_EXIST`）

## 3. 集成测试：多跳跨域 CNAME 冷缓存解析

- [x] 3.1 在 `server/state_define_test.go` 中新增 `TestCrossdomainCNAMEColdCacheResolves`：使用分层 mock DNS server（按查询域名返回不同响应，模拟 domain1→CNAME→domain2→CNAME→domain3→A 的委托链），验证 `Change()` 首次调用即返回正确 A 记录，不返回 SERVFAIL
- [x] 3.2 验证相同委托区域内的 CNAME 跳转（如 foo.akadns.net → bar.akadns.net）仍能复用 akadns.net 的 NS glue，不做不必要的重新委托

## 4. 回归验证

- [x] 4.1 运行 `go test -race -timeout 120s ./server/...` 确保全部通过
- [x] 4.2 运行 `go build ./cmd` 确保编译无误
