## ADDED Requirements

### Requirement: resolver SHALL emit canonical state-machine transition metrics
The resolver SHALL publish a Prometheus counter metric named `rec53_state_machine_transition_total` with bounded `from` and `to` labels that represent the actual state-machine edge taken by a request.

#### Scenario: normal state transition is recorded
- **WHEN** the resolver moves from one canonical state to another inside the main state-machine loop
- **THEN** it SHALL increment `rec53_state_machine_transition_total` once for that exact `from -> to` edge

#### Scenario: labels stay bounded and canonical
- **WHEN** the resolver emits a transition metric
- **THEN** the `from` and `to` labels SHALL use only canonical state or terminal-node names defined by the implementation
- **AND** SHALL NOT include query name, qtype, domain, upstream IP, or other unbounded values

### Requirement: resolver SHALL model terminal outcomes as transition edges
The resolver SHALL represent request termination in the transition metric by incrementing edges from the current canonical state to bounded terminal nodes such as success, FORMERR, SERVFAIL, generic error, or max-iteration protection exits.

#### Scenario: successful response path ends with an explicit terminal edge
- **WHEN** the resolver completes a request and returns a final response successfully
- **THEN** it SHALL record a terminal transition edge that makes the successful end of the path observable in metrics

#### Scenario: early return path ends with an explicit terminal edge
- **WHEN** the resolver returns early because of FORMERR, SERVFAIL, handler error, or max-iteration protection
- **THEN** it SHALL record a terminal transition edge for that outcome before returning

#### Scenario: generic error exit stays bounded
- **WHEN** the resolver encounters handler or internal error paths that do not warrant distinct transition labels
- **THEN** it SHALL record a bounded `error_exit` style terminal edge
- **AND** SHALL keep any finer-grained error reason detail in the existing failure-reason metrics or logs rather than in `from` / `to` labels

### Requirement: transition metrics SHALL cover loop-back and branch edges
The transition metric SHALL include branch-specific edges that materially change the request path, including CNAME loop-back behavior and iterative-resolution branch points.

#### Scenario: CNAME path records loop-back edge
- **WHEN** `CLASSIFY_RESP` identifies a CNAME and the resolver loops back to continue resolution
- **THEN** the emitted transition metrics SHALL include the loop-back edge actually taken by that request

#### Scenario: delegation path records its real next hop
- **WHEN** the resolver branches from glue extraction to nameserver cache lookup or upstream query
- **THEN** the emitted transition metrics SHALL record the exact branch that occurred
