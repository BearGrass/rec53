## 1. 收尾审计工件整理

- [ ] 1.1 在 proposal/design/specs 中补全 O-024 已实现能力与已覆盖测试的证据链说明
- [ ] 1.2 明确本次变更为“收尾收敛”，记录不改 `server/state_query_upstream.go` 行为与不新增配置接口

## 2. Backlog 状态收敛

- [ ] 2.1 将 `.rec53/BACKLOG.md` 中 O-024 从 `Planned` 迁移到 `Completed`
- [ ] 2.2 在 `Completed` 中补充 O-024 完成摘要（并发解析、首个成功返回、后台缓存更新、context 取消与防挂死）
- [ ] 2.3 在 O-024 完成摘要中记录性能验收采用现有 `BenchmarkFirstPacket`，不新增专项 benchmark

## 3. 验证与记录

- [ ] 3.1 运行 `go test -v -run TestConcurrentNSResolution ./e2e/...`
- [ ] 3.2 运行 `go test -v -run TestConcurrentNSResolution_CachePopulation ./e2e/...`
- [ ] 3.3 运行 `go test -v -run 'TestResolveNSIPsConcurrentlyNoPanic|TestResolveNSIPsConcurrentlyContextCancelDoesNotHang|TestResolveNSIPsConcurrentlyEmptyInput' ./server/...`
- [ ] 3.4 运行 `go test -bench BenchmarkFirstPacket -benchmem ./e2e/...` 并记录结果可复核
