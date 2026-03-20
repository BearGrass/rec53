## ADDED Requirements

### Requirement: Runtime observability labels remain bounded

rec53 SHALL expose new runtime observability metrics only with bounded labels such as fixed `result`, `reason`, `stage`, `rcode`, or `path` enumerations. Raw domain names, full upstream lists, request IDs, and arbitrary error strings MUST NOT appear as metric labels.

#### Scenario: Domain names are excluded from new metrics
- **WHEN** a cache miss, snapshot load failure, or upstream timeout is recorded
- **THEN** the corresponding metric SHALL classify the event by bounded result or reason labels
- **AND** the metric SHALL NOT include the queried domain name as a label

#### Scenario: Free-form errors are excluded from labels
- **WHEN** a runtime path emits an implementation-specific error string
- **THEN** the string SHALL be handled by logs rather than exposed as a Prometheus label value

### Requirement: Cache observability covers lookup results and entry lifecycle

rec53 SHALL expose cache observability metrics that distinguish positive hit, negative hit, and miss outcomes, and SHALL expose entry lifecycle signals for current cache size and expiration or cleanup activity.

#### Scenario: Positive and negative cache hits are distinguishable
- **WHEN** a query is answered from a positive cache entry or a negative cache entry
- **THEN** cache observability SHALL record distinct result categories for those two outcomes

#### Scenario: Cache misses are visible separately
- **WHEN** cache lookup does not find a usable entry
- **THEN** cache observability SHALL increment a miss category that is separate from all hit categories

#### Scenario: Cache lifecycle is visible
- **WHEN** expired entries are deleted or the cache size changes over time
- **THEN** runtime observability SHALL expose signals that allow operators to inspect current entry count and expiration or cleanup activity

### Requirement: Snapshot observability covers save and restore outcomes

rec53 SHALL expose snapshot observability metrics for save and load attempts, success or failure outcomes, restored entry count, skipped expired entry count, skipped corrupt entry count, and total operation duration.

#### Scenario: Successful snapshot load is measurable
- **WHEN** `LoadSnapshot` restores unexpired entries during startup
- **THEN** runtime observability SHALL record the load attempt as successful
- **AND** SHALL expose the imported entry count and duration

#### Scenario: Snapshot load skips expired or corrupt entries
- **WHEN** snapshot restore encounters expired or corrupt entries
- **THEN** runtime observability SHALL expose skipped entry counts classified by reason

#### Scenario: Snapshot save failure is measurable
- **WHEN** `SaveSnapshot` fails because of filesystem or serialization errors
- **THEN** runtime observability SHALL record a failed save outcome without changing shutdown semantics

### Requirement: Upstream observability covers retries and degradation reasons

rec53 SHALL expose upstream observability metrics for timeout, bad Rcode, alternate-IP fallback, and Happy Eyeballs winner path so developers and operators can distinguish transport failures from upstream answer quality issues.

#### Scenario: Timeout and bad Rcode are recorded separately
- **WHEN** an upstream query times out or returns a bad Rcode such as `SERVFAIL` or `REFUSED`
- **THEN** runtime observability SHALL classify these as separate failure reasons

#### Scenario: Alternate upstream fallback is visible
- **WHEN** query execution falls back from the first upstream candidate to an alternate candidate
- **THEN** runtime observability SHALL record that a fallback occurred

#### Scenario: Happy Eyeballs winner path is visible
- **WHEN** both primary and secondary upstream candidates race and one wins
- **THEN** runtime observability SHALL expose which path won using a bounded path label

### Requirement: XDP observability covers synchronization and cleanup health

When XDP is enabled, rec53 SHALL expose XDP observability metrics beyond basic hit or miss totals, including cache sync failures, cleanup deletion activity, and cache occupancy or entry count signals.

#### Scenario: XDP sync failure is measurable
- **WHEN** inline sync from Go cache to BPF cache_map fails
- **THEN** runtime observability SHALL record a sync failure signal without changing Go cache correctness

#### Scenario: XDP cleanup activity is measurable
- **WHEN** expired BPF cache entries are deleted during periodic cleanup
- **THEN** runtime observability SHALL expose the number of deleted entries

#### Scenario: XDP occupancy is measurable
- **WHEN** operators inspect XDP cache health
- **THEN** runtime observability SHALL provide a signal that reflects current XDP cache occupancy or active entry count

### Requirement: State machine observability aggregates stage progress and failure reasons

rec53 SHALL expose state machine observability metrics that count stage transitions and aggregate terminal failure reasons using bounded categories, so operators can identify which phase most often contributes to failures.

#### Scenario: Stage transitions are measurable
- **WHEN** a query enters a state machine stage such as cache lookup, NS cache lookup, or upstream query
- **THEN** runtime observability SHALL increment a stage counter for that bounded stage name

#### Scenario: Terminal failures are classified
- **WHEN** the state machine returns an error or terminal failure condition
- **THEN** runtime observability SHALL classify the failure by bounded reason rather than only increasing a generic SERVFAIL counter
