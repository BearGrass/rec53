## MODIFIED Requirements

### Requirement: DNS server initialization
The system SHALL initialize the DNS server with proper error handling and crash protection during startup. When `listeners` is greater than 1, `Run()` SHALL create N UDP+TCP `dns.Server` pairs with `ReusePort: true` and start each in its own goroutine. Readiness signalling SHALL use `sync.Once` and SHALL return only after a successful listener bind, or fail fast if startup errors arrive before readiness.

#### Scenario: Server starts with valid config
- **WHEN** DNS server is initialized with a valid listen address and warmup config
- **THEN** server successfully initializes and begins accepting DNS queries

#### Scenario: Server starts with listeners > 1
- **WHEN** DNS server is initialized with `listeners: 4`
- **THEN** server creates 4 UDP and 4 TCP `dns.Server` instances with `ReusePort: true`, starts all 8 goroutines, and signals readiness after the first UDP and first TCP listeners bind

#### Scenario: Server startup fails before ready
- **WHEN** a listener fails to bind before UDP or TCP readiness is signalled
- **THEN** `Run()` SHALL surface the startup error instead of waiting indefinitely on readiness channels

#### Scenario: Server starts with XDP enabled
- **WHEN** DNS server is initialized with `xdp.enabled: true` and `xdp.interface: "eth0"`
- **THEN** server SHALL initialize XDP before starting DNS listeners
- **AND** cache sync module SHALL receive the BPF cache map handle when attach succeeds

#### Scenario: Server starts with XDP enabled but attach fails
- **WHEN** XDP attach fails because of permissions, kernel support, or interface limitations
- **THEN** server SHALL log the degradation and continue to serve DNS via the Go path

### Requirement: DNS server shutdown
The system SHALL shut down background work in a deterministic order so that DNS listeners, warmup, XDP helpers, and cache-related goroutines do not deadlock or outlive the server lifecycle.

#### Scenario: Server shutdown with XDP
- **WHEN** server receives shutdown while XDP is enabled
- **THEN** it SHALL stop DNS listeners, cancel XDP background loops, and only then close BPF objects

#### Scenario: Server shutdown with background warmup
- **WHEN** warmup is still running during shutdown
- **THEN** shutdown SHALL cancel warmup before tearing down shared background services
