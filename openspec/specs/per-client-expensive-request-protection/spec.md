## Purpose

Define how rec53 limits expensive-path concurrency per client IP so a single requester cannot monopolize high-cost resolution work while preserving cheap-path responses and aggregate observability.

## ADDED Requirements

### Requirement: 每客户端昂贵请求并发必须有界
系统 SHALL 对每个客户端 IP 的昂贵解析并发施加上限，防止单个请求方同时占用无限数量的高成本解析路径。

#### Scenario: 本地快路径命中不消耗昂贵请求槽位
- **WHEN** 一个客户端请求完全通过 `hosts`、forwarding hit 或 cache hit 路径得到响应
- **THEN** 系统 SHALL NOT 将该请求计入该客户端的昂贵请求并发限制

#### Scenario: 递归解析占用一个昂贵请求槽位
- **WHEN** 一个客户端请求到达 `CACHE_LOOKUP_MISS` 并继续进入递归解析
- **THEN** 系统 SHALL 将该请求计为该客户端 IP 的一个昂贵 in-flight 请求
- **AND** 对同一个前台请求后续产生的 upstream retry 或子查询，系统 SHALL NOT 再额外占用新的槽位

#### Scenario: Forwarding 外部查询占用一个昂贵请求槽位
- **WHEN** 一个客户端请求离开 forwarding 快路径并发起 forwarding 外部 upstream 查询
- **THEN** 系统 SHALL 将该请求计为该客户端 IP 的一个昂贵 in-flight 请求
- **AND** 如果同一个前台请求后续还会经过其他昂贵内部状态，系统 SHALL NOT 再重复占用新的槽位

### Requirement: 命中限制时必须按策略拒绝
当某个客户端 IP 已达到允许的昂贵请求并发上限时，系统 SHALL 将新的昂贵请求视为策略拒绝，而不是内部处理失败。

注：这里的 `REFUSED` 选择基于 RFC 1035 第 4.1.1 节与 RFC 8499 第 3 节对 `REFUSED` / `SERVFAIL` 语义的推断。`REFUSED` 更符合“因 policy reasons 拒绝特定 requester”的场景；`SERVFAIL` 更符合服务器尝试处理但因自身问题失败的场景。

#### Scenario: 超限的昂贵请求被拒绝
- **WHEN** 一个客户端请求尝试进入昂贵路径，且该客户端 IP 已经达到配置的昂贵请求并发上限
- **THEN** 系统 SHALL 在额外昂贵工作开始前停止处理该请求
- **AND** 响应 SHALL 使用 DNS RCODE `REFUSED`

#### Scenario: 拒绝行为不改变 readiness 语义
- **WHEN** 系统因为某个客户端 IP 超过并发上限而拒绝一个昂贵请求
- **THEN** 这种 per-client 策略拒绝 SHALL NOT 扩展 runtime readiness/phase 模型，也不得引入新的健康状态

### Requirement: 昂贵请求槽位必须在昂贵解析完成时释放
昂贵请求并发槽位 SHALL 只跟踪一个前台请求的昂贵解析阶段，并在该阶段完成后及时释放。

#### Scenario: 槽位在慢响应写回完成前释放
- **WHEN** 一个客户端请求已经完成昂贵解析工作，且最终答案已经确定
- **THEN** 系统 SHALL 在响应写回剩余部分完成之前释放该客户端的昂贵请求槽位

#### Scenario: 错误退出会释放昂贵请求槽位
- **WHEN** 一个已经占用昂贵请求槽位的请求通过错误路径、重试耗尽路径或终态失败路径退出
- **THEN** 系统 SHALL 在该请求结束前释放对应客户端的昂贵请求槽位

### Requirement: 保护能力的可观测性必须以聚合为先
系统 SHALL 为昂贵请求保护暴露有界可观测性，帮助 operator 识别策略压力，同时在第一版中避免引入高基数指标面。

#### Scenario: 限制命中会增加聚合指标
- **WHEN** 一个昂贵请求因为客户端 IP 超过配置并发上限而被拒绝
- **THEN** 系统 SHALL 为该事件增加聚合保护指标计数
- **AND** 这些指标 SHALL NOT 使用原始客户端 IP 作为标签

#### Scenario: 限制命中日志按客户端限频
- **WHEN** 同一个客户端 IP 在短时间窗口内反复触发昂贵请求并发拒绝
- **THEN** 系统 SHALL 对该客户端 IP 的 warning 日志进行限频
- **AND** 每条实际输出的 warning 日志 SHALL 包含当前 inflight 值、配置的 limit、路径分类以及该时间窗口内的 suppressed-event count
