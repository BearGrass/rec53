## Why

当前 26 个 benchmark 均未报告 alloc 指标，无法量化每次查询的内存分配；生产环境无 pprof 端点，无法定位实际热点。缺乏分配基线和可观测手段，任何"优化"都只能凭直觉——这正是 v0.4.0 初始 `sync.Pool` 方案被否决的根因。先解决可观测性问题，再用数据决定在哪里优化。

## What Changes

- 全部 26 个 benchmark 添加 `b.ReportAllocs()`，建立 allocs/op 和 B/op 基线
- 新增受控 pprof HTTP 端点：默认关闭，通过 `debug.pprof_enabled` 开启，仅监听 `127.0.0.1`，纳入现有服务生命周期（context 取消 + Shutdown），不泄漏 goroutine
- `updatePercentiles()` 微优化：将 `make([]int32, n)` + `sort.Slice` 替换为 `[64]int32` 栈数组 + `slices.Sort`，目标从 3 allocs/op 降至 0-1 allocs/op
- Cache COW 设计审计（仅文档）：审计全部 `getCacheCopy`/`getCacheCopyByType` 调用方的可变性，输出设计草案，实施硬门槛为 pprof 证明 `Copy()` 占总分配 >30%

## Capabilities

### New Capabilities
- `benchmark-alloc-baseline`: 全量 benchmark 分配报告——确保所有 benchmark 输出 allocs/op 和 B/op 指标
- `pprof-endpoint`: 受控 pprof HTTP 端点——默认关闭、仅本地访问、纳入服务生命周期管理
- `cache-cow-audit`: Cache COW 设计审计文档——调用方只读审计清单、不可变包装方案、实施门槛

### Modified Capabilities
- `core-benchmarks`: `updatePercentiles` 固定数组微优化改变了 allocs/op 基线预期

## Impact

- `server/ip_pool_test.go`, `server/state_machine_bench_test.go`, `server/cache_bench_test.go`, `e2e/first_packet_bench_test.go`, `e2e/error_test.go`, `monitor/metric_bench_test.go` — 添加 `b.ReportAllocs()`
- `server/ip_pool_quality_v2.go` — `updatePercentiles()` 改用固定数组
- `cmd/rec53.go` 或 `monitor/` — pprof 端点集成
- `config.yaml` schema — 新增 `debug.pprof_enabled` 和 `debug.pprof_listen` 配置项
- `README.md` / `README.zh.md` — pprof 使用说明
- 新增文档：Cache COW 设计草案（位置待定，可能在 `docs/` 或 openspec specs 中）
