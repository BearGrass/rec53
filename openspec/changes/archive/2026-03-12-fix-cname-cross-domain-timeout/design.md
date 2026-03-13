## Context

rec53 是一个迭代式 DNS 解析器，通过状态机（`STATE_INIT → IN_CACHE → IN_GLUE → IN_GLUE_CACHE → ITER → CHECK_RESP → RET_RESP`）递归地向权威服务器查询并缓存结果。

当前的 CNAME 处理逻辑（`state_machine.go` 的 `CHECK_RESP_GET_CNAME` 分支）在跨域跳转时，通过 `isNSRelevantForCNAME` 决定是否保留上一次 ITER 得到的 `response.Ns/Extra`。保留时，后续 `IN_GLUE` 状态会直接用这批 NS IP 去查询新目标域；清空时，`IN_GLUE_NOT_EXIST → IN_GLUE_CACHE` 会从缓存或根服务器重新获取正确委托。

**现有 bug 的触发条件**：

```
www.huawei.com A 查询路径（冷缓存）：
  ITER 向 huawei.com NS 查询
  → 返回: CNAME www.huawei.com.akadns.net, Ns=[akadns.net NS], Extra=[glue]
  → isNSRelevantForCNAME("akadns.net.", "www.huawei.com.akadns.net.") = true
  → Ns/Extra 保留 → IN_GLUE_EXIST → ITER 向 akadns.net NS 查 akadns.net 域 ✓（这跳正确）
  → 返回: CNAME www.huawei.com.c.cdnhwc1.com, Ns=[akadns.net NS]（同域返回）
  → isNSRelevantForCNAME("akadns.net.", "www.huawei.com.c.cdnhwc1.com.") = false
  → Ns 清空 → IN_GLUE_NOT_EXIST → IN_GLUE_CACHE → 缓存冷 → 用根服务器
  → 从根迭代 cdnhwc1.com 委托链（需多轮 ITER）
  → ITER 向 cdnhwc1.com NS 查，NS 无 glue → resolveNSIPsConcurrently depth=1 截断
  → ITER_COMMON_ERROR → SERVFAIL → 客户端超时
```

第三次成功的原因是前两次的迭代过程中（即使最终失败），中间步骤的成功 ITER 已向缓存写入了 `cdnhwc1.com` 和 `chinamobile.com` 的 NS+glue（`state_define.go:856-859`），第三次 `IN_GLUE_CACHE` 命中缓存，跳过了无 glue NS 解析路径。

**核心问题**：`IN_GLUE` 状态（`state_define.go:348-358`）对"有 Ns 就视为有 glue"，没有验证这批 Ns 是否与**当前查询域**相关。

## Goals / Non-Goals

**Goals:**
- 修复冷缓存下多跳跨域 CNAME 解析必然超时的问题
- 首次查询 www.huawei.com 即可在正常 DNS 延迟内完成（< 10s）
- 不引入新的外部依赖
- 修复后通过所有现有测试

**Non-Goals:**
- 不解决 `resolveNSIPsConcurrently` depth=1 截断问题（该问题是防死锁设计，需单独评估）
- 不优化缓存命中率或预取策略
- 不改变缓存键格式或数据结构

## Decisions

### 决策 1：在 `inGlueState.handle` 中增加 Ns 域相关性校验

**方案**：`inGlueState.handle` 在返回 `IN_GLUE_EXIST` 之前，验证 `response.Ns[0].Header().Name`（即 NS 所属区域）是否是当前查询域（`request.Question[0].Name`）的**祖先域**（ancestor，含自身）。若不是，则清空 `response.Ns` 和 `response.Extra`，返回 `IN_GLUE_NOT_EXIST`。

```go
// 伪代码
func (s *inGlueState) handle(request, response) (int, error) {
    if len(response.Ns) != 0 && len(response.Extra) != 0 {
        nsZone := response.Ns[0].Header().Name
        queryName := request.Question[0].Name
        // 仅当 nsZone 是 queryName 的祖先（或相同区域）时，glue 才有效
        if dns.IsSubDomain(nsZone, queryName) {
            return IN_GLUE_EXIST, nil
        }
        // 旧域 NS 与新查询域无关，清空，走 IN_GLUE_CACHE 重新委托
        response.Ns = nil
        response.Extra = nil
    }
    return IN_GLUE_NOT_EXIST, nil
}
```

**备选方案 A**：在 `state_machine.go` 的 `CHECK_RESP_GET_CNAME` 中，CNAME 跨域时总是无条件清空 Ns/Extra（无论 `isNSRelevantForCNAME` 返回什么）。
- 缺点：破坏了 `isNSRelevantForCNAME` 已有的优化（同域 CNAME 时保留 glue 可减少一次 IN_GLUE_CACHE 查询），且与 B-004 修复的初衷冲突。

**备选方案 B**：修改 `isNSRelevantForCNAME` 逻辑，在 akadns.net → cdnhwc1.com 跨域时直接清空。
- 问题：`isNSRelevantForCNAME` 已经正确返回 `false`（已清空），真正的问题出在**之前**保留下来的 NS 在下一次 CNAME 跳转时仍残留在 response 中。逻辑本身不是问题，是 `IN_GLUE` 缺乏校验。

**选择方案（主方案）**：在 `inGlueState.handle` 中校验，因为这是最接近问题发生点的防线，且不依赖调用方是否正确清空了 Ns，具有防御性。

### 决策 2：`isNSRelevantForCNAME` 保持现有逻辑不变

`isNSRelevantForCNAME` 的逻辑（`state_machine.go:28-40`）在语义上是正确的：检查 NS 区域是否是 CNAME 目标的祖先。它为同域 CNAME（如 huawei.com → akadns.net 第一跳）提供了 glue 复用优化。不修改此函数，校验收口到 `inGlueState`。

### 决策 3：测试使用现有 `startWarmupTestMockDNS` 基础设施

新增集成测试时，复用 `warmup_test.go` 中的 `startWarmupTestMockDNS` 模式（mock DNS 响应 SERVFAIL），同时扩展为能模拟 CNAME 链委托的 mock server（按查询域名返回不同响应）。这样测试不依赖真实网络，且能精确控制 CNAME 链结构。

## Risks / Trade-offs

- **[风险] 清空了本可复用的 glue** → 当 NS 区域与查询域不相关但偶尔能提供有效响应时（极少见），新逻辑会多一次 `IN_GLUE_CACHE` 查询。代价是微小的额外延迟（通常命中 L1 缓存），可接受。
- **[风险] `dns.IsSubDomain` 的边界情况** → 根区域 `.` 是所有域的祖先，`IsSubDomain(".", anyDomain)` 应返回 `true`。需在单元测试中覆盖根区域 NS 的情况（根服务器 glue 不应被清空）。
- **[风险] Extra 中有多个 NS 但 Ns[0] 区域不代表整体** → 实践中同一批 NS 记录均属同一区域，取 `Ns[0]` 判断足够。若有异构 NS（不同区域），清空是保守但安全的选择。

## Open Questions

- 无。本设计已经过对完整代码路径的分析，实现路径清晰。
