# Roadmap

主 roadmap 只保留当前状态、接下来 1-2 个版本的计划，以及长期方向。
已完成版本与历史里程碑已归档到 [ROADMAP.archive.md](./ROADMAP.archive.md)。

## Version History

| Version | Date | Status | Highlights |
|---------|------|--------|------------|
| v0.2.0 | 2026-03 | done | 全量缓存快照，重启后首批查询直接命中 |
| v0.3.0 | 2026-03 | done | IP Pool stale prune，限制长期增长 |
| v0.4.0 | 2026-03 | done | alloc baseline、pprof 接入、低风险分配优化 |
| v0.6.0 | 2026-03 | done | XDP Cache 核心路径 |
| v0.6.1 | 2026-03 | done | XDP 指标、TTL 清理、性能验收、文档 |
| v1.0.0 | 2026-03 | done | 1.0 发布收敛，形成单机 node-local 发布基线 |
| v1.1.0 | planned | target | 性能收敛与热点复核 |
| v1.1.1 | planned | target | 可观测性系统化提升 |
| v1.1.2 | planned | target | 运行韧性与节点级高可用 |
| v1.1.3 | planned | target | DNSSEC 设计与预研 |

## Current Version: dev

### 已具备能力

- 完整递归解析：从根开始，支持 UDP/TCP、CNAME 链、Glue、EDNS0、优雅关闭
- 上游选择：IPQualityV2、4 状态健康模型、Happy Eyeballs、Bad Rcode 重试
- 缓存与冷启动：负缓存、NS 预热、全量缓存快照
- 本地策略：Hosts 本地权威、Forwarding 转发规则
- 运维能力：`rec53ctl`、Prometheus 指标、pprof、Docker Compose、XDP Cache

### 当前边界

- 当前定位仍是单机 `node-local recursive resolver`
- 不把分布式缓存集群作为近期主线
- 安全协议栈增强优先级低于性能收敛、观测和运行韧性

## 已完成归档

详细历史已移至 [ROADMAP.archive.md](./ROADMAP.archive.md)。
主 roadmap 只保留以下摘要：

- 基础能力：Hosts/Forwarding、状态机重构、综合测试与运维脚本
- 解析与缓存：负缓存、并发 NS 解析、NS 预热、全量缓存快照
- 上游质量：IPQualityV2、Happy Eyeballs、IP stale prune
- 性能与诊断：alloc baseline、pprof、热路径降分配
- 内核快路径：XDP Cache 核心、XDP 指标与 TTL 清理
- 发布收敛：`v1.0.0` tag、发布说明、用户/开发者文档重组

## Next Up

### 候选项 — SO_REUSEPORT 多 Listener 复测

**目标**：验证单 socket 是否仍是当前部署形态下的主要瓶颈，并决定这项工作应并入 `v1.1.0`，还是继续留在候选池。

**任务**

- [ ] 在真实双机场景复测 `listeners=1` 与 `listeners>1`
- [ ] 结合 `dnsperf + pprof` 判断收益是否稳定可复现
- [ ] 若收益明确，补配置、实现、文档和回归测试
- [ ] 若收益不稳定，回退到 `v1.1.0` 的热点复核结论中统一处理

**退出条件**

- [ ] 能明确回答“SO_REUSEPORT 是否值得进入后续正式版本”

## v1.1 版本线

`v1.1` 拆成连续小版本，先确认性能空间，再补观测，再补运行韧性，最后评估 DNSSEC。

### v1.1.0 — 性能收敛与热点复核

**目标**：确认当前架构下是否还有低风险、值得继续做的性能优化空间。

**任务**

- [ ] 用统一口径重跑 `dnsperf + pprof`
- [ ] 覆盖 loopback、双机直连、XDP cache hit 三类场景
- [ ] 对 `cpu`、`alloc_space`、`alloc_objects` 做前后对比
- [ ] 给候选优化项打标签：`值得做 / 收益不足 / 风险过高`
- [ ] 仅接受低风险 quick win

**不做**

- [ ] 不承诺一定继续提速
- [ ] 不引入 `sync.Pool(dns.Msg)` 一类高生命周期复杂度方案

### v1.1.1 — 可观测性系统化提升

**目标**：把现有指标体系提升到可运营、可定位、可解释问题的程度。

**任务**

- [ ] cache 指标补齐：hit/miss、negative cache、expire/eviction
- [ ] snapshot 指标补齐：load/save、恢复条目、跳过过期、失败数、耗时
- [ ] XDP 指标补齐：entries、sync errors、drop reason
- [ ] upstream 指标补齐：timeout、rcode、fallback、bad server、Happy Eyeballs 胜出路径
- [ ] 状态机阶段计数和失败原因聚合
- [ ] 输出 dashboard / operator checklist

**不做**

- [ ] 不引入高基数 label
- [ ] 不一次性把 tracing / logging / profiling 平台化

### v1.1.2 — 运行韧性与节点级高可用

**目标**：增强单节点在重启、冷启动、上游抖动、配置变更时的稳定性。

**任务**

- [ ] 定义 `readiness` / `liveness` / `degraded`
- [ ] 明确 warmup / snapshot / 冷启动行为与告警口径
- [ ] 复测多 listener 的收益与副作用
- [ ] 补齐 systemd / 容器友好的退出、重启和权限说明
- [ ] 评估限流、并发上限、资源保护策略

**不做**

- [ ] 不实现分布式缓存一致性
- [ ] 不引入中心化控制面

### v1.1.3 — DNSSEC 设计与预研

**目标**：先把 DNSSEC 的收益、边界、复杂度和上线风险说明白，再决定是否进入实现。

**任务**

- [ ] 梳理 `DO` / `AD` / `CD` 标志位语义
- [ ] 设计 trust anchor、验证失败语义、bogus/insecure 等状态处理
- [ ] 明确缓存语义与负缓存交互
- [ ] 评估是否同步引入 EDE
- [ ] 输出实现边界、依赖选择、测试矩阵和最终排期建议

**不做**

- [ ] 不在本迭代承诺完整 DNSSEC 上线
- [ ] 不把 `DoT/DoH` 与 DNSSEC 绑成同一交付包

## Open Backlog

详见 [BACKLOG.md](./BACKLOG.md)。当前仍值得持续关注的条目：

- `O-016`：AAAA (IPv6) 上游选择支持
- `O-006`：UDP 截断后的 TCP 重试
- `B-014`：Glue bailiwick 校验
- `O-022`：Response ID 校验
- `O-021`：无 glue 委派缓存策略
- `O-005`：负缓存 TTL 配置化

## Future

- DNS over QUIC (DoQ)
- Response Policy Zones (RPZ)
- 查询日志与分析
- Web 管理面板
- 多节点协调能力
