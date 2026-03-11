# IP池改进实现路线图

**项目名**：IP Pool Maintenance Algorithm Enhancement  
**推荐方案**：滑动窗口直方图 + 故障自动恢复  
**总工作量**：3-4周（15.5天）  
**开始日期**：待定  
**交付日期**：待定  

---

## 项目概览

### 目标
将IP池维护算法从单点值追踪升级到多样本统计，实现自动故障恢复，故障恢复时间从∞降低到3-5秒。

### 关键收益
- ✅ 故障自动恢复（3-5秒 vs 永久）
- ✅ 异常值容错（中位数免疫）
- ✅ 置信度驱动决策（小样本不等于大样本）
- ✅ 完整延迟可观测性（P50/P95/P99）

---

## 阶段划分

### Phase 1：基础设施（1周）
**目标**：实现滑动窗口、百分位计算，为后续阶段奠基

#### P1.1 新增数据结构 IPQualityV2
**工作项**：
- [ ] 定义IPQualityV2结构体
  - [ ] [64]int32 样本环形缓冲区
  - [ ] p50/p95/p99 统计字段
  - [ ] confidence 置信度字段（0-100%）
  - [ ] failCount/state 故障追踪字段
  - [ ] 使用 sync.RWMutex 保护并发访问

**验收标准**：
- 结构体定义完整，无编译错误
- 字段对齐和内存占用符合预期（288字节/IP）

**文件**：`server/ip_pool.go` ~80行代码

#### P1.2 实现样本管理
**工作项**：
- [ ] `RecordLatency(latency int32)` 方法
  - 添加样本到环形缓冲区
  - 更新置信度（sampleCount × 10%，上限100%）
  - 重置failCount和state
  - 调用updatePercentiles()
  
- [ ] `updatePercentiles()` 私有方法
  - 排序样本
  - 计算P50、P95、P99
  - 防守性编程：边界检查

**单元测试**：
- [ ] `TestRecordLatencySingleSample`: 单个样本
- [ ] `TestRecordLatencyMultipleSamples`: 多个样本正确排序
- [ ] `TestPercentileCalculation`: P50=中位数
- [ ] `TestPercentile95Calculation`: P95准确性
- [ ] `TestPercentile99Calculation`: P99准确性
- [ ] `TestConfidenceUpdate`: 置信度从0增长到100
- [ ] `TestRingBufferWraparound`: 满缓冲区新样本覆盖旧样本
- [ ] `TestEdgeCases`: 1个样本、64个样本边界

**验收标准**：
- 所有8个单元测试通过
- 代码覆盖率 > 90%

**文件**：`server/ip_pool.go` ~150行，`server/ip_pool_test.go` ~200行

#### P1.3 集成测试
**工作项**：
- [ ] 创建模拟DNS延迟分布
  - 正态分布：平均100ms，标准差20ms
  - 生成1000个样本
  
- [ ] 验证统计准确性
  - 计算理论P50（100ms）vs 实测
  - 计算理论P95 vs 实测
  - 误差范围 < 10%

**验收标准**：
- 模拟与实测偏差 < 10%

**文件**：`server/ip_pool_test.go` ~100行

**时间估算**：5天

---

### Phase 2：故障处理与恢复（1.5周）
**目标**：实现故障的指数退避和自动探针恢复机制

#### P2.1 故障降级
**工作项**：
- [ ] `RecordFailure()` 方法
  - failCount++
  - 设置lastFailure时间戳
  - 根据failCount实施指数退避：
    - 1-3次：state=DEGRADED, p50*=1.2
    - 4-6次：state=SUSPECT, p50=MAX
    - 7+次：state=SUSPECT，标记可恢复

**单元测试**：
- [ ] `TestFailureCount1To3`: DEGRADED状态 + 延迟×1.2
- [ ] `TestFailureCount4To6`: SUSPECT状态 + p50=MAX
- [ ] `TestFailureCount7Plus`: SUSPECT标记，可探针

**验收标准**：
- 3个故障阶段状态转移正确
- 延迟修改符合预期

**文件**：`server/ip_pool.go` ~50行

#### P2.2 周期性探针
**工作项**：
- [ ] `ShouldProbe()` 方法
  - 条件1：state == SUSPECT
  - 条件2：lastFailure距现在 > 30秒
  - 条件3：failCount >= 6
  
- [ ] `ResetForProbe()` 方法
  - failCount = 0
  - state = RECOVERED
  - 不重置p50，等待真实查询更新

- [ ] `periodicProbeLoop()` 后台任务
  - 创建 context.WithCancel()
  - 启动 goroutine，添加到 WaitGroup
  - 30秒间隔扫描
  - 对ShouldProbe()=true的IP执行探针
  - 监听ctx.Done()优雅关闭

- [ ] `probeAllSuspiciousIPs()` 探针执行
  - 扫描pool中所有IP
  - 对每个ShouldProbe()=true的IP发送DNS查询
  - 查询成功：调用ResetForProbe()
  - 查询失败：保持状态不变

**集成测试**：
- [ ] `TestShouldProbeTiming`: 30秒条件验证
- [ ] `TestProbeSuccessResets`: 探针成功状态转移
- [ ] `TestPeriodicProbeLoop`: 后台任务30秒触发
- [ ] `TestGracefulShutdown`: context取消时goroutine退出

**并发安全验证**：
- [ ] 100个并发查询 + 1个探针goroutine不死锁
- [ ] 探针中途关闭不导致资源泄漏

**验收标准**：
- 所有4个集成测试通过
- 无goroutine泄漏（go test -race通过）

**文件**：`server/ip_pool.go` ~200行

#### P2.3 综合集成测试
**工作项**：
- [ ] 模拟故障恢复完整流程
  - T0: 创建IP，state=ACTIVE
  - T1-3: 连续3次查询失败 → state=DEGRADED
  - T4-6: 继续3次查询失败 → state=SUSPECT
  - T7: 30秒后探针成功 → state=RECOVERED
  - T8: 实际查询成功 → state=ACTIVE
  - 验证整个过程耗时3-5秒恢复

**验收标准**：
- 故障恢复全流程 < 10秒
- 最终状态转移正确

**文件**：`server/ip_pool_test.go` ~150行

**时间估算**：8天（含测试调试）

---

### Phase 3：智能选择算法（1周）
**目标**：实现基于综合评分的IP选择算法

#### P3.1 评分函数
**工作项**：
- [ ] `GetScore()` 方法
  - 基础分 = p50
  - 置信度修正 = 1.0 + (100 - confidence) × 0.01
  - 状态权重 = {ACTIVE:1.0, DEGRADED:1.5, SUSPECT:100, RECOVERED:1.1}
  - 最终分 = 基础分 × 置信度修正 × 状态权重

**单元测试**：
- [ ] `TestScoreCalculation`: 基础评分逻辑
- [ ] `TestConfidenceWeighting`: 低置信度增权
- [ ] `TestDegradedWeighting`: 性能下降IP
- [ ] `TestSuspectWeighting`: 可疑IP降权100倍
- [ ] `TestRecoveredWeighting`: 恢复中IP轻微降权

**验收标准**：
- 5个单元测试通过
- 评分公式符合设计意图

**文件**：`server/ip_pool.go` ~50行

#### P3.2 改进的getBestIPs
**工作项**：
- [ ] `GetBestIPsV2(ips []string)` 方法
  - 遍历IP列表，计算每个GetScore()
  - 排序scores从小到大
  - 返回scores[0]和scores[1]
  - 兼容性：保留旧getBestIPs()

**单元测试**：
- [ ] `TestGetBestIPsEmpty`: 空列表返回空字符串
- [ ] `TestGetBestIPsSingle`: 单IP返回该IP
- [ ] `TestGetBestIPsMultiple`: 多IP返回score最低的两个
- [ ] `TestGetBestIPsScoreOrdering`: 结果符合评分顺序

**验收标准**：
- 4个单元测试通过
- 返回值正确

**文件**：`server/ip_pool.go` ~60行

#### P3.3 对比测试
**工作项**：
- [ ] 创建100个IP的测试用例
  - 50个ACTIVE IP，延迟60-120ms
  - 30个DEGRADED IP，延迟150-200ms
  - 15个SUSPECT IP，延迟MAX
  - 5个RECOVERED IP，延迟200ms
  
- [ ] 对比测试
  - 调用旧getBestIPs()，记录选择结果
  - 调用GetBestIPsV2()，记录选择结果
  - 新算法应选择更优的IP，避免故障IP

**验收标准**：
- 新算法选择的bestIP score < 旧算法score
- SUSPECT IP不被选择（除非别无选择）

**文件**：`server/ip_pool_test.go` ~200行

**时间估算**：7天

---

### Phase 4：迁移与优化（1周）
**目标**：集成到生产代码，性能测试，灰度发布

#### P4.1 迁移state_define.go
**工作项**：
- [ ] 替换getBestAddressAndPrefetchIPs()
  ```go
  // 旧：bestIP, backupIP := globalIPPool.getBestIPs(ipList)
  // 新：
  bestIP, backupIP := globalIPPool.GetBestIPsV2(ipList)
  ```

- [ ] 替换updateIPQuality()调用
  ```go
  // 旧：globalIPPool.updateIPQuality(theBestIP, int32(rtt/time.Millisecond))
  // 新：
  globalIPPool.RecordLatencyV2(theBestIP, int32(rtt/time.Millisecond))
  ```

- [ ] 移除UpIPsQuality()调用
  ```go
  // 旧：globalIPPool.UpIPsQuality(ipList)
  // 新：已内置在RecordLatency中，不再需要单独调用
  ```

- [ ] 新增故障处理
  ```go
  if err != nil {
      globalIPPool.RecordFailureV2(bestIP)  // 新增
      // ... 重试逻辑
  }
  ```

**验收标准**：
- 所有调用点替换完成
- go build无编译错误
- go test -v ./server/...通过

**文件修改**：`server/state_define.go` ~20行改动

#### P4.2 Prometheus指标
**工作项**：
- [ ] 新增metric类型
  ```go
  IPQualityP50Gauge     // rec53_ip_p50_latency_ms
  IPQualityP95Gauge     // rec53_ip_p95_latency_ms
  IPQualityP99Gauge     // rec53_ip_p99_latency_ms
  IPPoolStateGauge      // rec53_ip_pool_state
  IPPoolConfidenceGauge // rec53_ip_confidence
  ```

- [ ] 更新RecordLatency调用点
  - 添加IPQualityP50Gauge.Set()
  - 添加IPQualityP95Gauge.Set()
  - 添加IPQualityP99Gauge.Set()

- [ ] 更新RecordFailure调用点
  - 更新IPPoolStateGauge

**验收标准**：
- 所有指标正确导出
- Prometheus能采集到metrics

**文件修改**：`monitor/metric.go` ~100行

#### P4.3 性能基准测试
**工作项**：
- [ ] `BenchmarkGetBestIPsV2_100IPs`
  - 预期：< 100μs
  
- [ ] `BenchmarkGetBestIPsV2_1000IPs`
  - 预期：< 1ms

- [ ] `BenchmarkRecordLatency`
  - 预期：< 5μs

- [ ] `BenchmarkPercentileCalculation`
  - 预期：< 2μs

**运行**：
```bash
go test -bench=Benchmark -benchmem ./server
```

**验收标准**：
- 所有bench符合预期时间
- 内存分配 < 1KB/操作

**文件**：`server/ip_pool_test.go` ~150行

#### P4.4 E2E集成测试
**工作项**：
- [ ] 创建完整DNS查询流程测试
  - 初始化IP池
  - 模拟10个DNS查询
  - 第3个查询失败（故障转移）
  - 第5个查询超时（进入SUSPECT）
  - 第8-10个查询恢复
  - 验证选择结果、故障处理、恢复流程

**验收标准**：
- 完整E2E流程通过
- 故障恢复时序符合预期

**文件**：`e2e/ip_pool_test.go` ~300行

#### P4.5 灰度发布（可选）
**工作项**：
- [ ] 添加功能开关
  ```go
  // 环境变量或配置文件
  USE_IP_POOL_V2=true/false
  ```

- [ ] 两版本并行运行（A/B测试）
  ```go
  if useV2 {
      bestIP, _ = globalIPPoolV2.GetBestIPsV2(ipList)
  } else {
      bestIP, _ = globalIPPool.getBestIPs(ipList)
  }
  ```

- [ ] 度量对比
  - 新旧算法选择差异
  - 故障恢复时间改进

**验收标准**：
- 功能开关生效
- 两版本结果可对比

**文件修改**：`cmd/rec53.go` ~50行

**时间估算**：7天

---

## 工作项总结

| 阶段 | 工作项 | 预估 | 优先级 | 负责人 |
|------|--------|------|--------|--------|
| P1.1 | IPQualityV2结构体定义 | 1d | ⭐⭐⭐ | TBD |
| P1.2 | 样本管理 + 百分位计算 | 2d | ⭐⭐⭐ | TBD |
| P1.3 | 集成测试：统计验证 | 1d | ⭐⭐⭐ | TBD |
| P2.1 | 故障降级 RecordFailure | 1.5d | ⭐⭐⭐ | TBD |
| P2.2 | 周期探针 periodicProbeLoop | 2d | ⭐⭐⭐ | TBD |
| P2.3 | 集成测试：故障恢复 | 1.5d | ⭐⭐ | TBD |
| P3.1 | GetScore 评分函数 | 1d | ⭐⭐⭐ | TBD |
| P3.2 | GetBestIPsV2 选择 | 0.5d | ⭐⭐⭐ | TBD |
| P3.3 | 对比测试 | 1.5d | ⭐⭐ | TBD |
| P4.1 | 迁移 state_define.go | 1d | ⭐⭐⭐ | TBD |
| P4.2 | Prometheus 指标 | 1d | ⭐⭐ | TBD |
| P4.3 | 性能基准测试 | 0.5d | ⭐ | TBD |
| P4.4 | E2E 集成测试 | 1d | ⭐⭐ | TBD |
| P4.5 | 灰度发布（可选） | 1d | ⭐ | TBD |
| **总计** | | **15.5d** | | |

假设1人全职开发，**总工作量约3-4周**（含buffer和测试调试）。

---

## 风险管理

### 风险1：百分位计算性能
**风险描述**：每次RecordLatency都需要排序，可能成为瓶颈  
**缓解措施**：
- 样本数仅64，sort时间<1μs
- 可后续优化为O(N)的Quickselect算法
- 性能基准测试验证 < 2μs

### 风险2：并发竞态条件
**风险描述**：多goroutine同时修改pool可能导致数据不一致  
**缓解措施**：
- 使用RWMutex保护pool map
- 每个IPQuality有自己的mutex
- go test -race 验证

### 风险3：内存溢出
**风险描述**：IP数量过多导致内存占用爆炸  
**缓解措施**：
- 监控pool大小，告警 > 10000 IP
- 可添加LRU淘汰长期不使用的IP
- 预留扩展机制

### 风险4：探针风暴
**风险描述**：SUSPECT IP过多时，探针查询激增  
**缓解措施**：
- 每个SUSPECT IP至少等待30秒冷却
- 可添加信号量限制并发探针数
- 探针查询本身有DNS timeout保护

### 风险5：线上回滚困难
**风险描述**：新版本发现问题难以快速回滚  
**缓解措施**：
- 实现功能开关USE_IP_POOL_V2
- 两版本共存，快速切换
- 默认关闭新版本，逐步灰度

---

## 成功标准

### 功能验收
- ✅ Phase 1-4 所有工作项完成
- ✅ 单元测试覆盖率 > 80%
- ✅ go build无编译警告
- ✅ go test -race无竞态检测

### 性能验收
- ✅ GetBestIPsV2选择：1000 IP < 1ms
- ✅ RecordLatency：< 5μs
- ✅ 内存占用：1000 IP < 1MB

### 可靠性验收
- ✅ 故障恢复时间 < 10秒（目标 < 5秒）
- ✅ 0次线上回滚
- ✅ 监控指标正确导出
- ✅ 无新增告警

### 业务验收
- ✅ P99延迟改善 > 10%（监控数据）
- ✅ 故障自动恢复率 > 95%
- ✅ 异常值场景容错能力明显提升

---

## 后续优化方向

### 短期（1个月内）
- 数据持久化：重启不丢失样本数据
- 配置化参数：30秒探针间隔、64样本数可配置
- 更详细的日志：故障过程、状态转移

### 中期（6个月）
- 地理位置感知：不同地区的nameserver权重不同
- 缓存命中率融合：综合考虑可靠性 + 速度
- EDNS Client Subnet (ECS)支持：地理定位优化

### 长期（1年+）
- 机器学习预测：历史数据训练模型
- 跨nameserver关联分析：检测级联故障
- 自动nameserver黑名单：持续不可用自动剔除

---

**文档版本**：v1.0 (2026-03-11)  
**最后更新**：2026-03-11  
**审阅**：待安排  
**批准**：待安排
