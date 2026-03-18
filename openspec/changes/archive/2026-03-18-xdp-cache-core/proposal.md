## Why

CPU profiling 显示 172K QPS 下 22.5% CPU 耗在 `sendmsg` 系统调用、29% 在 GC/内存分配、8% 在 goroutine 栈增长。对于 cache hit（生产环境主要流量），整个 Go 运行时开销不必要——答案已知，只需原样返回。XDP 在网卡驱动层拦截 DNS 查询，cache hit 完全在内核空间完成，零系统调用、零内存拷贝、零 goroutine，可大幅突破当前 QPS 天花板。

v0.5.0 已完成 Go 层热路径优化（指标去分配、cache key 微优化、shallow copy），剩余 CPU 开销集中在系统调用和运行时层面，Go 内无法进一步消除——需要下沉到内核空间。

## What Changes

- 新增 eBPF/XDP 程序（C），在网卡驱动层解析 DNS 查询并匹配 BPF hash map 缓存
- cache hit 时直接 swap ETH/IP/UDP header、memcpy 预序列化响应、`XDP_TX` 返回，绕过整个 Go 栈
- cache miss 时 `XDP_PASS` 透传到 Go 走现有递归解析流程，行为不变
- 新增 Go loader（基于 cilium/ebpf），负责加载 eBPF 对象、attach XDP 到指定 interface
- 新增 cache 同步逻辑：Go 层 `setCacheCopy()` 写入缓存时同步写入 BPF map（wire format key + 预序列化 response）
- 新增 `xdp:` 配置块（默认 `enabled: false`），最小化配置项
- 构建系统新增 clang BPF 编译 + cilium/ebpf `bpf2go` 代码生成

## Capabilities

### New Capabilities

- `xdp-ebpf-program`: XDP/eBPF DNS cache 快速路径程序——ETH/IP/UDP/DNS 解析、qname 提取、BPF map 缓存查找、响应构建与 XDP_TX 返回
- `xdp-go-loader`: Go 侧 eBPF 加载器——加载编译后的 eBPF 对象、attach XDP 到网卡、native/generic 模式自动检测、生命周期管理
- `xdp-cache-sync`: Go 缓存到 BPF map 的同步机制——wire format 域名转换、预序列化响应、monotonic clock TTL 计算、BPF map 写入
- `xdp-build-system`: 构建系统集成——clang BPF 编译、cilium/ebpf bpf2go 代码生成、go:generate 嵌入
- `xdp-config`: XDP 配置接入——`XDPConfig` 结构体、config.yaml xdp 块、默认关闭

### Modified Capabilities

- `dns-server-startup`: 启动流程新增 XDP loader 初始化步骤（在 DNS listener 启动前 attach XDP）
- `cache-shallow-copy`: `setCacheCopy()` / `setCacheCopyByType()` 调用路径新增 BPF map 同步写入

## Impact

- **新增依赖**: `github.com/cilium/ebpf`（Go eBPF 库）、clang >= 14（BPF 编译）
- **运行时要求**: Linux kernel >= 5.15（`CONFIG_BPF=y`、`CONFIG_XDP_SOCKETS=y`）、`CAP_BPF` + `CAP_NET_ADMIN`
- **新增文件**: ~6-7 个（`server/xdp/dns_cache.c`、`server/xdp/dns_cache.h`、`server/xdp/Makefile`、`server/xdp_loader.go`、`server/xdp_sync.go`、生成的 `*_bpfel.go` / `*_bpfeb.go`）
- **现有代码影响**: `server/server.go`（启动/关闭流程）、`server/cache.go`（写入路径新增同步调用）、`cmd/rec53.go`（配置解析）
- **向后兼容**: XDP 默认关闭，不影响现有功能；所有现有测试无需修改
- **预计代码量**: ~800 行（~300 C + ~400 Go + ~100 构建/配置）
