# Tasks

## 1. Implementation

- [x] 1.1 Define a single shared upstream concurrency budget for outbound work and choose the first-version accounting rule for forwarding, Happy Eyeballs, and recursive NS resolution.
- [x] 1.2 Add acquire/release plumbing around the outbound upstream call sites so upstream work holds a slot only while it is actively consuming external capacity.
- [x] 1.3 Add the first soft-degradation step for gate saturation, reducing fanout before any fail-fast path is taken.
- [x] 1.4 Add bounded metrics and rate-limited logs for upstream gate pressure, including the current budget state and degradation outcome.

## 2. Validation

- [x] 2.1 Add focused tests that cover budget acquire/release, saturation behavior, and the first soft-degradation path.
- [x] 2.2 Add regression coverage for forwarding, Happy Eyeballs, and concurrent NS resolution so the new gate does not break existing successful upstream resolution paths.
- [x] 2.3 Update operator-facing docs and roadmap references so `v1.3.3` clearly states the upstream-pressure boundary and its relation to `v1.3.4`.
