## ADDED Requirements

### Requirement: handle retries with secondary IP on primary failure

#### Scenario: primary IP fails, secondary succeeds
- **WHEN** the primary upstream server fails to respond and a secondary IP exists
- **THEN** the query is retried against the secondary IP and returns `QUERY_UPSTREAM_NO_ERROR`

#### Scenario: both primary and secondary fail
- **WHEN** both the primary and secondary upstream servers fail
- **THEN** returns `QUERY_UPSTREAM_COMMON_ERROR`

---

### Requirement: handle bad Rcode from upstream triggers secondary retry

#### Scenario: primary returns SERVFAIL, secondary succeeds
- **WHEN** primary returns Rcode=SERVFAIL and a secondary IP exists
- **THEN** retries against secondary, records failure for primary, returns `QUERY_UPSTREAM_NO_ERROR`

#### Scenario: primary returns REFUSED, no secondary
- **WHEN** primary returns Rcode=REFUSED and no secondary IP is available
- **THEN** returns `QUERY_UPSTREAM_COMMON_ERROR`

---

### Requirement: handle rejects mismatched question in response

#### Scenario: response question name differs from request
- **WHEN** the upstream server returns a response with a different question name
- **THEN** returns `QUERY_UPSTREAM_COMMON_ERROR`

#### Scenario: response has empty question section
- **WHEN** the upstream returns a response with no question section
- **THEN** returns `QUERY_UPSTREAM_COMMON_ERROR`

---

### Requirement: handle returns NO_ERROR on NXDOMAIN

#### Scenario: upstream returns NXDOMAIN
- **WHEN** the upstream server returns Rcode=NXDOMAIN
- **THEN** returns `QUERY_UPSTREAM_NO_ERROR` (NXDOMAIN is a valid authoritative answer)

---

### Requirement: handle respects cancelled context

#### Scenario: context already cancelled before query
- **WHEN** `handle` is called with an already-cancelled context
- **THEN** returns `QUERY_UPSTREAM_COMMON_ERROR` immediately without sending a network request
