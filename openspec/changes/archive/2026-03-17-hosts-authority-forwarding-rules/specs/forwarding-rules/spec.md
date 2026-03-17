## ADDED Requirements

### Requirement: Forwarding zones in config
The system SHALL support a `forwarding` section in `config.yaml` where operators declare forwarding zones. Each zone entry SHALL have a `zone` field (domain suffix) and an `upstreams` list (one or more `host:port` addresses). Entries are matched using longest-suffix-first ordering.

#### Scenario: Valid forwarding zone is loaded at startup
- **WHEN** `config.yaml` contains a forwarding entry with zone `corp.example.com` and upstream `192.168.1.1:53`
- **THEN** the server SHALL start without error

#### Scenario: Forwarding zone with no upstreams rejected at startup
- **WHEN** a forwarding entry has an empty `upstreams` list
- **THEN** the server SHALL exit with a non-zero status and a descriptive error message

---

### Requirement: Forwarding lookup occurs after hosts, before cache
The system SHALL check forwarding rules after the hosts table but before cache lookup. A query matching a forwarding zone SHALL be forwarded to the configured upstreams without consulting the global cache or performing iterative resolution.

#### Scenario: Query matches a forwarding zone
- **WHEN** a client queries `svc.corp.example.com. IN A` and a forwarding zone for `corp.example.com` is configured
- **THEN** the server SHALL forward the query to the configured upstreams and return the upstream response to the client

#### Scenario: Query does not match any forwarding zone
- **WHEN** a client queries `public.example.org. IN A` and no forwarding zone matches
- **THEN** the server SHALL proceed to cache lookup and iterative resolution as normal

---

### Requirement: Longest suffix match selects forwarding zone
When multiple forwarding zones could match a query name, the system SHALL use the zone with the longest matching suffix.

#### Scenario: More specific zone takes precedence
- **WHEN** forwarding zones for `example.com` and `db.example.com` are both configured, and a client queries `primary.db.example.com. IN A`
- **THEN** the server SHALL forward the query using the upstreams for `db.example.com`, not `example.com`

---

### Requirement: Forwarding results are not cached globally
The system SHALL NOT write forwarded responses into `globalDnsCache`. Each forwarded query SHALL reach the upstream DNS server directly.

#### Scenario: Forwarded response bypasses global cache
- **WHEN** a forwarded query returns a response
- **THEN** a subsequent identical query SHALL again be forwarded to the upstream, not served from cache

---

### Requirement: All upstreams failing returns SERVFAIL
If all configured upstreams for a matching forwarding zone fail (timeout or error), the system SHALL return `RCODE=SERVFAIL` to the client and SHALL NOT fall back to iterative resolution.

#### Scenario: Single upstream times out
- **WHEN** the only configured upstream for a zone is unreachable and the upstream timeout expires
- **THEN** the server SHALL return `RCODE=SERVFAIL` to the client

#### Scenario: Multiple upstreams all fail
- **WHEN** all upstreams in a forwarding zone fail sequentially
- **THEN** the server SHALL return `RCODE=SERVFAIL` after exhausting all upstreams

---

### Requirement: Startup validation rejects malformed forwarding entries
The system SHALL validate all forwarding entries at startup and refuse to start if any entry has an empty zone, invalid upstream address format, or empty upstreams list.

#### Scenario: Invalid upstream address format
- **WHEN** a forwarding entry has upstream `not-valid-address`
- **THEN** the server SHALL exit with a non-zero status and a descriptive error message
