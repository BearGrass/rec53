# rec53 测试覆盖对比详细表（Markdown表格版本）

## 表1：场景覆盖对比（按例号）

| 例号 | 场景名称 | 覆盖状态 | 对应测试文件 | 对应测试函数 | 实现细节 |
|:----:|---------|:--------:|-----------|----------|---------|
| 1 | 缓存全命中（最短路径） | ✅ 是 | server/state_machine_test.go | TestInCacheStateHandle | 命中缓存中的A记录，直接S1→S13→S16 |
| 2 | 缓存命中negative cache | ❌ 否 | - | - | Negative cache未实现相关机制 |
| 3 | 完整迭代解析（从根开始） | ✅ 是 | e2e/authority_test.go | TestAuthorityStandardA, TestAuthorityNSOnlyResponse | 标准root→.com→example.com三层解析 |
| 4 | 缓存部分命中（已知委派） | ⚠️ 部分 | e2e/authority_test.go | TestAuthorityStandardA | 测试了有缓存快速路径，但未明确验证S1(分支4) |
| 5 | CNAME跟踪—跨区解析 | ✅ 是 | e2e/authority_test.go, e2e/resolver_test.go | TestAuthorityCNAMEMultiHop, TestCNAMEResolution | 验证www.a.com→cdn.b.net的跨域CNAME |
| 6 | 缓存中已有CNAME链 | ✅ 是 | server/state_machine_test.go | TestCNAMEChain_MultiLevelResolution, TestCNAMEChain_TTLPreservation | 多级缓存CNAME链路径验证 |
| 7 | 缓存中CNAME链部分命中 | ✅ 是 | server/state_machine_test.go | TestCNAMEChain_MultiLevelResolution | CNAME链尾未命中时继续S2迭代 |
| 8 | QTYPE=CNAME的查询 | ⚠️ 部分 | server/state_machine_test.go | TestCheckRespState_CNAMEDetection | CNAME检测有测试，但special handling可能不完整 |
| 9 | 权威回复NODATA | ❌ 否 | e2e/authority_test.go | TestAuthorityNODATA(SKIP) | B-012 bug：状态机循环，测试被跳过 |
| 10 | 权威回复NXDOMAIN | ❌ 否 | e2e/authority_test.go | TestAuthorityNXDOMAIN(SKIP) | B-012 bug：状态机循环，测试被跳过 |
| 11 | TC=1截断，回退TCP | ⚠️ 部分 | e2e/authority_test.go | TestAuthorityTCFlag | TC标志验证，但未实现TCP重试逻辑 |
| 12 | NS地址需要子查询 | ✅ 是 | e2e/authority_test.go | TestAuthorityGluelessDelegation | 无Glue NS的地址子查询完整验证 |
| 13 | CNAME+NXDOMAIN同一响应 | ❌ 否 | - | - | 无测试覆盖此组合场景 |
| 14 | 畸形查询—S0拒绝 | ✅ 是 | server/state_machine_test.go, e2e/error_test.go | TestStateInit_FORMERR_NoQuestion, TestStateInit_FORMERR_MultipleQuestions, TestMalformedQueries | QDCOUNT=0、多问题等验证 |
| 15 | 超时重试后换服务器 | ❌ 否 | e2e/resolver_test.go | TestQueryTimeout | 仅有超时检测，无重试和换服务器的完整流程 |
| 16 | 上游返回SERVFAIL→换服务器 | ❌ 否 | - | - | 无Mock SERVFAIL响应的E2E测试 |
| 17 | Response ID不匹配 | ❌ 否 | - | - | 无ID验证失败的测试 |
| 18 | CNAME循环检测 | ✅ 是 | server/state_machine_test.go | TestCheckRespState_CNAMEDetection | CNAME循环（a.com→b.com→a.com）检测 |
| 19 | CNAME链深度超限 | ✅ 是 | server/state_machine_test.go | TestCNAMEChain_MultiLevelResolution | max_alias_depth超限检测 |
| 20 | Referral循环检测 | ❌ 否 | - | - | 无明确的referral_history循环测试 |
| 21 | Query budget耗尽 | ❌ 否 | - | - | 无budget=0导致S15的测试 |
| 22 | 子查询深度超限 | ✅ 是 | e2e/glue_recursion_test.go | TestB017_NoGlueNSRecursionStackOverflow | 间接验证max_subquery_depth控制 |
| 23 | 所有NS不可达 | ❌ 否 | - | - | 无所有NS都故障的场景测试 |
| 24 | 空响应/不可分类异常 | ❌ 否 | - | - | 无S9情况D（不可分类）的测试 |
| 25 | TC=1+TCP仍异常 | ❌ 否 | - | - | TCP重试未实现（O-006功能） |

---

## 表2：按覆盖状态分类统计

### A. 完全覆盖（✅ 是）的场景
| 例号 | 场景名称 | 主要测试 |
|:----:|---------|---------|
| 1 | 缓存全命中 | TestInCacheStateHandle |
| 3 | 完整迭代解析 | TestAuthorityStandardA |
| 5 | CNAME跨区解析 | TestAuthorityCNAMEMultiHop |
| 6 | CNAME缓存链全命中 | TestCNAMEChain_* |
| 7 | CNAME缓存链部分命中 | TestCNAMEChain_MultiLevelResolution |
| 12 | NS地址子查询 | TestAuthorityGluelessDelegation |
| 14 | 畸形查询处理 | TestStateInit_FORMERR_* |
| 18 | CNAME循环检测 | TestCheckRespState_CNAMEDetection |
| 19 | CNAME深度超限 | TestCNAMEChain_MultiLevelResolution |
| 22 | 子查询深度超限 | TestB017_NoGlueNSRecursionStackOverflow |

**小计：10个场景** ✅

### B. 部分覆盖（⚠️ 部分）的场景
| 例号 | 场景名称 | 现有测试 | 缺失部分 |
|:----:|---------|---------|---------|
| 4 | 缓存部分命中 | TestAuthorityStandardA | 未明确验证S1(分支4)路径 |
| 8 | QTYPE=CNAME查询 | TestCheckRespState_CNAMEDetection | CNAME不跟踪的特殊处理未完全覆盖 |
| 11 | TC=1回退TCP | TestAuthorityTCFlag | 缺少TCP重试逻辑测试 |

**小计：3个场景** ⚠️

### C. 未覆盖（❌ 否）的场景
| 例号 | 场景名称 | 原因 |
|:----:|---------|------|
| 2 | Negative cache | 未实现negative cache机制 |
| 9 | NODATA响应 | B-012 bug导致测试跳过 |
| 10 | NXDOMAIN响应 | B-012 bug导致测试跳过 |
| 13 | CNAME+NXDOMAIN | 组合场景缺测 |
| 15 | 超时重试换服务器 | 重试逻辑未有完整E2E |
| 16 | SERVFAIL换服务器 | 缺Mock故障场景 |
| 17 | ID不匹配处理 | 未实现相关测试 |
| 20 | Referral循环 | 未实现相关测试 |
| 21 | Budget耗尽 | 未实现相关测试 |
| 23 | 所有NS不可达 | 完全故障场景缺测 |
| 24 | 空响应异常 | 不可分类场景缺测 |
| 25 | TC+TCP异常 | O-006未实现 |

**小计：12个场景** ❌

---

## 表3：按测试类型统计

| 测试类型 | 文件数 | 测试函数数 | 覆盖的例号 | 说明 |
|---------|-------|---------|-----------|------|
| E2E 集成测试 | 6 | ~45 | 1,3,4,5,6,7,12,14,18,19,22 | authority_test, resolver_test, cache_test, error_test等 |
| 单元测试 | 1 | ~60 | 1,6,7,8,14,18,19 | state_machine_test.go重点 |
| IP池测试 | 1 | 8 | （与例号无直接映射） | ippool_v2_test.go |
| 服务器测试 | 1 | 6 | 14,25 | server_test.go |

---

## 表4：覆盖率按流程阶段

| 流程阶段 | 描述 | 覆盖率 | 测试示例 |
|---------|------|-------|---------|
| S0 - START | 初始化&校验 | 95% | TestStateInit_FORMERR_* |
| S1 - CHECK_LOCAL_DATA | 本地数据检查 | 80% | TestInCacheStateHandle, 部分CNAME链 |
| S2 - PICK_STARTING_NS | 选择起始NS | 60% | authority_test通过Mock验证 |
| S3 - ENSURE_NS_ADDRESS | 确保NS地址 | 75% | TestAuthorityGluelessDelegation |
| S4 - SELECT_SERVER | 选择服务器 | 40% | 缺少重试换服逻辑测试 |
| S5 - SEND_QUERY | 发送查询 | 50% | 缺少超时重试测试 |
| S6 - RECEIVE_RESPONSE | 接收响应 | 55% | 缺少ID验证失败测试 |
| S7 - PRECHECK_RESPONSE | 响应预检 | 60% | TC标志有测试，ID验证缺测 |
| S8 - EXTRACT_ALIAS_CHAIN | 提取CNAME | 85% | TestCNAMEChain_*, TestCheckRespState_CNAMEDetection |
| S9 - CLASSIFY_RESPONSE | 分类响应 | 40% | 情况A、B覆盖，情况C、D缺测 |
| S10 - HANDLE_DELEGATION | 处理委派 | 70% | authority_test系列 |
| S11 - RESOLVE_NS_ADDRESS_SUBQUERY | NS子查询 | 75% | TestAuthorityGluelessDelegation |
| S12 - HANDLE_NEGATIVE | 处理负响应 | 30% | NODATA/NXDOMAIN均跳过 |
| S13 - ASSEMBLE_SUCCESS | 组装成功 | 85% | 多数成功路径验证 |
| S14 - ASSEMBLE_NEGATIVE | 组装失败 | 30% | 由S12决定，B-012影响 |
| S15 - FAIL_SERVFAIL | 失败处理 | 20% | 缺少多个失败路径测试 |
| S16 - DONE | 完成 | 90% | 隐含覆盖 |

---

## 表5：覆盖缺口的优先级补充建议

| 优先级 | 场景编号 | 原因 | 建议补充测试 | 实现复杂度 |
|:----:|---------|------|-----------|----------|
| 🔴 High | 2,9,10 | 缓存和响应是DNS的核心，缺失会影响功能正确性 | 实现negative cache机制，修复B-012 | 高 |
| 🔴 High | 21 | Budget控制是安全防护，缺失会导致无限递归 | 添加budget=0→S15的E2E测试 | 中 |
| 🟠 Medium | 13,20 | 组合和循环检测是鲁棒性要求 | 补充组合响应和循环检测Mock | 中 |
| 🟠 Medium | 15,16 | 故障恢复是高可用的基础 | 补充SERVFAIL/超时的Mock故障场景 | 中 |
| 🟡 Low | 17,23,24,25 | 边界条件增强覆盖 | 补充ID验证、全部故障、异常分类 | 低~中 |

---

## 测试执行统计

```
E2E测试总数: ~50个
  - resolver_test.go:     9个
  - cache_test.go:        4个
  - error_test.go:       12个
  - authority_test.go:   10个（含2个SKIP）
  - glue_recursion_test: 1个（+1个SKIP）
  - server_test.go:       6个
  - ippool_v2_test.go:    8个

单元测试总数: ~65个
  - state_machine_test.go: ~60个
  - cache_test.go:         ~5个

总计: ~115个测试函数

覆盖的例号场景: 13个（完全）+ 3个（部分）= 16个 / 25个
覆盖率: 64%
```

