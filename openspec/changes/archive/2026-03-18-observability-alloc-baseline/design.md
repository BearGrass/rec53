## Context

rec53 当前 26 个 benchmark 均未调用 `b.ReportAllocs()`，无法量化 allocs/op 和 B/op 基线；生产环境无 pprof 端点，无法在真实负载下定位分配热点。v0.4.0 最初规划为 `sync.Pool` 优化 `dns.Msg`，但评估发现：`dns.Msg` 生命周期跨越多个状态机步骤和 goroutine（Happy Eyeballs 竞速路径），归还时机不可追踪，use-after-free 风险远大于收益。方向调整为"先可观测，再优化"。

已知的低风险微优化点：`updatePercentiles()` 使用 `make([]int32, n)` + `sort.Slice`，ring buffer 上限固定 64 元素（256 bytes），可用栈数组实现零分配。

Cache 层所有 3 个生产调用方（`state_cache_lookup.go:37`, `state_lookup_ns_cache.go:38`, `state_query_upstream.go:194`）均为只读使用，防御性 `Copy()` 理论上可消除，但需设计不可变包装保证全局安全——本期仅审计并输出设计草案。

## Goals / Non-Goals

**Goals:**
- 全部 26 个 benchmark 报告 allocs/op 和 B/op，建立量化基线
- 提供受控 pprof HTTP 端点，支持 `go tool pprof` 在真实负载下采集 heap/cpu/goroutine profile
- pprof 端点默认关闭、仅监听本地、纳入服务生命周期
- `updatePercentiles()` 改用固定栈数组，消除热路径堆分配
- 输出 Cache COW 设计草案，含调用方审计清单和实施门槛

**Non-Goals:**
- `sync.Pool` 池化 `dns.Msg` 或任何对象——已评估否决
- Cache COW 代码实施——本期仅文档，实施需 pprof 数据验证
- 性能回归阈值调整——现有 benchmark 阈值不变
- pprof 的鉴权或 HTTPS——仅监听 `127.0.0.1`，SSH 隧道访问
- 新增 Prometheus 指标——pprof 已提供更细粒度的分配数据

## Decisions

### 1. `b.ReportAllocs()` 逐函数添加，不使用全局 flag

**选择**：在每个 `Benchmark*` 函数体第一行添加 `b.ReportAllocs()`。

**替代方案**：运行时传递 `-benchmem` flag。

**理由**：`b.ReportAllocs()` 内嵌到函数中，确保任何人跑 benchmark 都能看到分配指标，不依赖调用者记住传 flag。代码意图更明确：该 benchmark 关心分配。

### 2. pprof 端点使用独立 `http.Server`，不复用 metrics server

**选择**：在 `monitor/` 包新增 `StartPprofServer(ctx, listenAddr)` 函数，创建独立 `http.Server`，注册 `net/http/pprof` 默认 handler。由 `cmd/rec53.go` 在配置启用时调用，传入主 context。

**替代方案 A**：复用 Prometheus metrics HTTP server 的 mux。

**替代方案 B**：在 `cmd/rec53.go` 中内联启动。

**理由**：metrics server 使用 `promhttp.Handler()` 作为默认 handler，pprof 需要注册到 `DefaultServeMux` 或独立 mux。混入同一 server 会让 pprof 路径暴露在 metrics 端口上（可能面向外网）。独立 server 允许 pprof 监听不同地址（默认 `127.0.0.1:6060`），与 metrics server 的生命周期解耦。放在 `monitor/` 包保持职责归属一致。

### 3. pprof 配置项设计

**选择**：在 `config.yaml` 新增：
```yaml
debug:
  pprof_enabled: false    # 默认关闭
  pprof_listen: "127.0.0.1:6060"  # 默认仅本地
```

`Config` 结构体新增 `Debug` 子结构。未配置时 `pprof_enabled` 默认 `false`，`pprof_listen` 默认 `"127.0.0.1:6060"`。

**理由**：默认关闭消除生产误开风险。绑定 `127.0.0.1` 防止公网暴露。端口 `6060` 是 Go pprof 社区惯例。子结构 `debug` 为未来调试功能预留命名空间。

### 4. pprof server 生命周期管理

**选择**：`StartPprofServer` 接收 `context.Context`，在 goroutine 中 `ListenAndServe`，监听 context 取消后调用 `Shutdown(shutdownCtx)` 优雅退出。调用方（`cmd/rec53.go`）的 `WaitGroup` 追踪该 goroutine。

**理由**：与现有 DNS server 和 metrics server 的 shutdown 模式一致。`Shutdown` 而非 `Close` 允许排空活跃的 pprof 采集请求。

### 5. `updatePercentiles` 固定数组替代动态 slice

**选择**：将 `make([]int32, n)` + `sort.Slice` 替换为 `var buf [64]int32`，取 `buf[:n]` 子切片 + `slices.Sort(buf[:n])`。

**替代方案**：`sync.Pool` 池化 `[]int32`。

**理由**：ring buffer `maxSize` 硬编码为 64（常量 `RING_BUFFER_SIZE`），256 bytes 在栈上分配零成本。`sync.Pool` 的 Get/Put 开销（~50ns）在该场景下反而是负优化。`slices.Sort` 是 Go 1.21 标准库泛型排序，无 interface 装箱开销。

### 6. Cache COW 审计输出为 openspec spec 文档

**选择**：审计结果写入 `specs/cache-cow-audit/spec.md`，包含调用方审计清单、不可变包装方案草案、实施门槛定义。

**理由**：作为 openspec 管理的 spec，与代码变更同样可追踪、可归档。实施门槛（pprof 证明 `Copy()` >30% 总分配）作为正式需求记录，防止未来在无数据支撑时盲目实施。

## Risks / Trade-offs

- **[pprof 端点误开导致信息泄露]** → 默认关闭 + 仅绑定 `127.0.0.1`。即使误开，攻击者需先获得服务器 SSH 访问权限。文档明确标注"勿绑定 `0.0.0.0`"。

- **[`b.ReportAllocs()` 增加 benchmark 输出噪声]** → 可接受。alloc 指标是标准 `go test -bench` 输出的一部分，不影响性能测量精度。

- **[固定 `[64]int32` 数组在 `maxSize` 变化时需同步更新]** → `maxSize` 当前是编译期常量 `RING_BUFFER_SIZE = 64`，改为其他值需同步更新数组大小。添加编译期断言 `_ [1]struct{} = [1]struct{}{}` 或文档注释提醒。

- **[Cache COW 审计仅产出文档、不产出代码]** → 符合"先可观测再优化"原则。代码实施需 pprof 数据验证，防止过早优化。
