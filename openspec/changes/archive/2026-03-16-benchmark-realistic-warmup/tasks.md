## 1. 修改 BenchmarkFirstPacketWithWarmup

- [ ] 1.1 删除 `BenchmarkFirstPacketWithWarmup` 循环体内的 `server.FlushCacheForTest()` 调用
- [ ] 1.2 更新 `BenchmarkFirstPacketWithWarmup` 的 godoc 注释，准确描述"IPPool 热 + zone 缓存热"的生产真实场景

## 2. 新增 BenchmarkFirstPacketIPPoolOnly

- [ ] 2.1 在 `BenchmarkFirstPacketWithWarmup` 之后新增 `BenchmarkFirstPacketIPPoolOnly` 函数，逻辑与原 WithWarmup 相同（warmup 后调用 `FlushCacheForTest()`）
- [ ] 2.2 为 `BenchmarkFirstPacketIPPoolOnly` 添加 godoc 注释，说明此场景在生产中不存在，仅用于量化 IPPool 延迟数据对 NS 选择的单独贡献

## 3. 修改 BenchmarkFirstPacketComparison

- [ ] 3.1 在 Comparison 中修复 Scenario 2（删除 `FlushCacheForTest()`，使其与新 WithWarmup 语义一致）
- [ ] 3.2 在 Comparison 中新增 Scenario 2.5（IPPoolOnly：warmup 后 flush zone cache）
- [ ] 3.3 更新 Comparison 的输出表格头，从三列改为四列（新增 `ippool-only` 列）

## 4. 更新 package 文档注释

- [ ] 4.1 更新文件顶部 `// BenchmarkFirstPacket` package 注释，将场景列表从三条改为四条，准确描述每个场景的前置状态

## 5. 验证

- [ ] 5.1 运行 `go test -race -timeout 120s ./e2e/...` 确认无编译错误和测试失败
- [ ] 5.2 运行 `go test -v -run='^$' -bench='BenchmarkFirstPacketNoWarmup|BenchmarkFirstPacketIPPoolOnly|BenchmarkFirstPacketWithWarmup|BenchmarkFirstPacketCacheHit' -benchtime=3x -timeout=300s ./e2e/...` 并记录实测数据
- [ ] 5.3 确认 `BenchmarkFirstPacketWithWarmup/www.qq.com` 的 ms/query 低于 500ms

## 6. 更新 README

- [ ] 6.1 更新 `README.md` 中的 benchmark 数据表格（新增 IPPoolOnly 行，更新 WithWarmup 数据）
- [ ] 6.2 更新 `README.zh.md` 中对应的 benchmark 数据表格
