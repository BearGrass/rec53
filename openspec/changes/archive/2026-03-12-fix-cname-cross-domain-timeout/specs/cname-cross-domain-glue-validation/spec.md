## ADDED Requirements

### Requirement: IN_GLUE state validates NS zone relevance before accepting glue
`inGlueState.handle` SHALL verify that `response.Ns[0].Header().Name` (the NS zone) is an
ancestor of or equal to `request.Question[0].Name` (the current query domain) before
returning `IN_GLUE_EXIST`. If the NS zone is unrelated to the query domain, the state SHALL
clear `response.Ns` and `response.Extra` and return `IN_GLUE_NOT_EXIST`.

Relevance is determined using `dns.IsSubDomain(nsZone, queryName)`, which returns true when
`queryName` is a subdomain of (or equal to) `nsZone`.

#### Scenario: NS zone matches query domain ancestry
- **WHEN** `response.Ns` is non-empty, `response.Extra` is non-empty, and `response.Ns[0].Header().Name` is an ancestor of `request.Question[0].Name`
- **THEN** `inGlueState.handle` returns `IN_GLUE_EXIST` without modifying `response`

#### Scenario: NS zone is unrelated to query domain
- **WHEN** `response.Ns` is non-empty, `response.Extra` is non-empty, and `response.Ns[0].Header().Name` is NOT an ancestor of `request.Question[0].Name`
- **THEN** `inGlueState.handle` clears `response.Ns` and `response.Extra` and returns `IN_GLUE_NOT_EXIST`

#### Scenario: NS zone is root (universal ancestor)
- **WHEN** `response.Ns[0].Header().Name` is `"."` (root zone)
- **THEN** `inGlueState.handle` returns `IN_GLUE_EXIST` (root is always relevant)

#### Scenario: response.Ns is empty
- **WHEN** `response.Ns` is empty
- **THEN** `inGlueState.handle` returns `IN_GLUE_NOT_EXIST` (existing behavior, unchanged)

### Requirement: Multi-hop cross-domain CNAME resolves successfully on first cold-cache query
The resolver SHALL successfully resolve a domain that requires following a CNAME chain
crossing at least two different parent domains (e.g., A.com → B.net → C.com) on the first
query, even with a completely cold cache. The resolution SHALL complete without returning
SERVFAIL due to stale NS glue from a prior CNAME hop.

#### Scenario: Three-hop cross-domain CNAME resolves on first attempt
- **WHEN** a query is made for a domain whose answer requires following CNAMEs across three different registrable domains (domain1 → CNAME → domain2 → CNAME → domain3 → A record)
- **THEN** the resolver returns the correct final A records without SERVFAIL and without requiring a retry

#### Scenario: CNAME hop stays within same delegated zone
- **WHEN** a CNAME target is within the same delegated zone as the NS records in `response` (e.g., `foo.akadns.net → bar.akadns.net`)
- **THEN** the existing NS glue is preserved and reused (no unnecessary re-delegation)
