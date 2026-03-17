## ADDED Requirements

### Requirement: Static hosts entries in config
The system SHALL support a `hosts` section in `config.yaml` where operators declare static DNS records (A, AAAA, CNAME). Each entry SHALL have `name` (domain), `type` (record type), `value` (IP or target), and an optional `ttl` (default 60 seconds).

#### Scenario: Valid hosts entry is loaded at startup
- **WHEN** `config.yaml` contains a `hosts` entry with name `db.internal`, type `A`, value `10.0.0.5`
- **THEN** the server SHALL start without error and serve that record for matching queries

#### Scenario: Hosts entry with missing ttl uses default
- **WHEN** a hosts entry omits the `ttl` field
- **THEN** the server SHALL use a TTL of 60 seconds in the response

---

### Requirement: Hosts lookup takes highest priority
The system SHALL resolve queries against the `hosts` table before consulting the cache or performing iterative resolution. A match in `hosts` SHALL return immediately without touching the cache or upstream.

#### Scenario: Query matches a hosts A record
- **WHEN** a client queries `db.internal. IN A` and a matching hosts entry exists with value `10.0.0.5`
- **THEN** the server SHALL return a response with `RCODE=NOERROR`, AA flag set, and an A record `10.0.0.5` with the configured TTL

#### Scenario: Query does not match any hosts entry
- **WHEN** a client queries `unknown.example.com. IN A` and no hosts entry matches
- **THEN** the server SHALL proceed to the next stage (forwarding check or cache lookup)

---

### Requirement: Hosts supports A, AAAA, and CNAME record types
The system SHALL support exactly three record types in hosts: `A` (IPv4), `AAAA` (IPv6), and `CNAME` (canonical name alias).

#### Scenario: AAAA record returned for IPv6 query
- **WHEN** a client queries `ipv6.internal. IN AAAA` and a matching hosts AAAA entry exists with value `::1`
- **THEN** the server SHALL return an AAAA record with that value

#### Scenario: CNAME record returned
- **WHEN** a client queries `alias.internal. IN A` and a matching hosts CNAME entry maps it to `real.internal`
- **THEN** the server SHALL return a CNAME record pointing to `real.internal`

#### Scenario: Type mismatch on hosts entry
- **WHEN** a client queries `db.internal. IN AAAA` but only an A record exists for `db.internal` in hosts
- **THEN** the server SHALL return `RCODE=NOERROR` with no answers (NODATA), not proceed to cache or upstream

---

### Requirement: Startup validation rejects malformed hosts entries
The system SHALL validate all hosts entries at startup and refuse to start if any entry contains an invalid IP address, unsupported record type, or empty name/value.

#### Scenario: Invalid IP address in A record
- **WHEN** `config.yaml` contains a hosts A entry with value `not-an-ip`
- **THEN** the server SHALL exit with a non-zero status and a descriptive error message

#### Scenario: Unsupported record type
- **WHEN** `config.yaml` contains a hosts entry with type `MX`
- **THEN** the server SHALL exit with a non-zero status and a descriptive error message
