# Requirement Backlog

## Template

Use this format for each requirement:

> ### [F-xxx] Title
> Priority: High / Medium / Low
> Description: What is needed in 1-2 sentences
> Acceptance criteria:
> - Criterion 1
> - Criterion 2

Use these prefixes:
- `[F-xxx]` for features
- `[B-xxx]` for bugs
- `[O-xxx]` for optimizations

## Planned

<!-- No items currently planned -->

## Completed

### [B-004] CNAME with Valid NS Delegation Bug (completed 2026-03-10)
Priority: High
Description: When querying `www.huawei.com`, the resolver returns SERVFAIL instead of following the CNAME chain. The upstream server returns CNAME + valid NS delegation for the CNAME target's zone (`akadns.net`), but the previous fix (B-003) cleared ALL NS/Extra records, losing useful delegation info.

Root Cause: The fix at `state_machine.go:99-102` was too aggressive - it cleared NS/Extra even when they contained valid delegation info for the CNAME target's zone.

Fix: Added `isNSRelevantForCNAME()` helper function using `dns.IsSubDomain()` to check if NS zone matches or is parent of CNAME target. Modified CNAME handler to conditionally preserve NS/Extra.

Acceptance criteria:
- [x] Query CNAME chain domains returns complete chain and final A records
- [x] NS delegation for CNAME target's zone is preserved and used
- [x] No regression on cross-zone CNAME tests (B-003)
- [x] Unit tests for CNAME + valid delegation scenario
- [x] E2E test for `www.huawei.com` or similar CNAME chain

## Completed

### [B-003] CNAME Chain Resolution Bug (completed 2026-03-10)
Priority: High
Description: When querying a domain with CNAME chain (e.g., www.huawei.com), the resolver returns SERVFAIL instead of following the CNAME chain to get final A records.

Root Cause: In `CHECK_RESP_GET_CNAME` state transition, the response still contains stale NS/Extra records from the previous zone after updating the query name to the CNAME target. This causes the resolver to query wrong nameservers.

Acceptance criteria:
- [x] Query CNAME chain domains returns complete chain and final A records
- [x] TTL handling is correct (preserve individual TTLs from each record)
- [x] No regression on normal queries
- [x] E2E tests for CNAME chain scenarios

Implementation:
- Fix already in place at `state_machine.go:99-102`: Clear `response.Ns` and `response.Extra` before transitioning to `IN_CACHE`
- Added unit tests in `server/state_machine_test.go` for CNAME chain scenarios

### 启动报错 (completed 2026-03-10)
优先级： 高
描述：执行 `go build -o rec53 cmd/rec53.go` 时出现错误：
```bash
# command-line-arguments
cmd/rec53.go:70:22: undefined: parseLogLevel
```
**Root Cause**: Single-file build excludes other package files. Use `go build -o rec53 ./cmd` instead.
**Fix**: Updated CLAUDE.md and README.md with correct build command.