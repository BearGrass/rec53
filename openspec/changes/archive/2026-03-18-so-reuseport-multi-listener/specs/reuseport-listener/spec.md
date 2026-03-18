## ADDED Requirements

### Requirement: Configurable listener count
The system SHALL accept a `dns.listeners` configuration field specifying the number of UDP+TCP listener pairs to bind on the same address. A value of 0 or 1 SHALL create a single listener pair without `SO_REUSEPORT` (preserving current behaviour). A value greater than 1 SHALL create N listener pairs with `SO_REUSEPORT` enabled.

#### Scenario: Default configuration (listeners omitted)
- **WHEN** `dns.listeners` is not specified in config
- **THEN** the system creates exactly 1 UDP and 1 TCP listener without `SO_REUSEPORT`

#### Scenario: Explicit single listener
- **WHEN** `dns.listeners` is set to 1
- **THEN** the system creates exactly 1 UDP and 1 TCP listener without `SO_REUSEPORT`

#### Scenario: Multiple listeners enabled
- **WHEN** `dns.listeners` is set to N (N > 1)
- **THEN** the system creates N UDP and N TCP listeners, each with `SO_REUSEPORT` enabled, all bound to the same `dns.listen` address

#### Scenario: Invalid negative value
- **WHEN** `dns.listeners` is set to a negative value
- **THEN** config validation SHALL reject the configuration with a clear error message

### Requirement: SO_REUSEPORT socket option
When multiple listeners are configured (N > 1), each `dns.Server` instance SHALL set `ReusePort: true`, causing `miekg/dns` to apply `SO_REUSEPORT` via `setsockopt` on the underlying socket. On platforms that do not support `SO_REUSEPORT`, the flag SHALL be silently ignored by the library.

#### Scenario: Linux SO_REUSEPORT activation
- **WHEN** `dns.listeners` is set to 4 on a Linux system
- **THEN** 4 UDP sockets and 4 TCP sockets are bound to the same address with `SO_REUSEPORT`, and the kernel distributes incoming packets across them

#### Scenario: Unsupported platform fallback
- **WHEN** `dns.listeners` is set to 4 on a platform without `SO_REUSEPORT` support
- **THEN** the system does not crash; `miekg/dns` silently ignores the `ReusePort` flag

### Requirement: Ready signalling with multiple listeners
The server SHALL signal readiness (close `udpReady`/`tcpReady` channels) when the **first** listener of each protocol type binds successfully. `Run()` SHALL block until both UDP and TCP readiness are signalled, then return the error channel.

#### Scenario: First UDP listener binds
- **WHEN** the first of N UDP listeners successfully binds
- **THEN** `udpReady` channel is closed and `UDPAddr()` returns the bound address

#### Scenario: First TCP listener binds
- **WHEN** the first of N TCP listeners successfully binds
- **THEN** `tcpReady` channel is closed and `TCPAddr()` returns the bound address

#### Scenario: Subsequent listener failure
- **WHEN** the first listener binds successfully but a subsequent listener fails
- **THEN** `Run()` has already returned; the failure is reported on the error channel

### Requirement: Graceful shutdown of all listeners
`Shutdown()` SHALL stop all N UDP and N TCP listener instances. It SHALL cancel warmup, shut down each server via `ShutdownContext()`, wait for all goroutines, then shut down the IP pool and save snapshot.

#### Scenario: Shutdown with multiple listeners
- **WHEN** `Shutdown()` is called with 4 active listener pairs
- **THEN** all 4 UDP and 4 TCP servers are shut down gracefully, and the method blocks until all goroutines have exited

#### Scenario: Shutdown with single listener (backward compat)
- **WHEN** `Shutdown()` is called with 1 listener pair (listeners=1)
- **THEN** behaviour is identical to current implementation

### Requirement: Startup logging
When `SO_REUSEPORT` is active (listeners > 1), the server SHALL log the listener count at startup.

#### Scenario: Multi-listener startup log
- **WHEN** server starts with `dns.listeners: 4`
- **THEN** log output includes the number of listener pairs and mentions SO_REUSEPORT
