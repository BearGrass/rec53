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
| v1.1.0 | 2026-03 | done | 热点复核完成，确认仅保留低风险 alloc quick win |
| v1.1.1 | 2026-03 | done | 指标体系、运维入口、dashboard/checklist 收敛完成 |
| v1.1.2 | planned | target | 本地运维 TUI 看板 |
| v1.1.3 | planned | target | 运行韧性与节点级高可用 |
| v1.1.4 | planned | target | DNSSEC 设计与预研 |

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

### 当前探索主线 — v1.1.2 本地运维 TUI 看板

**目标**：基于现有 `/metric` 数据源提供本地 CLI/TUI 看板，让开发和运维无需先部署 Grafana 也能快速查看 rec53 当前状态。

**当前观察**

- `v1.1.1` 已经补齐 cache、snapshot、upstream、XDP、state machine 的核心观测面
- 当前仍缺一个“本地可直接看”的运维界面，现有 dashboard/checklist 主要还是文档形式
- 仓库里的 `rec53ctl` 目前是脚本入口，TUI 需要单独目录和实现边界，不能直接塞进主服务路径

**本轮探索任务**

- [ ] 明确 TUI 的数据源边界与目录结构
- [ ] 确认首版最小面板：traffic、cache、snapshot、upstream、XDP、state machine
- [ ] 确认刷新频率、布局、自适应降级和不可达目标处理
- [ ] 决定首版是独立二进制还是由 `rec53ctl` 包装调用

**退出条件**

- [ ] 能明确回答“TUI 首版做什么、不做什么、代码放哪里、如何接现有 metrics”

## v1.1 版本线

`v1.1` 拆成连续小版本，先确认性能空间，再补观测，再补本地运维界面，再补运行韧性，最后评估 DNSSEC。

### v1.1.0 — 性能收敛与热点复核

**状态**：已完成。结论是当前架构下暂无值得单独成版的性能主线，只保留低风险 quick win。

**结论**

- [x] 用统一口径重跑 `dnsperf + pprof`
- [x] 覆盖 loopback、双机直连、XDP cache hit 三类场景
- [x] 对 `cpu`、`alloc_space`、`alloc_objects` 做前后对比
- [x] 给候选优化项打标签：`值得做 / 收益不足 / 风险过高`
- [x] 仅接受低风险 quick win
- [x] 落地 `cache read skip Question copy`，减少 cache hit 路径 1 alloc/op
- [x] 确认该优化更适合作为版本内收敛项，而非单独发布理由

**不继续展开**

- [x] 不承诺一定继续提速
- [x] 不引入 `sync.Pool(dns.Msg)` 一类高生命周期复杂度方案
- [x] SO_REUSEPORT 多 listener 继续保留在后续版本候选池，不作为 `v1.1.0` 继续项

### v1.1.1 — 可观测性系统化提升

**状态**：已完成。当前版本已经把指标体系提升到可运营、可定位、可解释问题的程度。

**完成结果**

- [x] cache 指标补齐：hit/miss、negative cache、expire/eviction
- [x] snapshot 指标补齐：load/save、恢复条目、跳过过期、失败数、耗时
- [x] XDP 指标补齐：entries、sync errors、cleanup deleted
- [x] upstream 指标补齐：timeout、rcode、fallback、Happy Eyeballs 胜出路径
- [x] 状态机阶段计数和失败原因聚合
- [x] 输出 metrics 文档、中文版指标文档、dashboard layout 和 operator checklist

**交付物**

- [x] `docs/metrics.md`
- [x] `docs/metrics.zh.md`
- [x] `docs/user/observability-dashboard.md`
- [x] `docs/user/operator-checklist.md`

**不做**

- [x] 不引入高基数 label
- [x] 不一次性把 tracing / logging / profiling 平台化

### v1.1.2 — 本地运维 TUI 看板

**目标**：提供一个基于现有 `/metric` 数据源的本地 CLI/TUI 看板，让开发和运维无需先部署 Grafana，也能快速查看 rec53 当前状态与退化方向。

**任务**

- [ ] 明确 TUI 的数据源边界：默认消费 `http://127.0.0.1:9999/metric`
- [ ] 为 TUI 单独开目录，避免把主服务和运维界面代码混在一起
- [ ] 设计最小可用面板：traffic、cache、snapshot、upstream、XDP、state machine
- [ ] 确定刷新频率、配色、布局和降级行为（指标缺失、XDP 未启用、目标不可达）
- [ ] 先支持单实例、本地终端使用，再评估是否扩展多 target 或历史趋势
- [ ] 补充使用文档与终端截图/示意

**不做**

- [ ] 不把 TUI 直接嵌入主服务进程
- [ ] 不依赖 Grafana、Prometheus server 或外部数据库
- [ ] 不在首版承诺交互式历史图、告警管理或多节点聚合

### v1.1.3 — 运行韧性与节点级高可用

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

### v1.1.4 — DNSSEC 设计与预研

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
