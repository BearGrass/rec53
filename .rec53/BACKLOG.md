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

## In Progress

## Planned

### [O-024] 并发查询NS的IP（快速回源）
Priority: Medium
Description: 当需要解析 NS 名字到 IP 时，并发查询多个 NS（最多 5 个），使用首个成功响应，后台更新缓存。加快回源速度，提升查询延迟。
Acceptance criteria:
- [ ] 实现并发查询 NS IPs 的辅助函数（最多 5 个并发）
- [ ] 首个成功响应立即返回，不阻塞查询
- [ ] 后台 goroutine 更新剩余 NS IPs 到缓存
- [ ] 无 goroutine 泄漏，context 正确取消
- [ ] 默认并发数 5（TODO：后续变为可配置参数）
- [ ] E2E 测试验证并发场景正确性
- [ ] 性能基准测试验证无回退

## Unplanned

### [B-014] Glue 无 bailiwick 校验（安全风险）
Priority: Medium
Description: getIPListFromResponse() 直接采信 Additional 中的所有 A 记录作为 NS 地址，未验证 glue 是否在 bailiwick 范围内，存在 DNS cache poisoning 隐患（doc S10 明确要求 out-of-bailiwick glue 不纳入可信缓存）。
Acceptance criteria:
- [ ] 提取 glue 时校验 A/AAAA 记录的名字是否在当前 zone 的 bailiwick 内
- [ ] out-of-bailiwick glue 触发 NS 子查询解析，而非直接使用
- [ ] 添加单元测试验证 bailiwick 校验逻辑

### [O-021] 无 glue 时委派 NS 不缓存
Priority: Medium
Description: ITER 仅在 `len(Ns)>0 && len(Extra)>0` 时才缓存委派信息，NS 无 glue（out-of-bailiwick）时委派 NS 完全不缓存，导致相同区域下次解析无法命中委派缓存，需重新从更上层迭代。
Acceptance criteria:
- [ ] NS-only 响应（无 Extra）也应缓存 NS RRset
- [ ] 下次解析同区域时能从缓存找到委派起点，跳过上层迭代

### [O-022] Response ID 未校验（S7）
Priority: Low
Description: ITER 只校验 response.Question[0].Name，未校验 response.ID 是否与发出的 query.ID 一致，存在乱序响应或伪造响应被误接受的风险（doc S7 要求 ID 不匹配时丢弃）。
Acceptance criteria:
- [ ] 校验 newResponse.Id == newQuery.Id，不一致时视为无效响应
- [ ] 添加单元测试验证 ID 校验

### [O-016] Add AAAA (IPv6) Support
Priority: High
Description: getIPListFromResponse() only extracts IPv4 (A) records, missing IPv6 (AAAA) support.
Acceptance criteria:
- [ ] Update getIPListFromResponse() to also extract AAAA records
- [ ] Update getBestAddressAndPrefetchIPs() to handle IPv6
- [ ] Test with AAAA queries

### [O-006] TCP Retry for Truncated Responses (RFC 1035)
Priority: High
Description: Implement TCP retry when UDP response is truncated (TC flag set).
Acceptance criteria:
- [ ] Detect TC flag in response
- [ ] Retry query via TCP when TC is set
- [ ] Handle larger responses via TCP

### [O-005] Implement Negative Caching (RFC 2308)
Priority: Medium
Description: Implement NXDOMAIN and NODATA response caching as required by RFC 2308.
Acceptance criteria:
- [ ] Cache NXDOMAIN responses with TTL from SOA minimum field
- [ ] Cache NODATA responses (success with empty Answer)
- [ ] Unit tests for negative caching scenarios

### [O-018] 状态机死循环保护增强
Priority: Medium
Description: 状态机当前使用 MaxIterations=50 限制迭代次数，需增强保护机制防止栈溢出。
Acceptance criteria:
- [ ] 添加 NS 解析递归深度限制 (如最大 10 层)
- [ ] 添加 delegation 深度跟踪
- [ ] 单元测试验证各种死循环场景

## Test Coverage Enhancement Tasks

根据 doc/example 中25个场景的测试覆盖分析（当前覆盖率64%），以下为补充测试开发计划。
详细分析见：`.rec53/TEST_COVERAGE_ANALYSIS.md`

### [T-002] Fix B-012: Enable NODATA/NXDOMAIN Tests (completed 2026-03-12)
Priority: High
Related issues: B-012 (completed)
Description: 移除 TestAuthorityNODATA 和 TestAuthorityNXDOMAIN 上的 SKIP 标记，通过修复状态机响应分类逻辑使其通过。对应 doc/example 中的例9和例10。
Acceptance criteria:
- [x] 分析 checkRespState.handle() 的响应分类逻辑
- [x] 修改 S9(CLASSIFY_RESPONSE) 正确识别情况C（负响应：Authority+SOA）
- [x] 确保 Authority+SOA 响应被路由到 S12(HANDLE_NEGATIVE) 而非 S10(HANDLE_DELEGATION)
- [x] 移除 TestAuthorityNODATA 和 TestAuthorityNXDOMAIN 的 SKIP 标记
- [x] 运行 go test ./e2e/... -v 验证两个测试通过
- [x] 运行完整测试套件验证无回归
Status: Completed with B-012 fix (2026-03-12)

### [T-003] Implement Negative Cache E2E Test
Priority: High
Related issues: O-005
Description: 实现缓存命中 negative cache 的 E2E 测试。验证 NXDOMAIN/NODATA 响应被缓存，后续相同查询直接从 negative cache 返回而无需上游查询。对应 doc/example 中的例2。
Location: e2e/cache_test.go
Acceptance criteria:
- [ ] 实现 TestNegativeCacheHit() 函数
- [ ] Mock 权威服务器返回 RCODE=NXDOMAIN + SOA(TTL=300)
- [ ] 验证第一次查询向上游发起
- [ ] 验证第二次相同查询直接从 negative cache 返回（无上游查询）
- [ ] 验证响应时间<100ms（缓存命中）
- [ ] 验证缓存过期后重新向上游查询

### [T-004] Implement Query Budget Exhaustion Test
Priority: High
Description: 测试当查询预算耗尽时（budget=0）触发 S15 失败处理。构造深层 NS 委派链强制逐次递减 budget，验证最终返回 SERVFAIL。对应 doc/example 中的例21。
Location: 新建 e2e/budget_test.go
Acceptance criteria:
- [ ] 构造深层 NS 委派链：. → com → example.com → sub.example.com → ...
- [ ] 每层都返回 NS referral，强制 budget 逐次递减
- [ ] 验证当 budget 接近 0 时返回 SERVFAIL
- [ ] 验证状态机进入 S15(FAIL_SERVFAIL) 状态
- [ ] 验证在合理时间内完成（防止真正的无限循环）

### [T-005] Implement Timeout Retry and Server Switch Test
Priority: High
Description: 扩展 TestQueryTimeout，实现超时重试和换服务器的完整流程。当一个 NS 超时后重试，失败后自动切换到备用 NS 并成功。对应 doc/example 中的例15。
Location: e2e/resolver_test.go 或 e2e/error_test.go
Acceptance criteria:
- [ ] 实现 TestTimeoutRetryAndServerSwitch() 函数
- [ ] 配置 NS 池中两个服务器：ns1（总是超时）、ns2（正常响应）
- [ ] 验证向 ns1 发查询超时
- [ ] 验证重试相同 NS 仍然超时
- [ ] 验证自动切换到 ns2 并成功返回
- [ ] 验证最终查询成功，总耗时<5秒

### [T-006] Implement SERVFAIL and Server Blacklist Test
Priority: High
Description: 测试上游返回 SERVFAIL 后自动标记服务器为 bad 并切换到备用服务器。对应 doc/example 中的例16 和 B-013。
Location: e2e/error_test.go
Acceptance criteria:
- [ ] 实现 TestServfailAndServerBlacklist() 函数
- [ ] Mock NS1 返回 RCODE=SERVFAIL
- [ ] Mock NS2 返回正常的 A 记录
- [ ] 验证状态机标记 NS1 为 bad_server
- [ ] 验证查询自动切换到 NS2
- [ ] 验证最终响应来自 NS2，不是 NS1

### [T-007] Implement Response ID Mismatch Test
Priority: Medium
Description: 测试当响应 ID 与查询 ID 不匹配时的处理。解析器应丢弃错误 ID 的响应并继续等待正确响应。对应 doc/example 中的例17 和 O-022。
Location: 新建 e2e/protocol_test.go
Acceptance criteria:
- [ ] 实现 TestResponseIDMismatch() 函数
- [ ] 发送查询（ID=0x1234）
- [ ] Mock 返回错误 ID 的响应（ID=0xABCD）
- [ ] 验证解析器丢弃该响应并继续等待
- [ ] Mock 再发送正确 ID 的响应
- [ ] 验证最终收到正确响应，查询成功

### [T-008] Implement CNAME + NXDOMAIN Combination Test
Priority: Medium
Description: 测试 CNAME 链与 NXDOMAIN 同时出现的组合场景。Answer 段含 CNAME，但 RCODE=NXDOMAIN，Authority 含 SOA。对应 doc/example 中的例13。
Location: e2e/resolver_test.go
Acceptance criteria:
- [ ] 实现 TestCNAMENXDomainResponse() 函数
- [ ] Mock 权威返回：Answer 含 CNAME，RCODE=NXDOMAIN，Authority 含 SOA
- [ ] 验证 Answer 段保留完整 CNAME 链
- [ ] 验证 RCODE=NXDOMAIN
- [ ] 验证 Authority 段含 SOA 记录

### [T-009] Implement Referral Loop Detection Test
Priority: Medium
Description: 测试委派循环检测。当查询进入循环委派（例如 example.com NS 返回 referral 到 example.com 自己）时，应检测循环并返回 SERVFAIL。对应 doc/example 中的例20。
Location: e2e/error_test.go
Acceptance criteria:
- [ ] 实现 TestReferralLoop() 函数
- [ ] 构造循环委派：查询 x.example.com/A，example.com NS 返回 referral 到 example.com 自己
- [ ] 验证状态机检测循环（referral_history 中检测到重复签名）
- [ ] 验证返回 SERVFAIL
- [ ] 验证总查询次数<10（防止真正的无限循环）

### [T-010] Implement All NS Unreachable Test
Priority: Medium
Description: 测试当所有 NS 都不可达时的处理。某区域有多个 NS，全部超时或返回 REFUSED，无更多可选服务器时应返回 SERVFAIL。对应 doc/example 中的例23。
Location: e2e/error_test.go
Acceptance criteria:
- [ ] 实现 TestAllNSUnreachable() 函数
- [ ] 配置区域有两个 NS：NS1（全部查询超时）、NS2（全部返回 REFUSED）
- [ ] 验证 NS1 重试用尽后切换到 NS2
- [ ] 验证 NS2 也失败
- [ ] 验证状态机进入 S15(FAIL_SERVFAIL)
- [ ] 验证返回 SERVFAIL 给客户端



## Completed

### [B-013] 上游返回 SERVFAIL / REFUSED 不换服务器重试 (completed 2026-03-12)
Priority: Medium
Description: ITER 收到 SERVFAIL、REFUSED、FORMERR、NOTIMPL 时直接返回 ITER_COMMON_ERROR，未标记 bad server 并换其他 NS 重试，单个故障服务器即导致查询失败（doc S7 要求换 server）。

**Completion Summary**:
- Detected bad Rcodes (SERVFAIL, REFUSED, FORMERR, NOTIMPL) in iterState.handle()
- Added failure tracking via globalIPPool.GetIPQualityV2(ip).RecordFailure()
- Implemented retry logic with secondary IP for bad Rcodes
- Records latency metrics from successful secondary IP retries
- Returns SERVFAIL only if both primary and secondary IPs fail
- Added TestBadRcodeDetection E2E test in e2e/error_test.go
- All tests pass (no new regressions)

Acceptance criteria:
- [x] 上述 Rcode 时标记当前服务器为 bad 并尝试备用服务器
- [x] 所有可用服务器均失败后才返回 SERVFAIL

### [B-017] NS 递归解析栈溢出：遗漏 break 导致后续崩溃 (completed 2026-03-11)
Priority: Critical
Description: www.qq.com 查询能正确返回结果给客户端，但之后 rec53 进程因栈溢出而崩溃。根本原因：resolveNSIPsRecursively() 函数在成功解析一个 NS 的 A 记录后，仍继续循环遍历剩余的 NS 名字，导致不必要的深层递归调用。

Fix Summary:
- server/state_define.go:319 添加 break 语句，在成功解析首个 NS IP 后立即返回
- 编译测试通过：go build -o rec53 ./cmd
- 单元测试全部通过：go test -short ./server/...
- E2E 测试验证：e2e/glue_recursion_test.go 通过
- 回归测试：所有现有测试无破坏

### [T-001] 权威应答 E2E 测试覆盖 (completed 2026-03-11)
Priority: High
Description: 构建全面的 E2E 测试，模拟各种权威 DNS 服务器响应场景，验证状态机正确处理各类响应。
Implementation:
- utils/root.go: SetRootGlue/ResetRootGlue root hints injection
- server/state_define.go: SetIterPort/ResetIterPort for mock server port override
- server/cache.go: FlushCacheForTest(), server/ip_pool.go: ResetIPPoolForTest()
- e2e/helpers.go: MultiZoneMockServer, MockDNSHierarchy, setupResolverWithMockRoot, BuildStandardHierarchy
- e2e/authority_test.go: 9 test scenarios (7 pass, 2 skip pending B-012)
Acceptance criteria:
- [x] utils/root.go 新增 root hints 注入接口 (SetRootGlue/ResetRootGlue)
- [x] e2e/helpers.go 新增多层 mock DNS 层级 helper
- [x] 创建 e2e/authority_test.go 实现核心 9 个测试场景
- [x] 场景 5/6 标记 skip (依赖 B-012)
- [x] 集成到 CI (go test ./e2e/...)

### [B-011] S0 无基本请求校验（FORMERR） (completed 2026-03-11)
Priority: High
Description: STATE_INIT 未校验 QDCOUNT=1、QR=0、OPCODE=QUERY，畸形查询直接进入解析流程而非返回 FORMERR（违反 RFC 1035 Section 4.1.1）。
Acceptance criteria:
- [x] stateInitState.handle() 校验 QDCOUNT、QR、Opcode
- [x] 不通过校验时直接返回 FORMERR 响应
- [x] 添加单元测试覆盖畸形查询场景

### [B-010] IN_GLUE_CACHE 返回错误码问题
Priority: Low
Description: inGlueCacheState.handle() 返回错误的常量值。
Location: state_define.go:485
Acceptance criteria:
- [x] 修正返回值为 IN_GLUE_CACHE_MISS_CACHE

### [B-005] NS 递归解析栈溢出 Crash (completed 2026-03-11)
Priority: Critical
Description: 当请求 baidu.cc 等域名时，程序会 crash 并显示栈溢出错误。

Root Cause: resolveNSIPsRecursively() 函数在解析 NS 域名的 A 记录时会递归调用 Change() 状态机，导致无限递归。

Acceptance criteria:
- [x] 修复无限递归问题
- [x] 添加 NS 解析缓存机制
- [x] 添加 NS 递归深度限制
- [x] 请求 baidu.cc 不再 crash

### [B-004] CNAME with Valid NS Delegation Bug (completed 2026-03-10)
Priority: High
Description: When querying www.huawei.com, the resolver returns SERVFAIL instead of following the CNAME chain.

Acceptance criteria:
- [x] Query CNAME chain domains returns complete chain and final A records
- [x] NS delegation for CNAME target's zone is preserved and used

### [B-003] CNAME Chain Resolution Bug (completed 2026-03-10)
Priority: High
Description: When querying a domain with CNAME chain, the resolver returns SERVFAIL instead of following the CNAME chain.

Acceptance criteria:
- [x] Query CNAME chain domains returns complete chain and final A records
- [x] No regression on normal queries

### [B-012] NXDOMAIN / NODATA 响应码不传递给客户端 (completed 2026-03-12)
Priority: High
Description: ITER 收到 NXDOMAIN 后设置 response.Rcode，但随后 CHECK_RESP 发现 Answer 为空继续迭代 IN_GLUE，导致所有 NXDOMAIN 和 NODATA 最终以 SERVFAIL 返回客户端。需要修改状态机正确识别和缓存负响应。

**Completion Summary**:
- Added CHECK_RESP_GET_NEGATIVE state constant in server/state.go
- Implemented extractSOAFromAuthority() and hasSOAInAuthority() helpers in server/state_define.go
- Added DefaultNegativeCacheTTL constant (60s) with TODO for future configuration
- Modified checkRespState.handle() to detect negative responses (NXDOMAIN and NODATA)
- Implemented smart negative caching with SOA-based TTL (fallback to 60s)
- Updated state_machine.go to handle CHECK_RESP_GET_NEGATIVE case
- Enabled TestAuthorityNXDOMAIN and TestAuthorityNODATA in e2e/authority_test.go
- All acceptance criteria met ✅

Acceptance criteria:
- [x] ITER 收到 NXDOMAIN 时，CHECK_RESP / 状态机能识别并直接组装负响应
- [x] RCODE=NOERROR + 空 Answer + Authority 含 SOA 的 NODATA 场景正确返回 NOERROR+空Answer
- [x] E2E 测试验证 NXDOMAIN 和 NODATA 正确到达客户端

Related tasks:
- [x] T-002: Fix B-012: Enable NODATA/NXDOMAIN Tests (completed 2026-03-12)

### [F-003] IP Pool Maintenance Algorithm Improvement (completed 2026-03-12)
Priority: High
Description: Implement sliding window histogram-based IP pool quality tracking with automatic fault recovery. Current algorithm lacks fault recovery (IPs marked MAX_LATENCY never recover), lacks confidence-based selection, and has no exponential backoff. This leads to permanent performance degradation from transient network faults. Proposed solution uses 64-sample ring buffer with P50/P95/P99 metrics, exponential backoff for failures, and periodic background probing for recovery.

**Completion Summary**: All 4 phases complete. Phase 2 & Phase 4 delivered:
- Background probe loop: StartProbeLoop(), periodicProbeLoop(), probeAllSuspiciousIPs() ✅
- Integration tests (F-003/7): 9 comprehensive tests in server/ip_pool_integration_test.go covering full fault recovery lifecycle ✅
- Migration: state_define.go updated to GetBestIPsV2() with latency/failure recording ✅
- Prometheus metrics: P50/P95/P99 latency export via IPQualityV2GaugeSet() ✅
- Performance: Benchmark verified (94-98 µs for 1000 IPs, 10x under 1ms target) ✅
- E2E tests: 8 comprehensive E2E tests (latency, scoring, recovery, metrics, concurrency, confidence, throttling) ✅

Implementation Phases:
- `Phase 1` ✅ Complete: IPQualityV2 struct with ring buffer, percentile calculations (8 tests)
- `Phase 2` ✅ Complete: Fault handling with exponential backoff, ShouldProbe/ResetForProbe (8 tests + 9 integration tests)
- `Phase 3` ✅ Complete: Composite scoring with GetScore() and GetBestIPsV2() (8 tests)
- `Phase 4` ✅ Complete: Integration, metrics, benchmarks, background probing (5 tests)

Overall Success Criteria Met:
- ✅ Fault recovery time: 30-60 seconds for SUSPECT IPs (via periodicProbeLoop)
- ✅ Unit test coverage: 65+ tests in server/ip_pool_test.go, 9 in server/ip_pool_integration_test.go, 12 in monitor/metric_test.go
- ✅ Performance: 94-98 µs per IP selection for 1000 IPs (10x under 1ms target)
- ✅ E2E integration: 8 comprehensive tests covering full resolution flow
- ✅ Integration tests (F-003/7): 9 tests covering fault recovery lifecycle (ACTIVE→DEGRADED→SUSPECT→RECOVERED→ACTIVE)
- ✅ Prometheus metrics: P50/P95/P99 latency per IP, updated via RecordLatency()
- ✅ Zero regressions: All existing tests pass, no functionality degraded
- Skipped: F-003/15 (feature flag support) - deemed optional for production

### [B-017] NS 递归解析栈溢出（fixed 2026-03-11）
Priority: Critical
Description: www.qq.com 查询能正确返回结果给客户端，但之后 rec53 进程因栈溢出而崩溃。根本原因：resolveNSIPsRecursively() 函数在成功解析一个 NS 的 A 记录后，仍继续循环遍历剩余的 NS 名字，导致不必要的深层递归调用。

Fix Summary:
- server/state_define.go:319 添加 break 语句，在成功解析首个 NS IP 后立即返回
- 编译测试通过：go build -o rec53 ./cmd
- 单元测试全部通过：go test -short ./server/...
- E2E 测试验证：e2e/glue_recursion_test.go 通过
- 回归测试：所有现有测试无破坏

### [T-001] 权威应答 E2E 测试覆盖 (completed 2026-03-11)
Priority: High
Description: 构建全面的 E2E 测试，模拟各种权威 DNS 服务器响应场景，验证状态机正确处理各类响应。
Implementation:
- utils/root.go: SetRootGlue/ResetRootGlue root hints injection
- server/state_define.go: SetIterPort/ResetIterPort for mock server port override
- server/cache.go: FlushCacheForTest(), server/ip_pool.go: ResetIPPoolForTest()
- e2e/helpers.go: MultiZoneMockServer, MockDNSHierarchy, setupResolverWithMockRoot, BuildStandardHierarchy
- e2e/authority_test.go: 9 test scenarios (7 pass, 2 skip pending B-012)
Acceptance criteria:
- [x] utils/root.go 新增 root hints 注入接口 (SetRootGlue/ResetRootGlue)
- [x] e2e/helpers.go 新增多层 mock DNS 层级 helper
- [x] 创建 e2e/authority_test.go 实现核心 9 个测试场景
- [x] 场景 5/6 标记 skip (依赖 B-012)
- [x] 集成到 CI (go test ./e2e/...)

### [B-011] S0 无基本请求校验（FORMERR） (completed 2026-03-11)
Priority: High
Description: STATE_INIT 未校验 QDCOUNT=1、QR=0、OPCODE=QUERY，畸形查询直接进入解析流程而非返回 FORMERR（违反 RFC 1035 Section 4.1.1）。
Acceptance criteria:
- [x] stateInitState.handle() 校验 QDCOUNT、QR、Opcode
- [x] 不通过校验时直接返回 FORMERR 响应
- [x] 添加单元测试覆盖畸形查询场景

### [B-010] IN_GLUE_CACHE 返回错误码问题 (completed 2026-03-11)
Priority: Low
Description: inGlueCacheState.handle() 返回错误的常量值。
Location: state_define.go:485
Acceptance criteria:
- [x] 修正返回值为 IN_GLUE_CACHE_MISS_CACHE

### [B-005] NS 递归解析栈溢出 Crash (completed 2026-03-11)
Priority: Critical
Description: 当请求 baidu.cc 等域名时，程序会 crash 并显示栈溢出错误。

Root Cause: resolveNSIPsRecursively() 函数在解析 NS 域名的 A 记录时会递归调用 Change() 状态机，导致无限递归。

Acceptance criteria:
- [x] 修复无限递归问题
- [x] 添加 NS 解析缓存机制
- [x] 添加 NS 递归深度限制
- [x] 请求 baidu.cc 不再 crash

### [B-004] CNAME with Valid NS Delegation Bug (completed 2026-03-10)
Priority: High
Description: When querying www.huawei.com, the resolver returns SERVFAIL instead of following the CNAME chain.

Acceptance criteria:
- [x] Query CNAME chain domains returns complete chain and final A records
- [x] NS delegation for CNAME target's zone is preserved and used

### [B-003] CNAME Chain Resolution Bug (completed 2026-03-10)
Priority: High
Description: When querying a domain with CNAME chain, the resolver returns SERVFAIL instead of following the CNAME chain.

Acceptance criteria:
- [x] Query CNAME chain domains returns complete chain and final A records
- [x] No regression on normal queries

---

## 已删除的低价值项目

以下项目已从 backlog 中移除，因为价值低或非必要：

- O-012 ~ O-015: 代码重构 (当前代码可工作，重构引入风险)
- O-017: Cache API 统一 (不影响功能)
- O-007: CNAME + 其他记录共存 (RFC 违规响应罕见)
- O-008: Authority Section NS 处理 (已有 fallback)
- O-009: QNAME Minimization (隐私优化，复杂度高收益小)
- O-010: Iteration Depth Limiting (MaxIterations 已足够)
- O-011: TTL Upper Bound (几乎无实际需求)
- O-019: DNAME 支持 (几乎无人使用)
- O-020: SVCB/HTTPS 支持 (新技术，支持者少)
- B-006, B-007, B-008, B-009: 潜在问题 (实际影响小，可后续观察)