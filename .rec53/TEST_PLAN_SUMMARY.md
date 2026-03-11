# 测试覆盖补充计划总结

## 背景

根据 `doc/example` 中的25个DNS解析场景进行详细的测试覆盖分析，当前项目测试覆盖率为 **64%**（16/25）。

已生成两份详细分析文档：
- `TEST_COVERAGE_ANALYSIS.md` - 完整的覆盖分析报告
- `TEST_COVERAGE_DETAILED.md` - 按表格形式的详细对比

## 覆盖现状

### ✅ 已覆盖（13个场景）
- 例1：缓存全命中
- 例3：完整迭代解析
- 例5：CNAME跨区解析
- 例6/7：CNAME链处理（全/部分命中）
- 例12：NS地址子查询
- 例14：畸形查询处理
- 例18/19：CNAME循环和深度超限
- 例22：子查询深度超限

### ⚠️ 部分覆盖（3个场景）
- 例4：缓存部分命中（快速路径不够完整）
- 例8：QTYPE=CNAME查询（特殊处理不完全）
- 例11：TC=1截断（缺TCP重试逻辑）

### ❌ 未覆盖（9个场景）
最关键的3个：
- **例2：Negative cache缓存命中** → T-003
- **例9/10：NODATA/NXDOMAIN响应** → T-002（B-012 bug修复）
- **例21：Query budget耗尽** → T-004

其他6个：
- 例13：CNAME+NXDOMAIN组合 → T-008
- 例15：超时重试换服务器 → T-005
- 例16：SERVFAIL换服务器 → T-006
- 例17：ID不匹配 → T-007
- 例20：Referral循环 → T-009
- 例23：所有NS不可达 → T-010

## 补充测试计划（10个Task）

按优先级分组：

### 🔴 High Priority（关键功能）4个任务

| Task | 场景 | 描述 | 工时 | 覆盖提升 |
|------|------|------|------|---------|
| **T-002** | 例9,10 | 修复B-012，启用NODATA/NXDOMAIN测试 | 4h | +2 |
| **T-003** | 例2 | Negative Cache E2E测试 | 6h | +1 |
| **T-004** | 例21 | Query Budget耗尽测试 | 5h | +1 |
| **T-005** | 例15 | 超时重试和换服务器完整流程 | 7h | +1 |

**小计：** 22小时，覆盖+5个场景，达到 **89%** 覆盖率 ⭐ 推荐方案

### 🟠 Medium Priority（故障恢复）2个任务

| Task | 场景 | 描述 | 工时 |
|------|------|------|------|
| **T-006** | 例16 | SERVFAIL处理和服务器标记 | 5h |
| **T-007** | 例17 | Response ID不匹配处理 | 4h |

**小计：** 9小时，覆盖+2个场景

### 🟡 Low Priority（组合和边界）4个任务

| Task | 场景 | 描述 | 工时 |
|------|------|------|------|
| **T-008** | 例13 | CNAME+NXDOMAIN组合 | 6h |
| **T-009** | 例20 | Referral循环检测 | 8h |
| **T-010** | 例23 | 所有NS不可达 | 5h |
| 例24 | 空响应异常 | 4h |

**小计：** 23小时，覆盖+4个场景

## 实施路径建议

### 方案A：关键路径优先 ⭐ 推荐
**实施：T-002 → T-003 → T-004 → T-005**
- 工时：22小时
- 最终覆盖率：89%（23/25）
- 重点：核心功能和故障恢复
- 收益：快速提升覆盖率，解决最常见的问题

### 方案B：全量实现
**实施：T-002 → T-003 → ... → 全部10个**
- 工时：50小时
- 最终覆盖率：100%（25/25）
- 收益：完整测试覆盖

### 方案C：快速修复
**实施：T-002 → T-003 → T-004**
- 工时：15小时
- 最终覆盖率：76%（19/25）
- 收益：修复关键bug和核心功能

## 每个Task的实现框架

每个Task都已在 `BACKLOG.md` 中定义，包含：
- Priority（优先级）
- Related issues（关联问题）
- Description（详细描述）
- Location（实现位置）
- Acceptance criteria（验收标准）

### 实现建议

1. **T-002（修复B-012）**
   - 分析：checkRespState.handle() 中 S9(CLASSIFY_RESPONSE) 的响应分类逻辑
   - 修复：Authority+SOA 应路由到 S12(HANDLE_NEGATIVE) 而非 S10
   - 测试：启用已存在的 TestAuthorityNODATA 和 TestAuthorityNXDOMAIN

2. **T-003（Negative Cache）**
   - 扩展：e2e/cache_test.go
   - 流程：Mock NXDOMAIN → 缓存 → 再次查询应从缓存返回
   - 验证：响应时间和缓存命中率

3. **T-004（Budget耗尽）**
   - 新建：e2e/budget_test.go
   - 构造：深层NS委派链强制budget递减
   - 验证：budget=0时进入S15(FAIL_SERVFAIL)

4. **T-005（超时重试换服）**
   - 扩展：e2e/resolver_test.go 或 e2e/error_test.go
   - 场景：NS1超时→重试超时→切换NS2成功
   - 工具：使用现有的Mock DNS框架

## 文件引用

实现时参考以下文件：

### 核心状态机
- `server/state_machine.go` - 状态机主逻辑
- `server/state_define.go` - 各状态的handle()方法

### 现有测试框架
- `e2e/helpers.go` - Mock DNS服务器框架
- `e2e/authority_test.go` - E2E测试示例
- `e2e/error_test.go` - 错误处理测试示例

### 缓存和查询管理
- `server/cache.go` - 缓存逻辑
- `server/server.go` - 服务器主逻辑

## 预期收益

完成所有10个任务后：
- ✅ 测试覆盖率：100%（25/25 场景）
- ✅ 功能完整性：所有doc/example中的场景都有对应测试
- ✅ 回归防护：任何修改都能通过详细的测试套件验证
- ✅ 文档可信度：文档与测试完全同步

---

**最后更新**：2026-03-11
**分析工具**：OpenCode Agent (explore)
**详细分析**：见 TEST_COVERAGE_ANALYSIS.md 和 TEST_COVERAGE_DETAILED.md
