## ADDED Requirements

### Requirement: eBPF object loading
Go loader SHALL 使用 cilium/ebpf 库加载 `bpf2go` 生成的 eBPF 对象（编译时嵌入 Go binary）。加载失败时 MUST 返回错误并记录详细日志（内核版本、缺少的 BPF 特性等）。

#### Scenario: Successful loading
- **WHEN** XDP 启用且内核支持 BPF
- **THEN** loader SHALL 成功加载 eBPF 程序和所有 BPF maps
- **AND** 日志 SHALL 输出加载成功信息

#### Scenario: Loading failure due to kernel version
- **WHEN** 内核不支持所需 BPF 特性（如 kernel < 5.15）
- **THEN** loader SHALL 返回描述性错误信息
- **AND** rec53 SHALL 继续运行（XDP 功能降级，Go 路径正常工作）

### Requirement: XDP attach with mode auto-detection
Go loader SHALL 先尝试 native XDP mode（`XDP_FLAGS_DRV_MODE`）attach 到指定网卡，失败时自动回退到 generic mode（`XDP_FLAGS_SKB_MODE`）。实际使用的模式 MUST 在启动日志中输出。

#### Scenario: Native mode attach success
- **WHEN** 网卡驱动支持 native XDP
- **THEN** loader SHALL attach 为 native mode
- **AND** 日志 SHALL 输出 `[XDP] attached to <interface> in native mode`

#### Scenario: Fallback to generic mode
- **WHEN** 网卡驱动不支持 native XDP（如 loopback）
- **THEN** loader SHALL 自动回退到 generic mode
- **AND** 日志 SHALL 输出 `[XDP] attached to <interface> in generic mode (native not supported)`

#### Scenario: Both modes fail
- **WHEN** native 和 generic mode 均 attach 失败
- **THEN** loader SHALL 返回错误
- **AND** rec53 SHALL 继续运行（XDP 功能不可用，Go 路径正常工作）

### Requirement: Lifecycle management
Go loader SHALL 在 context 取消时（graceful shutdown）detach XDP 程序并关闭所有 BPF objects（maps、programs、links）。资源泄漏 MUST 为零。

#### Scenario: Graceful shutdown cleanup
- **WHEN** rec53 收到 SIGTERM/SIGINT 且 context 被取消
- **THEN** loader SHALL detach XDP 程序
- **AND** 关闭所有 BPF objects
- **AND** `bpftool prog show` SHALL 不再显示该 XDP 程序

#### Scenario: Crash recovery
- **WHEN** rec53 进程异常退出（未经 graceful shutdown）
- **THEN** 内核 SHALL 在 fd 关闭后自动清理 BPF 程序（内核引用计数机制）

### Requirement: BPF map handle exposure
Go loader SHALL 暴露 `cache_map` 和 `xdp_stats` 的 `*ebpf.Map` handle，供 cache sync 模块和未来的 metrics 模块使用。

#### Scenario: Cache map accessible after load
- **WHEN** eBPF 对象加载成功
- **THEN** `CacheMap()` 方法 SHALL 返回 non-nil `*ebpf.Map`
- **AND** 该 map 支持 `Lookup`、`Update`、`Delete` 操作

#### Scenario: Stats map accessible after load
- **WHEN** eBPF 对象加载成功
- **THEN** `StatsMap()` 方法 SHALL 返回 non-nil `*ebpf.Map`
- **AND** 该 map 支持 `Lookup` 操作
