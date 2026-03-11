# Task Management

## In Progress

<!-- No tasks currently in progress -->

## Backlog

### BUG

- [ ] [B-005] NS 递归解析栈溢出 crash (state_define.go:262-293)
  - Critical: 程序 crash
  - 修复后删除此条目

### Technical Debt

- [ ] [D-001] Add test cases for state machine (server/state_machine_test.go:29)

## Completed

- [x] [T-001] 权威应答 E2E 测试覆盖 (completed 2026-03-11)
  - Step 1: Root hints injection (utils/root.go) — SetRootGlue/ResetRootGlue
  - Step 2: Iter port override + MultiZoneMockServer + test helpers
  - Step 3: 9 test scenarios in e2e/authority_test.go (7 pass, 2 skip for B-012)
- [x] [B-010] IN_GLUE_CACHE 返回错误码问题 (completed 2026-03-11)
- [x] [B-011] S0 无基本请求校验 FORMERR (completed 2026-03-11)
- [x] [B-004] CNAME with Valid NS Delegation Bug (completed 2026-03-10)
- [x] [B-003] CNAME Chain Resolution Bug (completed 2026-03-10)