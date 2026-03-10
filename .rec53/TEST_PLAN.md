# 测试计划

**目标**: 将测试覆盖率从 ~30% 提升到 >60%

## 当前状态

```
$ go test -cover ./...
rec53/cmd      20.0%  (parseLogLevel 已测试)
rec53/monitor   3.2%  (SetLogLevel 已测试)
rec53/server   75.2%  (大部分已覆盖)
rec53/utils    82.6%  (Zone, Root 已测试)
rec53/e2e      28.6%  (需要网络，部分失败)
```

## 现有测试清单

### server 包 (75.2%) ✅ 覆盖良好

**cache_test.go** (已完善)
- [x] `TestGetCacheKey` - 缓存键生成
- [x] `TestCacheByType` - 类型化缓存
- [x] `TestCacheIsolation` - 深拷贝隔离
- [x] `TestSetAndGetCache` - 基本存取
- [x] `TestCacheExpiration` - TTL 过期
- [x] `TestCacheConcurrentAccess` - 并发安全
- [x] `TestCacheFlush` - 清空缓存
- [x] `TestCacheMultipleTypesSameDomain` - 多类型共存

**ip_pool_test.go** (已完善)
- [x] `TestIPQualityConcurrentAccess` - 原子操作并发
- [x] `TestIPQualityInit` - 初始化
- [x] `TestIPPoolGetSetIPQuality` - 存取操作
- [x] `TestIPPoolConcurrentAccess` - Pool 并发
- [x] `TestIPPoolGetBestIPs` - 最佳 IP 选择
- [x] `TestIPPoolGetPrefetchIPs` - 预取候选
- [x] `TestIPPoolUpdateIPQuality` - 更新质量
- [x] `TestIPPoolUpIPsQuality` - 提升质量
- [x] `TestIPPoolShutdown` - 关闭
- [x] `TestIPPoolIsTheIPInit` - 初始化检查

**state_machine_test.go** (已完善)
- [x] `TestCheckRespStateHandle` - 响应类型判断 (8 cases)
- [x] `TestInCacheStateHandle` - 缓存命中/未命中
- [x] `TestStateInitState` - 初始化状态
- [x] `TestInGlueState` - Glue 检查
- [x] `TestRetRespState` - 返回状态
- [x] `TestInGlueCacheState` - 缓存 NS 查找
- [x] `TestGetIPListFromResponse` - IP 提取
- [x] `TestGetBestAddressAndPrefetchIPs` - IP 选择
- [x] `TestIterState` - 迭代状态错误处理

**server_test.go** (已完善)
- [x] `TestNewServer` - 服务器创建
- [x] `TestServerRunAndShutdown` - 启动关闭
- [x] `TestServerUDPAddr` / `TestServerTCPAddr` - 地址获取
- [x] `TestIsUDP` - UDP/TCP 判断
- [x] `TestGetMaxUDPSize` - EDNS0 大小解析
- [x] `TestTruncateResponse` - UDP 截断

### utils 包 (82.6%) ✅ 覆盖良好

**zone_test.go**
- [x] `TestGetZoneList` - Zone 切分 (5 cases)
- [x] `TestGetZoneListConsistency` - 一致性

**root_test.go**
- [x] `TestGetRootGlue` - Root NS/A 记录
- [x] `TestGetRootGlueNSNames` - NS 名称验证
- [x] `TestGetRootGlueConsistency` - 一致性

### monitor 包 (3.2%) ⚠️ 需补充

**log_test.go**
- [x] `TestInitLogger` - atomicLevel 初始化
- [x] `TestSetLogLevel` - 日志级别设置
- [x] `TestAtomicLevel` - 级别修改

### cmd 包 (20.0%) ⚠️ 需补充

**log_level_test.go**
- [x] `TestParseLogLevel` - 日志级别解析 (11 cases)
- [x] `TestParseLogLevelConcurrency` - 并发安全

---

## 需补充的测试

### 第1批: monitor 包 (P0)

**monitor/metric.go** - 指标操作
```go
// TestMetricInCounterAdd - 入站计数器
// TestMetricOutCounterAdd - 出站计数器
// TestMetricLatencyHistogramObserve - 延迟直方图
// TestMetricIPQualityGaugeSet - IP 质量设置
// TestMetricRegister - 指标注册
```

### 第2批: cmd 包 (P1)

**cmd/rec53.go** - 主程序逻辑
```go
// TestMainFunction - 主函数流程 (使用 os.Exit mock)
// TestSignalHandling - 信号处理 (SIGINT/SIGTERM)
// TestGracefulShutdown - 优雅关闭流程
```

### 第3批: server 包缺失项 (P1)

**server/state_machine.go**
```go
// TestChangeMaxIterationsExceeded - 超过最大迭代
// TestChangeCNAMECycleDetection - CNAME 循环检测
// TestChangeOriginalQuestionPreserved - 原始问题保留
// TestChangeStateTransitions - 完整状态转换路径
```

**server/state_define.go - iterState**
```go
// TestIterStateSuccessfulQuery - 成功查询 (需要 mock DNS)
// TestIterStateFallbackToSecondIP - 主 IP 失败切换
// TestIterStateNXDOMAINHandling - NXDOMAIN 处理
// TestIterStateCacheUpdate - 缓存更新
```

### 第4批: utils 包缺失项 (P2)

**utils/net.go**
```go
// TestHcSuccess - 健康检查成功
// TestHcFailure - 健康检查失败
// TestHcTimeout - 超时处理
```

### 第5批: E2E 测试修复 (P2)

当前 E2E 测试因网络问题失败，需要：
1. 完善 MockAuthorityServer 模拟完整解析链
2. 添加无网络依赖的集成测试
3. 将真实网络测试标记为 `-tags=integration`

---

## 测试策略

### 单元测试模板

```go
func TestMetricInCounterAdd(t *testing.T) {
    // 初始化
    m := &Metric{reg: prometheus.NewRegistry()}
    m.Register()

    // 执行
    m.InCounterAdd("request", "example.com.", "A")

    // 验证 (通过 prometheus testutil)
    // ...
}
```

### 并发测试模板

```go
func TestConcurrentAccess(t *testing.T) {
    const workers = 100
    var wg sync.WaitGroup

    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            // 并发操作
        }()
    }

    wg.Wait()
}
```

### Mock DNS 服务器

使用 `e2e/helpers.go` 中的 `MockAuthorityServer`:
```go
zone := &Zone{
    Origin: "example.com.",
    Records: map[uint16][]dns.RR{
        dns.TypeA: {A("example.com.", "192.0.2.1", 300)},
    },
}
mock := NewMockAuthorityServer(t, zone)
defer mock.Stop()
```

---

## 执行计划

```
当前覆盖率: ~30%
    │
    ├── 第1批: monitor 包 (P0)
    │   └── 预计覆盖率提升: +5%
    │
    ├── 第2批: cmd 包 (P1)
    │   └── 预计覆盖率提升: +5%
    │
    ├── 第3批: server 包缺失项 (P1)
    │   └── 预计覆盖率提升: +10%
    │
    ├── 第4批: utils 包缺失项 (P2)
    │   └── 预计覆盖率提升: +3%
    │
    └── 目标: >60%
```

## 验收标准

| 里程碑 | 覆盖率 | 状态 |
|--------|--------|------|
| 第1批完成 | >35% | 待开始 |
| 第2批完成 | >40% | 待开始 |
| 第3批完成 | >50% | 待开始 |
| 全部完成 | >60% | 待开始 |

## 注意事项

1. **E2E 测试**: 需要网络连接，使用 `go test -short` 跳过
2. **并发测试**: 使用 `t.Parallel()` 标记可并行测试
3. **基准测试**: 后续添加 `BenchmarkXxx` 函数
4. **Mock 依赖**: 使用 `go.uber.org/mock` 生成 mock