## Context

rec53 是一个从根服务器开始的全递归 DNS 解析器，当前 Go 实现在 cache hit 路径上受限于系统调用开销（`sendmsg` 占 22.5% CPU）、GC/内存分配（29%）和 goroutine 栈增长（8%）。v0.5.0 已完成 Go 层面的热路径优化（指标去分配、cache key 微优化、shallow copy），剩余瓶颈在 Go 运行时之下，无法在用户态进一步消除。

XDP（eXpress Data Path）允许在网卡驱动层（或 generic 模式下在内核网络栈入口）运行 eBPF 程序处理网络包。对于 DNS cache hit，XDP 程序可以直接在内核空间构建响应并 `XDP_TX` 返回，完全绕过 Go 运行时。

当前 `globalDnsCache` 使用 `go-cache` 库（内存 map + TTL），cache 写入通过 `setCacheCopy()` / `setCacheCopyByType()` 完成。XDP cache 需要在 Go 写入缓存时同步一份预序列化的 wire-format 副本到 BPF hash map。

## Goals / Non-Goals

**Goals:**

- 实现可工作的 XDP DNS cache 快速路径：cache hit 由 eBPF 直接响应，miss 透传 Go
- Go 层 cache 写入时自动同步到 BPF map，无需独立同步 goroutine
- 默认关闭（`xdp.enabled: false`），不影响现有功能和测试
- 构建系统集成：`bpf2go` 生成 Go 绑定，eBPF 对象嵌入 Go binary
- 支持 native XDP 和 generic XDP 模式自动检测回退

**Non-Goals:**

- Prometheus 指标导出（v0.6.1 scope）
- BPF map TTL 过期清理 goroutine（v0.6.1 scope）
- veth pair 自动化集成测试（手动 dig + dnsperf 验证）
- IPv6 XDP 支持（当前 rec53 仅支持 IPv4 上游）
- EDNS0 / TCP / 大于 512 字节响应的 XDP 处理（透传 Go）
- `sync_interval` / `cache_size` 配置化（常量足够）

## Decisions

### D1: BPF map 类型 — `BPF_MAP_TYPE_HASH` (65536 entries)

**选择**: Hash map，固定 65536 entries。

**替代方案**:
- `BPF_MAP_TYPE_LRU_HASH`: 自动 LRU 淘汰，但 Linux < 5.16 的 LRU 实现有并发性能问题，且 Go 侧已管理 TTL。
- `BPF_MAP_TYPE_ARRAY`: O(1) 但需要自行管理 hash → index 映射，key 空间不固定。

**理由**: Hash map 是 DNS cache 场景的标准选择——key 空间不固定（域名+类型）、需要精确查找、verifier 友好。65536 条目足够覆盖单机热点域名。Go 侧 TTL 管理 + eBPF 内联 expire_ts 检查保证不服务过期条目。

### D2: Cache key 格式 — wire-format qname + qtype

**选择**: BPF map key = `{ u8 qname[MAX_QNAME_LEN]; u16 qtype; }` padding to fixed size, qname 使用 DNS wire format（长度前缀标签序列，小写归一化）。

**替代方案**:
- Presentation format（`"example.com.:1"`）: 需要在 eBPF 中做字符串 → wire format 转换，复杂且低效。
- Hash of qname: 损失调试可观测性，hash 碰撞需要额外处理。

**理由**: DNS wire format 是网络包中的原生格式，eBPF 程序可直接从包中 memcpy 作为 key，零转换开销。Go 侧需要做一次 presentation → wire format 转换（`dns.Fqdn()` + 手动编码），但这只在 cache write 时发生（低频路径）。

### D3: 响应预序列化 — Go 侧 Pack() 后存入 BPF map value

**选择**: Go 写入 BPF map 时调用 `dns.Msg.Pack()` 将完整响应序列化为 wire format 存入 value，eBPF 程序直接 memcpy 到包中。

**替代方案**:
- eBPF 内构建响应: 需要在 eBPF 中实现 DNS 序列化，复杂度极高且 verifier 限制多。
- 存储结构化数据 + eBPF 组装: 中间方案，但 verifier 对动态大小数据操作限制严格。

**理由**: 预序列化将复杂度移到 Go 侧（已有成熟的 `dns.Msg.Pack()`），eBPF 程序只需 memcpy + patch Transaction ID，极简且 verifier 友好。代价是 value 存储空间稍大（完整 wire format），但 512 字节上限可控。

### D4: TTL 管理 — monotonic clock expire_ts

**选择**: BPF map value 存储 `expire_ts`（monotonic 秒），使用 `bpf_ktime_get_ns() / 1_000_000_000` 比较。Go 侧用 `unix.ClockGettime(unix.CLOCK_MONOTONIC)` 计算 `expire_ts = now + ttl`。

**替代方案**:
- Wall clock (`time.Now().Unix()`): NTP 跳变会导致大量条目瞬间过期或永不过期。
- Go 侧定期扫描删除: 增加同步复杂度，且 eBPF 内联检查已保证不服务过期数据。

**理由**: Monotonic clock 不受 NTP 影响，eBPF 和 Go 共用同一时钟源，一致性有保证。过期条目占用 map 空间但不服务请求，v0.6.1 的清理 goroutine 会回收空间。

### D5: cilium/ebpf + bpf2go 作为 Go eBPF 库

**选择**: `github.com/cilium/ebpf` + `bpf2go` 代码生成。

**替代方案**:
- `libbpf-go`: 薄包装，依赖系统安装的 libbpf，交叉编译复杂。
- `gobpf` (iovisor): 维护不活跃，API 不稳定。
- 直接 syscall: 可行但需要大量样板代码。

**理由**: cilium/ebpf 是 Go 生态最成熟的纯 Go eBPF 库，无 CGO 依赖，`bpf2go` 在编译时将 `.o` 嵌入 Go binary（`//go:embed`），部署时无需额外文件。Cilium、Grafana Beyla 等生产项目均使用此方案。

### D6: XDP attach 模式 — native 优先，generic 回退

**选择**: 先尝试 native mode（`XDP_FLAGS_DRV_MODE`），失败后回退 generic mode（`XDP_FLAGS_SKB_MODE`）。

**理由**: Native mode 性能最优（驱动层处理），但需要网卡驱动支持。Generic mode 兼容所有网卡但性能稍低（内核网络栈入口）。开发/测试环境（loopback）只支持 generic mode。自动回退保证可用性。

### D7: Cache 同步策略 — 同步写入（inline）

**选择**: 在 `setCacheCopy()` / `setCacheCopyByType()` 返回前，同步写入 BPF map。

**替代方案**:
- 异步 channel + 专用 goroutine: 解耦但增加延迟和复杂度，且 cache write 频率不高。
- 定期全量同步: 简单但延迟高，cache hit 路径会有窗口期。

**理由**: Cache write 是低频路径（仅 cache miss 时触发），同步写入的额外延迟（BPF map update ~1-2μs）可忽略不计。即时同步保证 XDP cache 与 Go cache 强一致，无窗口期。

### D8: 响应大小上限 — 512 字节

**选择**: 仅缓存 `Pack()` 后 ≤ 512 字节的响应到 BPF map，超过的跳过。

**理由**: 512 字节是传统 DNS UDP 报文上限（无 EDNS0），覆盖绝大多数 A/AAAA/CNAME 查询响应。大于 512 字节的响应通常需要 EDNS0 或 TC+TCP 回退，不适合 XDP 快速路径。BPF map value 固定大小避免了动态分配复杂度。

## Risks / Trade-offs

**[Risk] Verifier 拒绝复杂 eBPF 程序** → 将 DNS 解析逻辑拆分为小函数，每个函数控制在 verifier 复杂度限制内。使用 `__always_inline`。bounded loop（kernel 5.3+）替代 `#pragma unroll`。开发过程中频繁加载测试。

**[Risk] BPF map 空间耗尽（65536 entries）** → `bpf_map_update_elem` 返回 `-ENOSPC` 时静默跳过，Go 侧正常处理。生产环境热点域名远低于 65536。v0.6.1 的 TTL 清理会回收过期条目空间。

**[Risk] XDP_TX 在某些网卡/虚拟化环境不工作** → Generic mode 回退保证基本可用。容器环境（Docker bridge）可能需要 `XDP_REDIRECT` 但不在 scope 内（默认关闭 XDP）。

**[Risk] Monotonic clock skew between eBPF and Go** → 两者使用同一内核 `CLOCK_MONOTONIC`，理论上无 skew。唯一风险是 Go 的 `unix.ClockGettime` 与 eBPF 的 `bpf_ktime_get_ns` 精度差异（纳秒 vs 秒级截断），但 TTL 精度为秒级，差异可忽略。

**[Risk] eBPF 构建依赖增加 CI 复杂度** → `bpf2go` 生成的 Go 文件提交到仓库，CI 不需要 clang。仅开发者修改 eBPF 代码时需要 clang >= 14。

**[Trade-off] BPF map value 存储预序列化响应占用较多内存** → 每条 entry 最大 512 + 元数据字节，65536 条目 ≈ 32-40 MB 内核内存。对于 DNS 服务器可接受。换来的是 eBPF 程序极简（仅 memcpy + patch TxID）。

**[Trade-off] 仅支持 IPv4** → 当前 rec53 上游查询仅使用 IPv4，XDP 程序仅解析 IPv4 header。IPv6 支持可在需求明确后添加（额外 ~50 行 C 代码）。
