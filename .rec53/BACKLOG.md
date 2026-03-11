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

### [F-003] IP Pool Maintenance Algorithm Improvement
Priority: High
Description: Implement sliding window histogram-based IP pool quality tracking with automatic fault recovery. Current algorithm lacks fault recovery (IPs marked MAX_LATENCY never recover), lacks confidence-based selection, and has no exponential backoff. This leads to permanent performance degradation from transient network faults. Proposed solution uses 64-sample ring buffer with P50/P95/P99 metrics, exponential backoff for failures, and periodic background probing for recovery.

Design & Roadmap:
- `.rec53/IP_POOL_DESIGN.md` — Technical design (data structures, algorithms, concurrency strategy)
- `.rec53/IP_POOL_ROADMAP.md` — Implementation roadmap (4 phases, 15.5 days, risk mitigation)

Acceptance criteria:
- [ ] Phase 1: `IPQualityV2` struct with 64-sample ring buffer and percentile calculations
  - `RecordLatency()`, `updatePercentiles()` methods implemented
  - Unit tests: 12+ test cases for percentile accuracy, boundary conditions
  - Integration test: simulate realistic latency distributions
- [ ] Phase 2: Fault handling with exponential backoff and auto-recovery
  - `RecordFailure()` implements: DEGRADED (1-3 failures) → SUSPECT (4-6) → RECOVERED (7+ auto-probe)
  - `ShouldProbe()`, `ResetForProbe()` for periodic recovery probing
  - Background probe loop: every 30 seconds, context-based shutdown
  - Integration test: verify recovery time < 5 seconds for transient faults
  - Concurrency verified with RWMutex + atomic operations
- [ ] Phase 3: Composite scoring and intelligent selection
  - `GetScore()` = p50 × confidenceMultiplier × stateWeight
  - `GetBestIPsV2()` returns top 2 IPs based on composite scores
  - Comparative testing: new algorithm vs old on 100 IPs with various fault scenarios
- [ ] Phase 4: Integration and monitoring
  - Migrate `state_define.go` to use GetBestIPsV2() instead of getBestIPs
  - Prometheus metrics: `rec53_ip_p50_latency_ms`, `rec53_ip_p95_latency_ms`, `rec53_ip_p99_latency_ms` gauges
  - Performance benchmark: 1000 IPs selection time < 1ms
  - E2E test: full DNS query flow with IP pool selection
  - Optional: feature flag for A/B testing old vs new algorithm

Success criteria:
- Fault recovery time: 3-5 seconds (vs current infinite)
- P99 latency: > 10% improvement
- Unit test coverage: > 80%
- Performance: < 1ms per IP selection for 1000 IPs
- 0 production rollbacks
- No increase in monitoring alerts

### [B-012] NXDOMAIN / NODATA 响应码不传递给客户端
Priority: High
Description: ITER 收到 NXDOMAIN 后设置 response.Rcode，但随后 CHECK_RESP 发现 Answer 为空继续迭代 IN_GLUE，导致所有 NXDOMAIN 和 NODATA 最终以 SERVFAIL 返回客户端，与 O-005（缓存）是两个独立问题。
Acceptance criteria:
- [ ] ITER 收到 NXDOMAIN 时，CHECK_RESP / 状态机能识别并直接组装负响应
- [ ] RCODE=NOERROR + 空 Answer + Authority 含 SOA 的 NODATA 场景正确返回 NOERROR+空Answer
- [ ] E2E 测试验证 NXDOMAIN 和 NODATA 正确到达客户端

### [B-013] 上游返回 SERVFAIL / REFUSED 不换服务器重试
Priority: Medium
Description: ITER 收到 SERVFAIL、REFUSED、FORMERR、NOTIMPL 时直接返回 ITER_COMMON_ERROR，未标记 bad server 并换其他 NS 重试，单个故障服务器即导致查询失败（doc S7 要求换 server）。
Acceptance criteria:
- [ ] 上述 Rcode 时标记当前服务器为 bad 并尝试备用服务器
- [ ] 所有可用服务器均失败后才返回 SERVFAIL

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

### [B-017] NS 递归解析栈溢出：遗漏 break 导致后续崩溃
Priority: Critical
Description: www.qq.com 查询能正确返回结果给客户端，但之后 rec53 进程因栈溢出而崩溃。根本原因：resolveNSIPsRecursively() 函数在成功解析一个 NS 的 A 记录后，仍继续循环遍历剩余的 NS 名字，导致不必要的深层递归调用。

问题分析：
- dig 查询 www.qq.com 首次超时→重试→再次超时→第三次成功返回结果（80ms）
- 结果正确：3 条 CNAME/A 记录正确到达客户端
- 但几秒后 rec53 进程因栈溢出而崩溃
- 日志显示：resolveNSIPsRecursively() 在行 295-320 的 for 循环从未执行 break

根本原因详解：
1. www.qq.com 查询需要 qq.com 的 NS 信息
2. qq.com 的 NS 记录（ns3.qq.com、ns4.qq.com、ns1.qq.com、ns2.qq.com）没有 glue 记录
3. iterState.handle() 第 374 行调用 resolveNSIPsRecursively() 来递归解析这些 NS 的 A 记录
4. 第一个 NS (ns3.qq.com) 的解析成功，ipList 获得 IP 地址
5. ❌ 但函数 311-315 行只是 append IP，第 317-319 行的 if check 仅用于日志，没有 break
6. 继续 for 循环，尝试解析 ns4.qq.com、ns1.qq.com、ns2.qq.com
7. 每个 NS 的解析都触发完整的状态机递归调用 Change(stm)
8. 调用栈不断加深，最终溢出

修复方案（最小化）：
在 server/state_define.go 第 317-320 行的 if 块中添加 break：
```go
if len(ipList) > 0 {
    monitor.Rec53Log.Debugf("[ITER] Resolved NS %s to IPs: %v", nsName, ipList)
    break  // 获得第一个可用 NS IP 后立即返回
}
```

原理：DNS 递归解析只需要一个可用的 nameserver IP 地址就能继续迭代，获得第一个 NS 的 IP 后，没必要继续解析其他 NS 的 IP，避免不必要的深层递归调用。

Implementation:
- [x] server/state_define.go: resolveNSIPsRecursively() 第 319 行添加 break
- [x] 编译验证：go build -o rec53 ./cmd 成功
- [x] 单元测试验证：go test -short ./server/... 全部通过（无破坏）
- [x] 创建 e2e/glue_recursion_test.go 新文件
  - [x] TestB017_NoGlueNSRecursionStackOverflow: 验证无 glue NS 场景的正确处理
  - [ ] TestB017_MultipleNSNoGlueRecovery: 跳过（待实现 NS 故障模拟）

Acceptance criteria:
- [x] 在 server/state_define.go:317-320 添加 break 语句
- [x] 编译通过：go build -o rec53 ./cmd
- [ ] E2E 测试验证：TestB017_NoGlueNSRecursionStackOverflow
  - [ ] 创建 e2e/glue_recursion_test.go 文件（已创建）
  - [ ] 测试构造 3 层 DNS 层级：root → .com TLD → qq.com 权威
  - [ ] qq.com NS 记录无 glue 记录，触发递归 NS 解析
  - [ ] 验证答案包含正确的 CNAME + A 记录
  - [ ] 验证查询在 5 秒内完成（无无限循环）
  - [ ] 验证 mock 服务器请求数 < 20（无过度递归）
  - [ ] 验证 NS A 记录查询数 < 4（break 正确生效，不解析所有 NS）
  - [ ] 运行 go test -v ./e2e/... 通过
- [ ] 功能验证：dig @localhost www.qq.com 快速返回正确结果，无超时重试
- [ ] 稳定性验证：查询返回后，rec53 进程继续运行，不会崩溃
- [ ] 回归测试：go test ./... 全部通过，无破坏

## Completed

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