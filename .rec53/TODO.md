# Task Management

## In Progress

<!-- No tasks currently in progress -->

## Backlog

### BUG
<!-- Format: - [ ] [B-xxx] description (file:line) -->
- [ ] [B-001] E2E TestCacheBehavior timeout on second query (e2e/cache_test.go:74)
- [ ] [B-002] E2E TestResolverIntegration NS record returns SERVFAIL (e2e/resolver_test.go:99)

### Optimization
<!-- Format: - [ ] [O-xxx] description -->

### Technical Debt
<!-- Format: - [ ] [D-xxx] description (source) -->
- [ ] [D-001] Add test cases for state machine (server/state_machine_test.go:29)

## Completed
<!-- Move completed items here with completion date -->
<!-- Format: - [x] [B-001] description (completed YYYY-MM-DD) -->
- [x] [B-004] CNAME with Valid NS Delegation Bug (completed 2026-03-10)
  - Added `isNSRelevantForCNAME()` helper function using `dns.IsSubDomain()`
  - Modified CHECK_RESP_GET_CNAME handler to conditionally preserve NS/Extra
  - Unit tests pass: TestIsNSRelevantForCNAME, TestCNAMEChain_ValidNSDelegation, TestCNAMEChain_StaleNSDelegation
  - E2E tests for real domains may timeout due to network issues
- [x] [B-003] CNAME Chain Resolution Bug (completed 2026-03-10)
  - Fix was already in place at state_machine.go:99-102
  - Added comprehensive unit tests in state_machine_test.go
  - Tests: TestCNAMEChain_ClearStaleRecords, TestCNAMEChain_CrossZoneResolution, TestCNAMEChain_MultiLevelResolution, TestCNAMEChain_TTLPreservation