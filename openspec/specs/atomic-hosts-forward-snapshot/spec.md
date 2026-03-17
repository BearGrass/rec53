## ADDED Requirements

### Requirement: Atomic snapshot for hosts and forward configuration
The system SHALL store hosts map, hosts names, and forward zones as a single immutable snapshot accessed via `atomic.Pointer`, so that concurrent readers and writers never observe a partially-updated configuration state.

#### Scenario: Concurrent write and read see consistent snapshot
- **WHEN** `setGlobalHostsAndForward` is called while a request-handling goroutine is reading the configuration
- **THEN** the goroutine SHALL observe either the complete old snapshot or the complete new snapshot, never a mix of old and new fields

#### Scenario: Empty snapshot on init
- **WHEN** the server package is loaded before any configuration is applied
- **THEN** `globalHostsForward.Load()` SHALL return a non-nil pointer to an empty snapshot (nil maps, nil slice)

#### Scenario: Reset clears all configuration fields atomically
- **WHEN** `ResetHostsAndForwardForTest` is called
- **THEN** a subsequent `Load()` SHALL return a snapshot where `hostsMap`, `hostsNames`, and `forwardZones` are all nil or empty

### Requirement: Hosts lookup reads from atomic snapshot
The `hostsLookupState.handle` function SHALL load the configuration snapshot once at the start of execution and use only the loaded snapshot throughout the function, rather than reading global variables multiple times.

#### Scenario: Empty hosts map returns MISS
- **WHEN** the snapshot's `hostsMap` is nil or empty
- **THEN** `handle` SHALL return `HOSTS_LOOKUP_MISS` without error

#### Scenario: Hit returns answer from snapshot
- **WHEN** the snapshot's `hostsMap` contains an entry matching the query name and type
- **THEN** `handle` SHALL copy the answer RRs and return `HOSTS_LOOKUP_HIT`

#### Scenario: NODATA returns HIT with empty answer
- **WHEN** the snapshot's `hostsNames` contains the query name but `hostsMap` has no matching type entry
- **THEN** `handle` SHALL return `HOSTS_LOOKUP_HIT` with `Rcode=NOERROR` and zero answer RRs

### Requirement: Forward lookup reads from atomic snapshot
The `forwardLookupState.handle` function SHALL load the configuration snapshot once at the start of execution and use only the loaded snapshot's `forwardZones` throughout the function.

#### Scenario: Empty zones returns MISS
- **WHEN** the snapshot's `forwardZones` is nil or empty
- **THEN** `handle` SHALL return `FORWARD_LOOKUP_MISS` without error

#### Scenario: Matching zone forwards to upstream
- **WHEN** the snapshot's `forwardZones` contains a zone that is a suffix of the query name
- **THEN** `handle` SHALL forward the query to the zone's upstreams and return `FORWARD_LOOKUP_HIT` on success

#### Scenario: No matching zone returns MISS
- **WHEN** no zone in the snapshot's `forwardZones` is a suffix of the query name
- **THEN** `handle` SHALL return `FORWARD_LOOKUP_MISS`

### Requirement: Test helpers use atomic snapshot interface
All test code in `package server` that sets or inspects hosts/forward configuration SHALL use the atomic snapshot interface rather than assigning to raw global variables, ensuring `-race` detector passes when tests run in parallel.

#### Scenario: Internal test sets configuration via snapshot helper
- **WHEN** a `package server` test needs to set `hostsMap` or `hostsNames` or `forwardZones`
- **THEN** it SHALL call `setSnapshotForTest` (or equivalent) rather than assigning to a removed raw global variable
- **THEN** the `-race` detector SHALL report no data race during the test

#### Scenario: Internal test restores configuration on cleanup
- **WHEN** a `package server` test sets a custom snapshot in setup
- **THEN** its `defer` or `t.Cleanup` SHALL restore the previous snapshot via the atomic interface

### Requirement: e2e test cleanup resets hosts and forward state
`setupResolverWithMockRoot` SHALL reset hosts and forward configuration in its `t.Cleanup` function to prevent state leakage between e2e tests.

#### Scenario: Hosts and forward state cleared after test
- **WHEN** a test that called `setupResolverWithMockRoot` finishes
- **THEN** `ResetHostsAndForwardForTest` SHALL have been called, leaving an empty snapshot for subsequent tests
