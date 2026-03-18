## MODIFIED Requirements

### Requirement: DNS server initialization
The system SHALL initialize the DNS server with proper error handling and crash protection during startup, ensuring all dependencies are properly validated before use. When `listeners` is greater than 1, `Run()` SHALL create N UDP+TCP `dns.Server` pairs with `ReusePort: true` and start each in its own goroutine. Ready-channel closure SHALL use `sync.Once` to guarantee exactly-once signalling from the first listener to bind.

#### Scenario: Server starts with valid config
- **WHEN** DNS server is initialized with valid listen address and warmup config
- **THEN** server successfully initializes and begins accepting DNS queries

#### Scenario: Server starts with listeners > 1
- **WHEN** DNS server is initialized with `listeners: 4`
- **THEN** server creates 4 UDP and 4 TCP `dns.Server` instances with `ReusePort: true`, starts all 8 goroutines, and signals readiness after the first UDP and first TCP listeners bind

#### Scenario: Server initialization with nil config
- **WHEN** DNS server receives nil or invalid configuration
- **THEN** system validates configuration before use and returns error instead of panicking

#### Scenario: Server initialization with invalid listen address
- **WHEN** DNS server is initialized with unparseable listen address
- **THEN** initialization fails gracefully with clear error message instead of crashing

#### Scenario: Warmup routine robustness
- **WHEN** warmup routine is started during server initialization
- **THEN** any panics in warmup are contained and don't crash the main server process
