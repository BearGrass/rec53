## 1. 写侧修复：缓存 glueless NS referral

- [x] 1.1 修改 `server/state_query_upstream.go:572`，在 `len(Ns) != 0 && len(Extra) != 0` 条件之外，增加对 `len(Ns) != 0 && len(Extra) == 0` 的处理，调用 `setCacheCopy` 写入 glueless NS referral

## 2. 读侧修复：LOOKUP_NS_CACHE 命中 glueless 条目

- [x] 2.1 修改 `server/state_lookup_ns_cache.go:40`，将条件 `len(Ns) != 0 && len(Extra) != 0` 改为 `len(Ns) != 0`，允许命中仅含 Ns 的缓存条目

## 3. 通路修复：EXTRACT_GLUE 允许 glueless NS 通过

- [x] 3.1 修改 `server/state_extract_glue.go:32`，对 `len(Ns) != 0 && len(Extra) == 0` 的情况也进行 `dns.IsSubDomain` zone 匹配校验，匹配时返回 `EXTRACT_GLUE_EXIST`

## 4. 单元测试

- [x] 4.1 在 `server/state_query_upstream_test.go` 中新增测试：`QUERY_UPSTREAM` 收到 glueless NS referral 时写入缓存
- [x] 4.2 在 `server/state_lookup_ns_cache_test.go`（如不存在则在 `server/` 下新建）中新增测试：`LOOKUP_NS_CACHE` 命中 glueless 缓存条目，返回 `HIT`
- [x] 4.3 在 `server/state_extract_glue_test.go`（如不存在则在 `server/` 下新建）中新增测试：`EXTRACT_GLUE` 对 glueless NS zone 匹配和不匹配两种情况

## 5. 验证

- [x] 5.1 运行 `go test -race -timeout 120s ./server/... ./e2e/...`，确保全部通过
- [x] 5.2 运行 benchmark 并对比首包延迟变化
- [x] 5.3 更新 `README.md` 和 `README.zh.md` 中的 benchmark 结果
