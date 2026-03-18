## ADDED Requirements

### Requirement: lastSeen 时间戳跟踪
`IPQualityV2` SHALL 维护一个 `lastSeen` 时间戳字段，记录该 IP 最近一次被查询路径引用的时间。`lastSeen` 在 `RecordLatency` 和 `RecordFailure` 调用时 MUST 被更新为当前时间。`NewIPQualityV2()` 创建时 MUST 将 `lastSeen` 初始化为 `time.Now()`。
`lastSeen` 的读写 MUST 遵循 `IPQualityV2` 的并发保护语义：写入在持有 `IPQualityV2.mu` 的情况下执行，读取必须通过同一互斥保护（例如 `GetLastSeen()` 持有 `RLock`）。

#### Scenario: RecordLatency 更新 lastSeen
- **WHEN** 对某个 IP 调用 `RecordLatency(latency)` 记录一次成功响应
- **THEN** 该 IP 的 `lastSeen` MUST 被更新为当前时间（误差 < 1s）

#### Scenario: RecordFailure 更新 lastSeen
- **WHEN** 对某个 IP 调用 `RecordFailure()` 记录一次失败
- **THEN** 该 IP 的 `lastSeen` MUST 被更新为当前时间（误差 < 1s）

#### Scenario: 新建条目的 lastSeen 初始化
- **WHEN** 通过 `NewIPQualityV2()` 创建新的 IP 质量跟踪条目
- **THEN** `lastSeen` MUST 被初始化为 `time.Now()`，不得为零值

### Requirement: PruneStaleIPs 清理陈旧条目
`IPPool` SHALL 提供 `PruneStaleIPs(threshold time.Duration)` 方法，删除所有 `lastSeen` 距当前时间超过 `threshold` 的 IP 条目。属于豁免集合的 IP MUST 被跳过，不论其 `lastSeen` 值。

#### Scenario: 超过阈值的 IP 被清理
- **WHEN** 某 IP 的 `lastSeen` 距今超过 24h，且该 IP 不在豁免集合中
- **THEN** `PruneStaleIPs(24h)` MUST 从 IP 池中删除该条目

#### Scenario: 未超过阈值的 IP 被保留
- **WHEN** 某 IP 的 `lastSeen` 距今不足 24h
- **THEN** `PruneStaleIPs(24h)` MUST 保留该条目不变

#### Scenario: 豁免 IP 不被清理
- **WHEN** 某 IP 属于豁免集合（根服务器 IP），即使其 `lastSeen` 距今超过阈值
- **THEN** `PruneStaleIPs` MUST 跳过该 IP，不得删除

#### Scenario: 空池调用无副作用
- **WHEN** IP 池为空时调用 `PruneStaleIPs`
- **THEN** 方法正常返回，无 panic，无日志错误

### Requirement: 根服务器豁免集合依赖注入
`StartProbeLoop` SHALL 接受一个 `exemptIPs map[string]struct{}` 参数，作为 prune 时的豁免集合。`utils` 包 SHALL 提供 `ExtractRootIPs() map[string]struct{}` 函数，从 `GetRootGlue()` 的 Extra section 提取所有根服务器 A 记录 IP。

#### Scenario: StartProbeLoop 接受豁免集合
- **WHEN** 默认运行路径调用 `StartProbeLoop(exemptIPs)`，并传入由 `utils.ExtractRootIPs()` 构造的 13 个根服务器 IP 集合
- **THEN** 后续 `PruneStaleIPs` 调用 MUST 使用该集合进行豁免判断

#### Scenario: ExtractRootIPs 返回完整根服务器 IP 集合
- **WHEN** 调用 `utils.ExtractRootIPs()`
- **THEN** 返回的 map MUST 包含 `GetRootGlue()` 中所有 13 个根服务器的 A 记录 IP

#### Scenario: 自定义豁免集合用于测试
- **WHEN** 测试中传入自定义豁免集合 `{"1.2.3.4": {}}`
- **THEN** prune 时 MUST 仅按该集合做豁免判断（即仅豁免 `1.2.3.4`）

### Requirement: 定期自动 prune
系统 SHALL 在 `periodicProbeLoop` 中按时间间隔自动执行 `PruneStaleIPs(STALE_IP_THRESHOLD)`：当距离上次 prune 已超过 `PRUNE_INTERVAL = 30 * time.Minute` 时必须触发一次。`STALE_IP_THRESHOLD` 为包级常量 `24 * time.Hour`。不得为此新增 goroutine。

#### Scenario: 正常运行时定期 prune
- **WHEN** `periodicProbeLoop` 运行，且距离上次 prune 已达到或超过 `PRUNE_INTERVAL`
- **THEN** MUST 调用一次 `PruneStaleIPs(24h)`

#### Scenario: shutdown 时 prune 停止
- **WHEN** `IPPool.Shutdown` 被调用，context 取消
- **THEN** prune 循环 MUST 随 probe loop 一起停止，不泄漏 goroutine
