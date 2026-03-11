# Task Management

## In Progress

(Empty - all current tasks moved to BACKLOG for planning phase)

## Backlog

### Feature Tasks (from BACKLOG.md)

- [ ] [F-003] IP Pool Maintenance Algorithm Improvement
  - [x] [F-003/1] Create `IPQualityV2` struct in server/ip_pool.go (Phase 1 foundation)
  - [x] [F-003/2] Implement `RecordLatency()` and `updatePercentiles()` in server/ip_pool.go (Phase 1)
  - [x] [F-003/3] Write unit tests for percentile calculations in server/ip_pool_test.go (Phase 1)
  - [x] [F-003/4] Implement `RecordFailure()` with exponential backoff in server/ip_pool.go (Phase 2)
  - [x] [F-003/5] Implement `ShouldProbe()` and `ResetForProbe()` in server/ip_pool.go (Phase 2)
  - [~] [F-003/6] Add background probe loop startup in server/ip_pool.go (Phase 2) — *deferred to Phase 4*
  - [ ] [F-003/7] Write integration tests for fault recovery in server/ip_pool_test.go (Phase 2)
  - [x] [F-003/8] Implement `GetScore()` method in server/ip_pool.go (Phase 3)
  - [x] [F-003/9] Implement `GetBestIPsV2()` method in server/ip_pool.go (Phase 3)
  - [x] [F-003/10] Write comparative tests for algorithm in server/ip_pool_test.go (Phase 3)
  - [ ] [F-003/7] Write integration tests for fault recovery in server/ip_pool_test.go (Phase 2)
  - [ ] [F-003/8] Implement `GetScore()` method in server/ip_pool.go (Phase 3)
  - [ ] [F-003/9] Implement `GetBestIPsV2()` method in server/ip_pool.go (Phase 3)
  - [ ] [F-003/10] Write comparative tests for algorithm in server/ip_pool_test.go (Phase 3)
  - [ ] [F-003/11] Migrate state_define.go to use GetBestIPsV2() (Phase 4)
  - [ ] [F-003/12] Add Prometheus metrics for p50/p95/p99 in monitor/metrics.go (Phase 4)
  - [ ] [F-003/13] Run performance benchmark for 1000 IPs in server/ip_pool_test.go (Phase 4)
  - [ ] [F-003/14] Add E2E integration tests in e2e/dns_test.go (Phase 4)
  - [ ] [F-003/15] Add feature flag support (optional) in server/config.go (Phase 4)

### Technical Debt

- [ ] [D-001] Add test cases for state machine (server/state_machine_test.go:29)

## Completed

- [x] [B-005] NS 递归解析栈溢出 crash (completed 2026-03-11)
  - Status: FIXED - resolveNSIPsRecursively() in state_define.go:289-323 handles NS resolution via state machine
  - Verified: Tests pass, no infinite recursion, proper error handling with depth limits
- [x] [T-001] 权威应答 E2E 测试覆盖 (completed 2026-03-11)
  - Step 1: Root hints injection (utils/root.go) — SetRootGlue/ResetRootGlue
  - Step 2: Iter port override + MultiZoneMockServer + test helpers
  - Step 3: 9 test scenarios in e2e/authority_test.go (7 pass, 2 skip for B-012)
- [x] [B-010] IN_GLUE_CACHE 返回错误码问题 (completed 2026-03-11)
- [x] [B-011] S0 无基本请求校验 FORMERR (completed 2026-03-11)
- [x] [B-004] CNAME with Valid NS Delegation Bug (completed 2026-03-10)
- [x] [B-003] CNAME Chain Resolution Bug (completed 2026-03-10)