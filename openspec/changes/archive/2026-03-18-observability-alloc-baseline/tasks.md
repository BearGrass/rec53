## 1. 全量 benchmark 添加 `b.ReportAllocs()`

- [x] 1.1 `server/ip_pool_test.go` — 6 个 Benchmark 函数添加 `b.ReportAllocs()`
- [x] 1.2 `server/state_machine_bench_test.go` — 4 个 Benchmark 函数添加 `b.ReportAllocs()`
- [x] 1.3 `server/cache_bench_test.go` — 5 个 Benchmark 函数添加 `b.ReportAllocs()`（实际 5 个，无 BenchmarkCacheTTLExpiry）
- [x] 1.4 `e2e/first_packet_bench_test.go` — 5 个 Benchmark 函数添加 `b.ReportAllocs()`
- [x] 1.5 `e2e/error_test.go` — `BenchmarkIntegrationQuery` 添加 `b.ReportAllocs()`
- [x] 1.6 `monitor/metric_bench_test.go` — 4 个 Benchmark 函数添加 `b.ReportAllocs()`
- [x] 1.7 运行 benchmark 验证输出包含 allocs/op 列 ✓

## 2. `updatePercentiles` 固定数组微优化

- [x] 2.1 `server/ip_pool_quality_v2.go` — `updatePercentiles()` 改用 `[64]int32` 栈数组 + `slices.Sort`，已替换 `sort` 为 `slices` 导入
- [x] 2.2 验证 allocs/op：0 allocs/op（基线 3 → 0），ns/op 从 1480 降至 ~700 ✓
- [x] 2.3 `go test -race ./server/...` 无数据竞争 ✓

## 3. pprof 配置项

- [x] 3.1 `cmd/rec53.go` — `Config` 结构体新增 `Debug` 子结构，默认 `pprof_listen: "127.0.0.1:6060"`
- [x] 3.2 `generate-config.sh` — 添加 `debug:` 节注释示例

## 4. pprof HTTP server 实现

- [x] 4.1 `monitor/pprof.go` — 新增 `StartPprofServer(ctx, listenAddr)` 函数，独立 mux + graceful shutdown
- [x] 4.2 `cmd/rec53.go` — 当 `PprofEnabled == true` 时启动 pprof server，context 在 signal 时取消
- [x] 4.3 验证：`go build` 通过，代码逻辑与 metrics server 一致
- [x] 4.4 验证：默认 `PprofEnabled: false`，不启动 pprof

## 5. pprof 文档

- [x] 5.1 `README.md` — 新增 Profiling / pprof 配置示例和使用说明
- [x] 5.2 `README.zh.md` — 同步新增中文版 pprof 说明，内容与英文版一致

## 6. Cache COW 设计审计

- [x] 6.1 完成调用方审计：3 个生产调用方（2 MUTATING/别名，1 READ_ONLY），输出 `docs/cache-cow-audit.md`
- [x] 6.2 编写 Cache COW 设计草案：3 个方案（ReadOnlyMsg、Linter 契约、选择性移除）+ 风险评估
- [x] 6.3 定义实施硬门槛：pprof heap profile `Copy()` >30% 总分配，含验证步骤和决策模板

## 7. 收尾

- [x] 7.1 `go test -race ./...` 全量通过 ✓（5 packages, 0 failures）
- [x] 7.2 `go build -o rec53 ./cmd` 编译通过 ✓
- [x] 7.3 `docs/architecture.md` — 新增 Core Subsystem: Observability (monitor) 段落
- [x] 7.4 `.rec53/ROADMAP.md` — v0.4.0 全部任务标记完成 ✓
