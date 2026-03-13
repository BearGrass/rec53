## Context

`e2e/first_packet_bench_test.go` 中 `BenchmarkFirstPacketWithWarmup` 的实现在 warmup 完成后调用 `server.FlushCacheForTest()`，清空了所有 DNS 缓存——包括 warmup 期间通过迭代解析 `.com`/`.net` 等 TLD 域名所填入的 zone 缓存条目。

实测对比：

| 场景 | 测量状态 | www.qq.com 耗时 |
|------|---------|----------------|
| WithWarmup（当前，flush 后） | IPPool 热 + zone 缓存**空** | ~754 ms |
| 真实生产（warmup 后无 flush） | IPPool 热 + zone 缓存**热** | ~268 ms |
| NoWarmup | IPPool 冷 + zone 缓存空 | ~808 ms |

当前 WithWarmup 与 NoWarmup 仅差 ~54ms（仅来自 IPPool 能选快速根服务器），而生产中 warmup 带来的实际加速约 540ms——benchmark 严重低估了 warmup 的价值。

`BenchmarkFirstPacketComparison` 存在同样的问题，其 Scenario 2（WithWarmup）也调用了 `FlushCacheForTest()`。

## Goals / Non-Goals

**Goals:**
- 修复 `BenchmarkFirstPacketWithWarmup`，使其测量"IPPool 热 + zone 缓存热"的生产真实状态
- 保留"IPPool 热 + zone 缓存冷"场景，但重命名为 `BenchmarkFirstPacketIPPoolOnly` 并准确说明其局限性
- 同步修复 `BenchmarkFirstPacketComparison`，新增 IPPoolOnly 列
- 更新 README benchmark 数据

**Non-Goals:**
- 不修改 warmup 本身的逻辑（`server/warmup.go`）
- 不引入新的外部依赖
- 不修改其他 benchmark（`NoWarmup`、`CacheHit`）

## Decisions

### 决策 1：WithWarmup 迭代循环中不再 flush zone cache，仅 reset IPPool

**选择**：每次迭代开头调用 `server.ResetIPPoolForTest()`（同现在），但**不**调用 `FlushCacheForTest()`。

**理由**：
- 生产中 warmup 完成后，zone 缓存是持续存在的；只有 IPPool 需要每次重置以确保每个迭代都重新经历 warmup 过程
- 保留 zone 缓存意味着每次迭代开始时状态接近生产：IPPool 从空开始 warmup，warmup 同时填充 zone 缓存，warmup 结束后发起首次新域名查询

**替代方案考虑**：
- 每次迭代同时 flush zone cache → 这正是当前的错误行为，排除
- 完全不 reset IPPool → 会导致不同迭代间 IPPool 状态叠加，结果不可重现，排除

### 决策 2：新增 `BenchmarkFirstPacketIPPoolOnly` 保留当前语义

**选择**：将当前 WithWarmup 的行为（warmup 后 flush zone cache）提取为独立 benchmark `BenchmarkFirstPacketIPPoolOnly`，加注释说明其场景："仅 IPPool 有延迟数据，zone 缓存为空——这是一种理论场景，生产中不存在"。

**理由**：
- 该场景可以量化 IPPool 单独带来的加速（约 54ms），有学术价值
- 直接删除会损失信息；重命名+文档比删除更好

### 决策 3：`BenchmarkFirstPacketComparison` 同步修复，新增 IPPoolOnly 列

**选择**：
- Scenario 2（WithWarmup）删除 `FlushCacheForTest()`
- 在 WithWarmup 和 CacheHit 之间新增 Scenario 2.5（IPPoolOnly）
- 更新输出表格头

**理由**：Comparison benchmark 是一次性快速报告工具，四列对比才完整体现 warmup 的每个组成部分。

### 决策 4：每次迭代内的操作顺序

**WithWarmup 的新操作顺序**：
```
ResetIPPoolForTest()          // 重置 IPPool，确保本次迭代重新经历 warmup
NewServerWithConfig(warmupCfg) // 创建服务器（warmup enabled）
s.Run()                       // 启动服务器，warmup 后台运行
time.Sleep(warmupCfg.Duration + 2s)  // 等待 warmup 完成
// 不调用 FlushCacheForTest()  ← 关键改动
b.ResetTimer()
fpQueryOnce(...)              // 测量首次新域名查询
b.StopTimer()
```

**注意**：不调用 `FlushCacheForTest()` 意味着 zone 缓存在迭代间会**积累**。为避免第二次迭代的 zone 缓存比第一次更热，每次 `ResetIPPoolForTest()` 后创建新服务器，且 warmup 会重新填充 zone 缓存——缓存是 package-level global，跨迭代共享。

这实际上使第二次及后续迭代的 zone 缓存"更热"，但这也更接近生产实际（生产中 zone 缓存持续积累）。benchmark 数据应该在多次运行中趋于稳定。

## Risks / Trade-offs

- **[风险] 迭代间 zone 缓存积累导致后续迭代更快** → 缓解：benchtime 设置较小（如 3x）时影响有限；README 注释说明此行为
- **[风险] 测试域名的 TTL 到期导致迭代间结果差异** → 缓解：warmup 使用的是 TLD 级域名（TTL 48h），不受影响
- **[权衡] WithWarmup 每次迭代耗时从 ~10s 增加到 ~10s（无变化，warmup duration 不变）** → 无影响

## Migration Plan

1. 修改 `e2e/first_packet_bench_test.go`
2. 运行 benchmark 确认 WithWarmup 结果接近生产实测（~268ms for www.qq.com）
3. 更新 `README.md` 和 `README.zh.md` 中的 benchmark 数据表格

无回滚风险（仅测试文件改动，不影响生产代码）。
