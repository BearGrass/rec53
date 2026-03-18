# Roadmap

## Version History

| Version | Date     | Highlights                                                                        |
|---------|----------|-----------------------------------------------------------------------------------|
| v0.2.0  | 2026-03  | 全量缓存快照——重启后所有域名首次查询即缓存命中                                    |
| v0.4.1  | planned  | SO_REUSEPORT 多 Listener——突破单 socket 吞吐上限                                  |
| v0.6.0  | 2026-03  | XDP Cache 核心——eBPF 程序、Go loader、cache sync、XDP_TX 响应                     |
| v0.6.1  | planned  | XDP 生产就绪——Prometheus 指标、TTL 清理、性能验收、文档                            |
| dev     | 2026-03  | NS 缓存快照持久化，消除重启冷启动延迟                                             |
| dev     | 2026-03  | 并发 NS 解析（O-024），最多 5 worker，首个成功即返回                               |
| dev     | 2026-03  | Happy Eyeballs 并发上游查询，先到先得                                              |
| dev     | 2026-03  | IPQualityV2 — 滑动窗口环形缓冲 + P50/P95/P99 + 4 状态健康模型                     |
| dev     | 2026-03  | 负缓存（NXDOMAIN/NODATA）+ Bad Rcode 重试（SERVFAIL/REFUSED 换服务器）             |
| dev     | 2026-03  | NS 预热 — 30 个高流量 TLD，CPU 感知动态并发                                       |
| dev     | 2026-03  | 状态机重构 — 单文件拆分、baseState 嵌入、语义命名                                  |
| dev     | 2026-03  | Hosts 本地权威、Forwarding 转发规则、rec53ctl 运维脚本、/dev/stderr 日志           |
| dev     | 2026-03  | Graceful shutdown、综合测试套件、E2E 测试框架                                      |
| dev     | 2026-03  | IP 质量追踪 + 预取、Prometheus 指标、Docker Compose 部署                           |

## Current Version: dev

### 核心功能

- 从根服务器开始的完整递归 DNS 解析
- UDP/TCP 双协议支持
- LRU 缓存 + TTL 过期（默认 5 分钟）
- CNAME 链追踪与循环检测（MaxIterations=50）
- Glue 记录处理
- EDNS0 支持（4096 字节缓冲区）
- UDP 截断响应（TC 标志）
- 优雅关闭（5 秒超时）

### IP 质量与上游选择

- **IPQualityV2** — 64 样本滑动窗口环形缓冲区，P50/P95/P99 百分位延迟
- **4 状态健康模型** — ACTIVE → DEGRADED → SUSPECT → RECOVERED，指数退避 + 30s 后台探测
- **Happy Eyeballs** — 并发向最优 + 次优 IP 发送查询，先到先得（超时默认 1.5s）
- **IP 预取** — 候选服务器预取加速后续查询
- **Bad Rcode 重试** — SERVFAIL/REFUSED 时标记 bad server，自动切换备用 IP

### 缓存与预热

- **负缓存** — NXDOMAIN/NODATA 检测与缓存（基于 SOA Minttl，默认回退 60s）
- **NS 预热** — 启动时查询 30 个高流量 TLD 的 NS 记录，CPU 感知动态并发
- **缓存快照持久化** — 关闭时保存全量 DNS 缓存到 JSON 文件，启动时恢复未过期条目

### 本地策略

- **Hosts 本地权威** — A/AAAA/CNAME 静态记录，AA=true 权威应答，优先于缓存和上游
- **Forwarding 转发规则** — 最长后缀匹配转发到指定上游 DNS，结果不写缓存

### 运维与可观测性

- **rec53ctl** — 单入口运维脚本：`build` / `install` / `upgrade` / `uninstall` / `run`
- **Prometheus 指标端点** — 查询计数、延迟、缓存命中率、IP 池状态
- **Docker Compose 部署** — 一键容器化部署
- **并发 NS 解析** — 最多 5 个 worker 并发解析 NS IP，首个成功即返回

---

## ~~v0.1.0~~ ✅ 已完成 — Hosts 本地权威 + Forwarding 转发规则

**完成于**：2026-03

- `server/state_hosts.go` — `HOSTS_LOOKUP` 状态，支持 A / AAAA / CNAME，AA=true 权威应答
- `server/state_forward.go` — `FORWARDING_CHECK` 状态，最长后缀匹配，结果不写缓存
- 原子快照替换（`atomic.Pointer`）消除 hosts/forward 配置读写数据竞争
- 配置格式：`hosts:` 和 `forwarding:` 块，支持精确域名匹配

---

## ~~v0.5.0~~ ✅ 已完成 — rec53ctl 运维脚本

**完成于**：2026-03

- `rec53ctl`（项目根目录）：单入口 bash 脚本，子命令覆盖完整运维生命周期
- `build`：go 工具链检查 + `dist/rec53` 编译，支持 `GOOS`/`GOARCH` 交叉编译
- `install`：build → 复制二进制 → 处理配置（`--force-config` 强制覆盖）→ 写 systemd unit → enable/start
- `upgrade`：备份旧二进制 → build（`SKIP_BUILD=1` 可跳过）→ 热替换 → 启动失败自动回滚
- `uninstall`：stop/disable → 删 unit/二进制/config.yaml → 保留非空 CONFIG_DIR
- `run`：前台启动，`-rec53.log /dev/stderr` 日志输出到终端，`exec` 确保信号直传
- `monitor/log.go` 修复：检测 `/dev/stderr`/`/dev/stdout` 时直接写对应 fd，跳过 lumberjack 轮转

---

## ~~F-003~~ ✅ 已完成 — IPQualityV2 + Happy Eyeballs

**完成于**：2026-03-13

- **IPQualityV2** — 全面重写 IP 质量追踪系统
  - 64 样本滑动窗口环形缓冲区，替代简单均值
  - P50/P95/P99 百分位延迟计算，导出到 Prometheus
  - 复合评分 + 4 状态健康模型（ACTIVE → DEGRADED → SUSPECT → RECOVERED）
  - 指数退避 + 30 秒后台探测循环
  - 线程安全：原子操作（无锁快速路径）
- **Happy Eyeballs 并发查询** — 同时向最优 + 次优 IP 发送查询，先到先得，超时可配置（默认 1.5s）

---

## ~~B-012~~ ✅ 已完成 — 负缓存 + Bad Rcode 重试

**完成于**：2026-03-12

- **负缓存（B-012）** — NXDOMAIN/NODATA 检测和缓存，基于 SOA Minttl，默认回退 60 秒
- **Bad Rcode 重试（B-013）** — SERVFAIL/REFUSED 时标记 bad server 并尝试备用 IP

---

## ~~O-024~~ ✅ 已完成 — 并发 NS 解析

**完成于**：2026-03-16

- 最多 5 个并发 worker 解析 NS IP，首个成功立即返回
- 后台更新缓存，减少后续查询延迟
- NS 递归解析深度限制，防止栈溢出

---

## ~~O-025/026/027~~ ✅ 已完成 — NS 预热优化

**完成于**：2026-03-12

- 启动时查询 30 个高流量 TLD 的 NS 记录（Round 1 之后）
- CPU 感知动态并发控制
- 精选 TLD 列表，优化内存使用

---

## ✅ 已完成 — 全量缓存快照（原 NS 缓存快照 → v0.2.0）

**完成于**：2026-03-17

- 关闭时保存全量 DNS 缓存条目（A/AAAA 答案、CNAME、NS 委派、负缓存等）到 JSON 文件
- 启动时恢复未过期条目，消除冷启动延迟
- `remainingTTL` 覆盖 Answer + Ns + Extra 三个 section
- 配置项：`snapshot.enabled` + `snapshot.file`
- v0.1.0 仅保存 NS 委派；v0.2.0 扩展为全量缓存，~20 行代码改动

---

## ✅ 已完成 — 状态机重构

**完成于**：2026-03-13

- 状态文件拆分为单文件（每个状态独立源文件）
- 语义命名（`state_hosts.go`、`state_forward.go`、`state_check_resp.go` 等）
- `baseState` 嵌入，消除状态结构体重复字段
- 辅助函数提取，提升可读性

---

## ~~v0.2.0~~ ✅ 已完成 — 全量缓存快照

**完成于**：2026-03-17

**目标**：快照从仅保存 NS 委派扩展到保存全部缓存条目，重启后所有域名首次查询即缓存命中。

> **背景**：v0.1.0 快照仅保存 NS 委派条目，恢复后每个域名的最终 A/AAAA 答案仍需 1-2 次上游往返。
> 对于单机生产部署（应用 + Docker + 爬虫），重启后爬虫批量请求数千域名时，
> 全部缓存 miss 会导致前几分钟吞吐量下降。全量快照可让爬虫重启后第一秒即全速运行。
>
> 原「学习型预热 Round 2」方案（衰减 LFU + `publicsuffix` + ~500 行代码）已废弃，
> NS 缓存快照持久化已覆盖其 80-90% 的价值，剩余增量由本改动以 ~20 行代码完成。

### 实际改动

- `server/snapshot.go` — 移除"必须含 NS RR"的过滤条件，保存所有缓存条目
- `server/snapshot.go` — `remainingTTL` 扩展覆盖 Answer + Ns + Extra 三个 section
- 单元测试 16 个（含 A/AAAA、CNAME、混合 section、corrupt base64、目录自动创建等）
- e2e 测试 4 个（A 记录存活重启、NS 委派存活重启、过期条目跳过、disabled no-op）

---

## v0.3.0 — IP 池 Stale Prune ✅

**目标**：防止 `globalIPPool` 无限增长，清理长期未被真实查询引用的 IP 条目。

### 实现

- `IPQualityV2` 新增 `lastSeen time.Time` 字段，`RecordLatency` / `RecordFailure` 时更新（`sync.RWMutex` 保护）
- `PruneStaleIPs(threshold)` 方法：写锁遍历 IP 池，删除 `lastSeen` 超过阈值且不在豁免集合中的条目
- 集成到 `periodicProbeLoop`，每 30 分钟基于 wall-clock 比较触发一次 prune
- **根服务器豁免**：`utils.ExtractRootIPs()` 提取 13 组根服务器 IP，在 `StartProbeLoop` 时传入，永不被 prune
- 阈值 `STALE_IP_THRESHOLD = 24h`（包级常量）— 24h 而非最初设计的 2h，因为 `lastSeen` 仅在迭代解析时更新（不含缓存命中），短阈值会误删合法 IP

### 变更文件

- `server/ip_pool_quality_v2.go` — `lastSeen` 字段、`GetLastSeen()` 方法
- `server/ip_pool.go` — `exemptIPs`/`lastPruneAt` 字段、`PruneStaleIPs()` 方法、常量、`StartProbeLoop` 签名变更
- `server/server.go` — 调用方适配 `StartProbeLoop(utils.ExtractRootIPs())`
- `utils/root.go` — `ExtractRootIPs()` 函数
- `server/ip_pool_prune_test.go` — 9 个单元测试
- `utils/root_test.go` — `TestExtractRootIPs` 测试
- `docs/architecture.md` — IP Pool 段落更新

---

## v0.4.0 — 可观测性 + 分配基线 ✅

**目标**：建立内存分配的量化基线和生产可观测能力，为后续性能优化提供数据驱动的决策依据。

### 背景

v0.4.0 最初规划为 `sync.Pool` 内存优化，经评估后调整方向（见下方"已评估方案"）。
当前 26 个 benchmark 均未报告 alloc 指标，缺少分配基线；生产环境无 pprof 端点，无法定位实际热点。
先解决可观测性问题，再用数据决定是否以及在哪里优化。

### 任务清单

**1. 全量 benchmark 加 `b.ReportAllocs()`（建立 alloc baseline）**

- [x] `server/ip_pool_test.go` — 6 个 benchmark 加 `b.ReportAllocs()`
- [x] `server/state_machine_bench_test.go` — 4 个 benchmark 加 `b.ReportAllocs()`
- [x] `server/cache_bench_test.go` — 5 个 benchmark 加 `b.ReportAllocs()`
- [x] `e2e/first_packet_bench_test.go` — 5 个 benchmark 加 `b.ReportAllocs()`
- [x] `e2e/error_test.go` — `BenchmarkIntegrationQuery` 加 `b.ReportAllocs()`
- [x] `monitor/metric_bench_test.go` — 4 个 benchmark 加 `b.ReportAllocs()`

**2. 接入受控 pprof（用真实负载定位热点）**

- [x] `monitor/pprof.go` — 新增独立 pprof HTTP 端点
- [x] 默认关闭，通过配置项 `debug.pprof_enabled: true` 开启
- [x] 开启时默认仅监听 `127.0.0.1:6060`，防止暴露在公网
- [x] pprof HTTP server 纳入服务生命周期（context 取消 + Shutdown 优雅退出）
- [x] `README.md` / `README.zh.md` 同步更新 pprof 使用说明

**3. `updatePercentiles` 固定数组微优化（低风险 quick win）**

- [x] `server/ip_pool_quality_v2.go` — `updatePercentiles()` 改用 `[64]int32` 栈数组 + `slices.Sort`
- [x] 验证结果：allocs/op 从 3 降至 0，ns/op 从 1480 降至 ~700

**4. Cache COW 设计与调用方只读审计（仅文档，不实施）**

- [x] 完成 `getCacheCopy` / `getCacheCopyByType` 全部调用方的可变性审计清单 → `docs/cache-cow-audit.md`
- [x] 输出 Cache COW 设计草案，包含：3 个方案（ReadOnlyMsg、Linter 契约、选择性移除）+ 风险评估
- [x] 实施硬门槛：仅当 pprof 证明 cache `Copy()` 占总分配 >30% 时才进入代码阶段

### 已评估、暂不实施的方案

**`sync.Pool` 池化 `dns.Msg`**

评估结论：`dns.Msg` 生命周期跨越多个状态机步骤和 goroutine（特别是 Happy Eyeballs 竞速路径），
归还时机难以追踪，易引入 use-after-free 和复用污染。当前部署规模（单机本地递归解析器，<1000 QPS）
下 GC 不构成瓶颈。维护风险远大于性能收益。

**`sync.Pool` 池化 `updatePercentiles` 临时 slice**

评估结论：ring buffer 上限固定 64 元素（256 bytes），用 `[64]int32` 栈数组即可实现零分配，
无需引入 `sync.Pool` 的 Get/Put 开销和并发复杂度。

---

## v0.4.1 — SO_REUSEPORT 多 Listener

**目标**：通过 `SO_REUSEPORT` 多 socket 监听突破单 socket 吞吐上限（当前 ~91-95 K QPS），实现随 CPU 核心数线性扩展。

### 背景

并发扩展压测（`docs/benchmarks.md` "Concurrency Scaling" 章节）证实：

- 吞吐在 c=32 饱和于 ~91 K QPS，增加并发仅增延迟不增吞吐
- pprof 显示 `syscall.Syscall6`（`recvfrom`/`sendto`）占 ~25% CPU
- 瓶颈为 **单 UDP socket 的内核读写串行化**，非 CPU 计算
- 提升 CPU 频率仅预期 +15-25%，提核心数在当前架构下无效

`miekg/dns` v1.1.52 原生支持 `dns.Server.ReusePort bool`，Linux 上通过 `unix.SO_REUSEPORT` setsockopt 实现，内核在多个 socket 间负载均衡。

### 预期收益（4C8T, listeners=4）

| 估计 | QPS | 倍率 |
|------|-----|------|
| 保守 | 150–200 K | 1.6–2.2× |
| 乐观 | 250–300 K | 2.7–3.3× |

延迟 P50/P99 应下降（每个 socket 排队深度更浅）。

### 任务清单

**1. server 结构体改造**

- [ ] `server/server.go` — `udpSrv/tcpSrv *dns.Server` → `udpSrvs/tcpSrvs []*dns.Server` + `listeners int` 字段
- [ ] `Run()` — 循环创建 N 对 listener，设置 `ReusePort: n > 1`，`sync.Once` 保护 ready channel
- [ ] `Shutdown()` — 循环关闭 N 个 server

**2. 配置接入**

- [ ] `cmd/rec53.go` — `DNSConfig` 新增 `Listeners int` 字段 + 校验
- [ ] `NewServerWithFullConfig` 签名增加 `listeners` 参数
- [ ] `config.yaml` + `generate-config.sh` — 添加 `listeners:` 注释说明

**3. 验证**

- [ ] 现有测试通过（`NewServer()` 默认 listeners=1，行为不变）
- [ ] `go test -race ./...` 无竞争
- [ ] dnsperf 对比压测：listeners=1 vs listeners=4，记录结果到 `docs/benchmarks.md`

**4. 文档**

- [ ] `README.md` / `README.zh.md` — 新增 SO_REUSEPORT 配置说明
- [ ] `docs/benchmarks.md` — 补充多 listener 压测对比数据
- [ ] `docs/architecture.md` — 更新 server 层描述

### 不需要改动的代码

- `ServeDNS` handler — 不变
- `globalDnsCache` / `globalIPPool` — 已有 RWMutex/atomic 保护，多 listener 无额外竞争
- State Machine / Metrics — 不变
- 所有现有测试 — 不变（默认退化为单 listener）

### 开发成本

~75 行代码改动，3 个文件（`server.go`、`cmd/rec53.go`、`config.yaml`），预计 1 小时完成。

---

## v0.5.0 — 热路径降分配优化（基于 pprof）

**目标**：在不引入高风险生命周期复杂度（例如 `sync.Pool(dns.Msg)`）的前提下，优先降低已确认热路径的分配开销。

**基线文档约定**：`docs/benchmarks.md` 中的 `Profiling Findings (2026-03, dnsperf + pprof)` 作为
v0.5.0 后续优化的统一基线文档；所有优化项应使用同口径命令复采并与该基线做前后对比。

### pprof 基线（dnsperf + 去噪）

- 压测工具：`tools/dnsperf`（UDP，`c=128`，稳定约 `~100k QPS`）
- 去噪 `alloc_space`（focus=`rec53/server|github.com/miekg/dns`）显示：
  - `getCacheCopy/getCacheCopyByType` 累计约 **26-27%**
  - `dns.Msg.Copy/CopyTo` 累计约 **25%**
  - 指标上报（`InCounterAdd/OutCounterAdd/LatencyHistogramObserve`）累计约 **24%**
- 去噪 `cpu`（同 focus）显示热点集中在 `ServeDNS -> Change -> cacheLookup` 路径

### 任务清单（按优先级）

**1. 指标上报路径去分配（高收益、低风险）**

- [ ] `monitor/metric.go` — `With(prometheus.Labels{...})` 改为 `WithLabelValues(...)`，消除每次 map 分配
- [ ] 指标维度收敛：移除或降维高基数 `name` label（至少不直接使用原始域名）
- [ ] 更新 `README.md` / `README.zh.md` 的指标说明与兼容性变更

**2. Cache COW POC（高收益）**

- [ ] 基于 `docs/cache-cow-audit.md` 实现最小 POC：cache 读路径可选零拷贝，写路径保持 copy-on-write 语义
- [ ] 新增防御性测试：验证读路径不会污染缓存条目，验证并发下无 data race（`-race`）
- [ ] 对比基准：`BenchmarkCacheGetHit` / `BenchmarkStateMachineCacheHit` 的 allocs/op 与 ns/op

**3. `getCacheKey` 热路径微优化（中收益、极低风险）**

- [ ] `server/cache.go` — 用字符串拼接 + `strconv` 替换 `fmt.Sprintf("%s:%d", ...)`
- [ ] 验证 `BenchmarkCacheKey` 与 `BenchmarkCacheGetHit` 的回归阈值

**4. 二次 pprof 验证（收敛闭环）**

- [ ] 使用 `tools/dnsperf`（`c=128`）重跑去噪 profile
- [ ] 目标：热点占比下降并记录到 `docs/benchmarks.md`（含命令、环境、对比表）
- [ ] 若 Cache COW 收益不足或风险过高，回退为“仅保留指标与 key 优化”

### 不做事项（维持 v0.4.0 结论）

- `sync.Pool(dns.Msg)`：生命周期跨状态机和并发路径，归还时机复杂，维护风险高于收益

---

## v0.6.0 — XDP Cache 核心（eBPF 程序 + Go loader + cache sync）

**目标**：实现完整的 XDP/eBPF DNS cache 快速路径——cache hit 由 XDP 直接返回（`XDP_TX`），miss 透传到 Go。

**设计文档**：`docs/superpowers/specs/2026-03-18-xdp-dns-cache-design.md`

### 背景

CPU profiling 显示 172K QPS 下 22.5% CPU 耗在 `sendmsg` 系统调用、29% 在 GC/内存分配、8% 在 goroutine 栈增长。
对于 cache hit（生产环境主要流量），整个 Go 运行时开销不必要——答案已知，只需原样返回。
XDP 在网卡驱动层拦截 DNS 查询，cache hit 完全在内核空间完成，零系统调用、零内存拷贝、零 goroutine。

### 任务清单

**1. 构建系统 + 骨架**

- [ ] `server/xdp/dns_cache.h` — 共享结构体定义（`cache_key`、`cache_value`、stats indices）
- [ ] `server/xdp/Makefile` — clang `-target bpf` 编译规则
- [ ] 集成 cilium/ebpf `bpf2go`，配置 `//go:generate` 嵌入编译后的 eBPF 对象到 Go binary
- [ ] `go.mod` 添加 `github.com/cilium/ebpf` 依赖
- [ ] **Checkpoint：** 骨架 eBPF 程序（全 `XDP_PASS`）能加载到 lo，`bpftool prog show` 可见

**2. 完整 eBPF 程序**

- [ ] `server/xdp/dns_cache.c` — 解析 ETH/IP/UDP/DNS header，识别 DNS 端口（53）
- [ ] `extract_qname()` — bounded loop qname 提取 + inline lowercase（kernel 5.3+ 原生 bounded loop，不用 `#pragma unroll`）
- [ ] `QDCOUNT == 1` 前置检查，非标准查询直接 `XDP_PASS`
- [ ] `bpf_map_lookup_elem` cache 查找 + `expire_ts` 过期检查（`bpf_ktime_get_ns() / 1e9`）
- [ ] 响应构建：swap ETH/IP/UDP header → `bpf_xdp_adjust_tail()` 调整包大小 → 重新验证指针 → memcpy 预序列化响应 → patch Transaction ID → IP TTL=64 → IP checksum → UDP checksum=0
- [ ] `resp_len` bounds check（`<= MAX_DNS_RESPONSE_LEN`）满足 verifier
- [ ] BPF maps：`cache_map`（`BPF_MAP_TYPE_HASH`，65536 entries）、`xdp_stats`（`BPF_MAP_TYPE_PERCPU_ARRAY`）
- [ ] hit/miss/pass/error 计数器更新

**3. Go Loader**

- [ ] `server/xdp_loader.go` — 加载 eBPF 对象、attach XDP 到指定 interface、auto-detect native→generic fallback
- [ ] 生命周期管理：context 取消时 detach XDP、关闭 BPF objects
- [ ] 启动日志：输出实际 attach mode（native/generic）、interface 名称

**4. Cache 同步（Go → BPF map）**

- [ ] `server/xdp_sync.go` — `setCacheCopy()` 调用后即时写入 BPF map
- [ ] Presentation→wire format 域名转换 + `strings.ToLower()` 归一化
- [ ] Monotonic clock：`unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)` 计算 `expire_ts`（不用 `time.Now().Unix()`）
- [ ] `dns.Msg.Pack()` 序列化响应，`> 512 bytes` 跳过 XDP 缓存
- [ ] `bpf_map.Update()` 返回 `-ENOSPC` 时静默跳过

**5. Config（最小化）**

- [ ] `cmd/rec53.go` — `XDPConfig` 结构体：`Enabled bool`、`Interface string`（其余硬编码为常量）
- [ ] `config.yaml` + `generate-config.sh` — 添加 `xdp:` 配置块（默认 `enabled: false`）

**6. 验证**

- [ ] 手动测试：`dig @127.0.0.1` 查询已缓存域名，确认 XDP_TX 路径工作
- [ ] BPF stats map 读取确认 hit 计数递增
- [ ] cache miss 透传到 Go 正常解析
- [ ] 所有现有 e2e 测试通过（XDP 透传不影响功能）
- [ ] `go test -race ./...` 无竞争

### 开发依赖

- `clang` >= 14（BPF target）
- Linux kernel >= 5.15（`CONFIG_BPF=y`、`CONFIG_XDP_SOCKETS=y`）
- `CAP_BPF` + `CAP_NET_ADMIN`（或 root）

### 预计规模

~800 行代码（~300 C + ~400 Go + ~100 构建/配置），6-7 个新文件。

---

## v0.6.1 — XDP 生产就绪（指标 + 清理 + 验收）

**目标**：补全生产运行所需的可观测性、自动清理和性能验收。

**前置**：v0.6.0 完成。

### 任务清单

**1. Prometheus 指标导出（4 个核心 counter）**

- [ ] `server/xdp_metrics.go` — 周期读取 BPF per-CPU counters，导出为 Prometheus metrics
- [ ] `rec53_xdp_cache_hits_total`、`rec53_xdp_cache_misses_total`、`rec53_xdp_pass_total`、`rec53_xdp_errors_total`

**2. TTL 过期清理**

- [ ] 周期 goroutine（固定 100ms）：`unix.ClockGettime(unix.CLOCK_MONOTONIC)` 获取当前时间，遍历 BPF map，删除 `expire_ts` 过期条目
- [ ] 不做跨 cache 对账（eBPF 内联 expire_ts 检查已保证不服务 stale entries，map 空间浪费可接受）

**3. 性能验收**

- [ ] dnsperf benchmark：XDP enabled（generic/lo）vs disabled
- [ ] **验收标准**：cache hit QPS 可测量提升，P99 latency 不回退（cache miss 路径不受影响）
- [ ] 结果记录到 `docs/benchmarks.md`

**4. 文档**

- [ ] `docs/architecture.md` — 新增 XDP Cache 层描述
- [ ] `docs/benchmarks.md` — XDP 性能对比数据
- [ ] `README.md` / `README.zh.md` — XDP 配置说明、构建依赖、CAP_BPF 权限要求

### 预计规模

~200 行代码（~100 Go metrics/cleanup + ~100 tests），2 个文件新增 + 文档更新。

### 暂不实施

- 跨 cache 对账（eBPF 内联过期检查已保证正确性，map 空间充裕时无需额外清理）
- `rec53_xdp_cache_entries` gauge、`rec53_xdp_cache_sync_errors_total`（按需再加）
- `sync_interval` / `cache_size` 配置化（常量足够）
- veth pair 自动化集成测试（手动 dig + dnsperf 验证，功能稳定后再补）

---

## v1.0.0 — DNSSEC + DoT/DoH

**目标**：达到生产可用的安全性和传输加密标准。

### 任务清单

- [ ] DNSSEC 验证（完整信任链，从根到叶）
- [ ] DNS over TLS (DoT) 支持（RFC 7858）
- [ ] DNS over HTTPS (DoH) 支持（RFC 8484）
- [ ] 并发向多个 nameserver 查询（减少单点超时影响）
- [ ] 查询频率限制（防滥用）

---

## 开放 Backlog

以下为已识别但未排期的工作项，详见 `.rec53/BACKLOG.md`：

### 功能增强（高优先级）

| ID    | 标题                                    | 说明                                           |
|-------|-----------------------------------------|------------------------------------------------|
| O-016 | AAAA (IPv6) 上游选择支持                | 当前仅使用 A 记录选择上游 IP                   |
| O-006 | TCP 重试截断响应（RFC 1035）            | UDP 响应被截断时自动 TCP 重试                  |

### 安全与健壮性

| ID    | 标题                                    | 说明                                           |
|-------|-----------------------------------------|------------------------------------------------|
| B-014 | Glue 无 bailiwick 校验                  | 防止缓存投毒风险                               |
| O-022 | Response ID 未校验                      | 应验证 DNS 响应 ID 匹配请求                    |
| O-018 | 状态机死循环保护增强                    | 当前仅 MaxIterations=50，可增加更细粒度检测    |

### 缓存优化

| ID    | 标题                                    | 说明                                           |
|-------|-----------------------------------------|------------------------------------------------|
| O-021 | 无 glue 时委派 NS 不缓存               | 避免缓存不完整的委派信息                       |
| O-005 | 负缓存 TTL 可配置化                    | `DefaultNegativeCacheTTL=60` 应从配置读取      |

### 测试覆盖增强

| ID    | 标题                                    | 优先级 |
|-------|-----------------------------------------|--------|
| T-003 | 负缓存 E2E 测试                        | High   |
| T-004 | 查询预算耗尽测试                        | High   |
| T-005 | 超时重试与服务器切换测试                | High   |
| T-006 | SERVFAIL 与服务器黑名单测试             | High   |
| T-007 | Response ID 不匹配测试                  | Medium |
| T-008 | CNAME + NXDOMAIN 组合测试               | Medium |
| T-009 | Referral 循环检测测试                   | Medium |
| T-010 | 全部 NS 不可达测试                      | Medium |

---

## Future

### 长期目标

- DNS over QUIC (DoQ) 支持
- Response Policy Zones (RPZ)
- 高可用集群（多节点协调缓存）
- 查询日志与分析（可接入 ELK / ClickHouse）
- Web 管理面板
