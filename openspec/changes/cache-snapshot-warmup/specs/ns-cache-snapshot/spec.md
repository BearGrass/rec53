## ADDED Requirements

### Requirement: NS cache snapshot on shutdown
The system SHALL serialize all NS delegation cache entries to a JSON file on graceful shutdown (triggered by SIGTERM, SIGINT, or programmatic Shutdown() call), provided snapshot is enabled in configuration. Each entry SHALL include wire-format DNS message (base64-encoded), cache key, and save timestamp. Write failures SHALL be logged as errors but MUST NOT prevent normal shutdown completion.

#### Scenario: Snapshot written on graceful shutdown
- **WHEN** rec53 undergoes graceful shutdown (any trigger) and snapshot.enabled is true
- **THEN** a JSON file is written to snapshot.file path containing all NS delegation entries from globalDnsCache

#### Scenario: Shutdown completes even if snapshot write fails
- **WHEN** rec53 undergoes graceful shutdown and snapshot.file path is not writable
- **THEN** an error is logged and shutdown proceeds normally without blocking

#### Scenario: Snapshot disabled by default
- **WHEN** snapshot.enabled is false or the snapshot config block is absent
- **THEN** no file is written on shutdown and existing behavior is unchanged

### Requirement: NS cache restored from snapshot on startup
The system SHALL read the snapshot file synchronously in `cmd/main()` before calling `server.Run()`, so the cache is populated before UDP/TCP listeners start. Deserialization and cache writes complete on the calling goroutine; no background goroutine is used. Expired entries (save timestamp + TTL ≤ current time) SHALL be skipped. File-not-found and parse errors SHALL be treated as warnings, not fatal errors.

#### Scenario: Restore completes before first DNS query
- **WHEN** rec53 starts with a valid snapshot file and snapshot.enabled is true
- **THEN** LoadSnapshot() returns before server.Run() is called, guaranteeing NS entries are in globalDnsCache before any UDP or TCP query arrives

#### Scenario: Non-expired entries restored to cache
- **WHEN** rec53 starts and a valid snapshot file exists with entries whose TTL has not expired
- **THEN** those NS entries are present in globalDnsCache before the first DNS query is handled

#### Scenario: Expired entries are skipped
- **WHEN** a snapshot entry's save_timestamp + ttl is less than the current time
- **THEN** that entry is not written to globalDnsCache

#### Scenario: Missing snapshot file is not fatal
- **WHEN** rec53 starts and snapshot.file does not exist
- **THEN** startup proceeds normally with only Round 1 TLD warmup

#### Scenario: Corrupt snapshot file is not fatal
- **WHEN** rec53 starts and snapshot.file contains invalid JSON
- **THEN** a warning is logged and startup proceeds normally

### Requirement: Snapshot configuration
The system SHALL expose a SnapshotConfig struct with fields: Enabled (bool), File (string path). The snapshot block SHALL be optional in config.yaml; omitting it MUST be equivalent to enabled: false. All NS delegation entries are saved without truncation.

#### Scenario: Config block omitted is equivalent to disabled
- **WHEN** config.yaml does not contain a snapshot: block
- **THEN** no snapshot is read or written
