## Why

`BenchmarkFirstPacketWithWarmup` 在 warmup 完成后调用 `FlushCacheForTest()`，清空了所有 DNS 缓存（包括 warmup 期间填入的 `.com`、`.net` 等 zone 缓存），导致 benchmark 测量的是"只有 IPPool 延迟数据、zone 缓存全空"的状态——这种状态在生产中不会出现。生产中 warmup 完成后，zone 缓存已被填充，首次查询新域名无需再查根服务器取 `.com` NS。因此 WithWarmup 场景仅比 NoWarmup 快约 54ms，而实际差距应为 300–500ms，benchmark 严重低估了 warmup 的价值。

## What Changes

- 修改 `BenchmarkFirstPacketWithWarmup`：warmup 完成后**不**清空 zone 缓存（删除 `FlushCacheForTest()` 调用），仅保留 IPPool 的 `ResetIPPoolForTest()` 逻辑，使每次迭代都在"zone 缓存已热、IPPool 已热"的状态下测量首次新域名查询耗时
- 新增 `BenchmarkFirstPacketIPPoolOnly`：专门测量"IPPool 已热、zone 缓存全空"场景（即当前 WithWarmup 的实际行为），保留该场景但命名更准确
- 修改 `BenchmarkFirstPacketComparison`：同步修复 WithWarmup 场景（删除 `FlushCacheForTest()`），新增 IPPoolOnly 列
- 更新顶部 package 文档注释，准确描述四个场景的语义
- 运行 benchmark，用实测数据更新 `README.md` 和 `README.zh.md`

## Capabilities

### New Capabilities

- `realistic-warmup-benchmark`：定义"warmup 后首包"benchmark 的语义规范——WithWarmup 场景应保留 warmup 副作用（zone 缓存），IPPoolOnly 场景显式说明其与生产状态的差异

### Modified Capabilities

- `core-benchmarks`：新增 e2e 首包 benchmark 场景语义要求（WithWarmup 不得 flush zone cache；IPPoolOnly 须明确文档其局限性）

## Impact

- `e2e/first_packet_bench_test.go`：修改 `BenchmarkFirstPacketWithWarmup`，新增 `BenchmarkFirstPacketIPPoolOnly`，修改 `BenchmarkFirstPacketComparison`
- `README.md` / `README.zh.md`：更新 benchmark 数据表格
- `openspec/specs/core-benchmarks/spec.md`：补充 e2e 首包场景语义要求
