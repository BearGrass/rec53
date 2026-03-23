## MODIFIED Requirements

### Requirement: DNS server initialization
The system SHALL initialize the DNS server with proper error handling and crash protection during startup, ensuring all dependencies are properly validated before use. When `listeners` is greater than 1, `Run()` SHALL create N UDP+TCP `dns.Server` pairs with `ReusePort: true` and start each in its own goroutine. Ready-channel closure SHALL use `sync.Once` to guarantee exactly-once signalling from the first listener to bind. When XDP is enabled, `Run()` SHALL initialize the XDP loader (load eBPF objects, attach XDP to configured interface) BEFORE starting DNS listeners, and pass the BPF cache_map handle to the cache sync module. The startup and shutdown sequence SHALL also keep the runtime health contract consistent: the server SHALL remain not-ready until a DNS listener bind succeeds, SHALL treat warmup as a non-blocking post-bind phase, SHALL transition from `warming` to `steady` when warmup completes, and SHALL transition to not-ready before graceful shutdown completes listener teardown.

#### Scenario: Server starts with valid config
- **WHEN** DNS server is initialized with valid listen address and warmup config
- **THEN** server successfully initializes and begins accepting DNS queries

#### Scenario: Server starts with listeners > 1
- **WHEN** DNS server is initialized with `listeners: 4`
- **THEN** server creates 4 UDP and 4 TCP `dns.Server` instances with `ReusePort: true`, starts all 8 goroutines, and signals readiness after the first UDP and first TCP listeners bind

#### Scenario: Server starts with XDP enabled
- **WHEN** DNS server is initialized with `xdp.enabled: true` and `xdp.interface: "eth0"`
- **THEN** server SHALL load eBPF objects and attach XDP to `eth0` BEFORE starting DNS listeners
- **AND** cache sync module SHALL receive the BPF cache_map handle
- **AND** 启动日志 SHALL 输出 XDP attach 模式和网卡名称

#### Scenario: Server starts with XDP enabled but attach fails
- **WHEN** XDP attach 失败（如内核不支持、权限不足）
- **THEN** server SHALL 记录错误日志但继续启动（XDP 功能降级）
- **AND** DNS 服务 SHALL 正常工作（Go 路径不受影响）

#### Scenario: Server shutdown with XDP
- **WHEN** server 收到 shutdown 信号且 XDP 已启用
- **THEN** server SHALL 先关闭 DNS listeners，再 detach XDP 并关闭 BPF objects

#### Scenario: Server initialization with nil config
- **WHEN** DNS server receives nil or invalid configuration
- **THEN** system validates configuration before use and returns error instead of panicking

#### Scenario: Server initialization with invalid listen address
- **WHEN** DNS server is initialized with unparseable listen address
- **THEN** initialization fails gracefully with clear error message instead of crashing

#### Scenario: Warmup routine robustness
- **WHEN** warmup routine is started during server initialization
- **THEN** any panics in warmup are contained and don't crash the main server process

#### Scenario: Warmup does not block readiness after bind
- **WHEN** DNS listeners are bound successfully and warmup continues in the background
- **THEN** the server SHALL remain ready to accept DNS traffic while runtime phase reports warming

#### Scenario: Warmup completion transitions to steady
- **WHEN** warmup is enabled and the background warmup routine finishes after listener bind
- **THEN** the server SHALL transition runtime phase to `steady` without dropping readiness

#### Scenario: Startup failure never transitions to ready
- **WHEN** DNS server startup fails before any DNS listener reaches serving state
- **THEN** the server SHALL NOT expose a ready runtime state

#### Scenario: Graceful shutdown drops readiness before listener teardown completes
- **WHEN** rec53 begins graceful shutdown
- **THEN** the server SHALL transition its runtime readiness to false before all listener shutdown work finishes
