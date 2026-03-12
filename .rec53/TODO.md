# Task Management

## In Progress

## Backlog

### Bug Fixes (from BACKLOG.md)

- [ ] [B-013] 上游返回 SERVFAIL / REFUSED 不换服务器重试
  - [ ] [B-013/1] Modify server/state_define.go — detect SERVFAIL/REFUSED/FORMERR/NOTIMPL response codes in iterState.handle()
  - [ ] [B-013/2] Modify server/state_define.go — add failure tracking and server-switch logic in ITER response handling
  - [ ] [B-013/3] Modify server/state_define.go — implement retry with secondary IP for bad Rcodes
  - [ ] [B-013/4] Update server/state_machine.go — ensure state machine handles ITER retries correctly
  - [ ] [B-013/5] Create e2e/error_test.go or extend existing — TestServfailAndServerSwitch test
  - [ ] [B-013/6] Run tests — verify go test ./e2e/... passes
  - [ ] [B-013/7] Run tests — verify go test ./server/... passes
  - [ ] [B-013/8] Run full test suite — verify go test -race ./... passes

### Feature Tasks (from BACKLOG.md)

### Technical Debt

- [ ] [D-001] Add test cases for state machine (server/state_machine_test.go:29)

## Completed

- [x] [F-003] IP Pool Maintenance Algorithm Improvement (completed 2026-03-12)
  - [x] [F-003/1] Create `IPQualityV2` struct in server/ip_pool.go (Phase 1 foundation)
  - [x] [F-003/2] Implement `RecordLatency()` and `updatePercentiles()` in server/ip_pool.go (Phase 1)
  - [x] [F-003/3] Write unit tests for percentile calculations in server/ip_pool_test.go (Phase 1)
  - [x] [F-003/4] Implement `RecordFailure()` with exponential backoff in server/ip_pool.go (Phase 2)
  - [x] [F-003/5] Implement `ShouldProbe()` and `ResetForProbe()` in server/ip_pool.go (Phase 2)
  - [x] [F-003/7] Write integration tests for fault recovery in server/ip_pool_integration_test.go (Phase 2, 9 tests)
  - [x] [F-003/8] Implement `GetScore()` method in server/ip_pool.go (Phase 3)
  - [x] [F-003/9] Implement `GetBestIPsV2()` method in server/ip_pool.go (Phase 3)
  - [x] [F-003/10] Write comparative tests for algorithm in server/ip_pool_test.go (Phase 3)
  - [x] [F-003/6] Add background probe loop startup in server/ip_pool.go (Phase 4)
    - [x] [F-003/6a] Implement `StartProbeLoop()` method in server/ip_pool.go
    - [x] [F-003/6b] Implement `periodicProbeLoop()` method in server/ip_pool.go
    - [x] [F-003/6c] Implement `probeAllSuspiciousIPs()` method in server/ip_pool.go
    - [x] [F-003/6d] Integrate probe loop startup in server/server.go Run() method
    - [x] [F-003/6e] Write unit tests for probe loop in server/ip_pool_test.go
  - [x] [F-003/11] Migrate state_define.go to use GetBestIPsV2() (Phase 4)
  - [x] [F-003/12] Add Prometheus metrics for p50/p95/p99 in monitor/metrics.go (Phase 4)
  - [x] [F-003/13] Run performance benchmark for 1000 IPs in server/ip_pool_test.go (Phase 4)
  - [x] [F-003/14] Add E2E integration tests in e2e/dns_test.go (Phase 4)
  - Skipped: [F-003/15] Feature flag support (optional)
- [x] [B-012] NXDOMAIN / NODATA 响应码不传递给客户端 (completed 2026-03-12)
  - [x] [B-012/1] Update server/state.go — add CHECK_RESP_GET_NEGATIVE constant
  - [x] [B-012/2] Update server/state_define.go — add DefaultNegativeCacheTTL constant (60s) with TODO for future configuration
  - [x] [B-012/3] Update server/state_define.go — add extractSOAFromAuthority() helper function
  - [x] [B-012/4] Update server/state_define.go — add hasSOAInAuthority() helper function
  - [x] [B-012/5] Update server/state_define.go — modify checkRespState.handle() to detect negative responses with smart caching
  - [x] [B-012/6] Update server/state_machine.go — add CHECK_RESP_GET_NEGATIVE case in Change() function
  - [x] [B-012/7] Update e2e/authority_test.go — remove t.Skip() from TestAuthorityNXDOMAIN (line 333)
  - [x] [B-012/8] Update e2e/authority_test.go — remove t.Skip() from TestAuthorityNODATA (line 375)
  - [x] [B-012/9] Run tests — verify go test ./e2e/... -v passes
  - [x] [B-012/10] Run tests — verify go test ./server/... -v passes
- [x] [B-017] NS 递归解析栈溢出（fixed 2026-03-11）
  - Fix: Added break statement in resolveNSIPsRecursively() at line 319
  - Verification: E2E test pass, all regression tests pass
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