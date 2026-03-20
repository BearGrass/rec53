# Roadmap

主 roadmap 只保留当前状态、接下来 1-2 个版本的计划，以及长期方向。
已完成版本与历史里程碑已归档到 [roadmap.archive.md](./roadmap.archive.md)。

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
| v1.1.2 | 2026-03 | done | 本地运维 TUI 看板 |
| v1.1.3 | 2026-03 | done | TUI 增强、发布介绍文档与开发排障能力 |
| v1.2.0 | planned | target | 运行韧性与节点级高可用 |
| v1.2.1 | planned | target | DNSSEC 设计与预研 |

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

详细历史已移至 [roadmap.archive.md](./roadmap.archive.md)。
主 roadmap 只保留以下摘要：

- 基础能力：Hosts/Forwarding、状态机重构、综合测试与运维脚本
- 解析与缓存：负缓存、并发 NS 解析、NS 预热、全量缓存快照
- 上游质量：IPQualityV2、Happy Eyeballs、IP stale prune
- 性能与诊断：alloc baseline、pprof、热路径降分配
- 内核快路径：XDP Cache 核心、XDP 指标与 TTL 清理
- 发布收敛：`v1.0.0` tag、发布说明、用户/开发者文档重组

## Next Up

### 当前推进主线 — v1.2.0 运行韧性与节点级高可用

**目标**：在 `v1.1` 可观测性与本地运维版本线完成后，把主线切到单节点运行韧性，优先解决重启、冷启动、上游抖动、配置变更时的稳定性与可恢复性。

**当前判断**

- `v1.1.1` 到 `v1.1.3` 已经形成连续的 observability/local-ops 交付线
- `rec53top` 已具备 overview、detail、bounded since-start counters 和发布入口文档
- 下一阶段更值得投入的是 readiness / liveness / degraded、warmup 与 snapshot 行为、systemd / 容器友好退出与重启语义

**下一步焦点**

- [x] 保持 `v1.1` 主线聚焦可观测性与本地运维，不并行展开新的大主题
- [x] 将 `v1.1.3` 定义为 TUI 增强与发布介绍文档版本
- [x] 将运行韧性工作顺延到 `v1.2.0`
- [x] 将 DNSSEC 设计与预研顺延到 `v1.2.1`
- [ ] 在 `v1.2.0` 中定义 `readiness` / `liveness` / `degraded` 行为与判断口径
- [ ] 梳理 warmup / snapshot / 冷启动 / 重启的稳定性与告警边界

## v1.1 版本线

`v1.1` 现在明确收敛为“可观测性与本地运维”版本线：先完成指标体系，再交付本地 TUI MVP，再继续补 TUI 的开发排障能力、发布介绍文档与后续增强骨架。运行韧性和 DNSSEC 不再与 `v1.1` 并行争抢主线。

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

### v1.1.2 — 本地运维 TUI 看板 MVP

**状态**：已完成。

**目标**：先把 `v1.1.2` 收敛成一个可交付的 MVP：基于现有 `/metric` 数据源提供单实例、本地终端、只读的 CLI/TUI 看板，让开发和运维无需先部署 Grafana，也能快速查看 rec53 当前状态与退化方向。

**范围判断**

- [x] 当前不再继续扩版；`v1.1.2` 本身可以收敛为一个足够小的 MVP
- [x] 版本边界定为：单 target、当前状态 + 短窗口速率、固定六面板、只读
- [x] 多 target、历史趋势、交互式 drill-down 留到后续版本或 backlog

**任务拆解**

**Batch 1：边界与骨架**

- [x] 冻结命令形态、目录结构和默认 target：`http://127.0.0.1:9999/metric`
- [x] TUI 单独开目录，不混入 `server/` 和主服务启动路径
- [x] 引入专门的 Go TUI 依赖，并限制其只作用于独立命令

**Batch 2：数据采集与状态模型**

- [x] 直连抓取 `/metric`，不依赖 Grafana、Prometheus server 或外部数据库
- [x] 把六类指标收敛成固定状态模型：traffic、cache、snapshot、upstream、XDP、state machine
- [x] 在本地用连续 scrape 计算短窗口 rate/ratio，而不是只显示原始 counter
- [x] 明确三种降级状态：目标不可达、指标缺失、XDP 未启用

**Batch 3：界面与交互**

- [x] 落地固定六面板布局与状态摘要
- [x] 确定刷新频率、配色、终端宽度适配和最小交互（退出、刷新、帮助）
- [x] 确保“是否正常、哪里退化了”一眼能看出来，而不是逼用户先懂 PromQL
- [x] 在实现上保留后续 detail / drill-down / history 的扩展点，但不把它们塞进 MVP
- [x] 补上终端兼容性 fallback，避免“可启动但无显示”时完全不可用

**Batch 4：文档与验收**

- [x] 补充使用文档、自测路径和终端截图/示意
- [x] 更新 README 和 roadmap 入口
- [x] 用本地 rec53 实例做一轮手工验收，确认六面板都能读到合理状态

**不做**

- [ ] 不把 TUI 直接嵌入主服务进程
- [ ] 不接 Prometheus query API 或外部时序库
- [ ] 不在首版承诺多节点聚合、历史图、告警管理或自定义面板布局

**后续增强候选**

- [x] detail 视图：展开单个面板并补充 bounded breakdown，帮助解释当前 summary
- [x] detail 累计计数器：在短窗口 rate/ratio 之外补充 since-start counters，方便开发定位代码路径与长期热点
- [x] detail 面板语义统一：明确每个 detail 面板固定展示哪些当前值、累计值、top labels 和 next checks
- [x] 发布介绍文档：提供一份面向用户/发布说明的 TUI 功能介绍页，解释适用场景、边界和阅读方式
- [ ] panel drill-down：从 summary 进入更细的 cache / upstream / XDP 子视图
- [ ] history sparklines：在终端内显示短历史趋势，而不接外部时序库
- [ ] 排序 / 筛选：当后续引入更多细分项时，支持按失败原因或 stage 聚焦
- [ ] 更完整的键位体系：页签、返回、帮助浮层、target 切换

### v1.1.3 — TUI 增强、发布介绍文档与开发排障能力

**状态**：已完成。

**目标**：把 `rec53top` 从“本地可用的 MVP 看板”推进到“可发布介绍、能辅助开发定位问题、并为后续 UX polish 留出稳定语义基础”的阶段。

**任务**

- [x] detail 增加累计计数器与 bounded since-start 视图
- [x] 统一各 detail 面板的语义层次：当前判断、累计计数、top labels、next checks
- [x] 补一份面向发布的 TUI 介绍文档，可被 README / release notes 直接引用
- [ ] 梳理 TUI 后续增强候选的优先级与依赖关系
- [ ] 在完成信息层增强后，再处理文案压缩、布局密度和快捷键提示等 UX polish

**不做**

- [ ] 不在这一版把 TUI 扩成完整监控平台
- [ ] 不引入多 target 聚合、外部时序库或复杂可视化历史

### v1.2.0 — 运行韧性与节点级高可用

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

### v1.2.1 — DNSSEC 设计与预研

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

这些条目还没有进入明确版本排期，但仍值得持续跟踪。已经完成的历史 backlog 项不再在这里重复保留。

### Resolver And Protocol

#### [B-014] Glue 无 bailiwick 校验

- Priority: Medium
- Description: Additional 中的 glue 记录仍未做 bailiwick 校验，存在被越权 glue 影响解析路径的风险。
- Acceptance:
  提取 glue 时校验 A/AAAA 记录名字是否落在当前 zone 的 bailiwick 内。
  对 out-of-bailiwick glue 改为触发 NS 子查询解析，而不是直接采信。
  补充对应单元测试。

#### [O-021] 无 glue 时委派 NS 不缓存

- Priority: Medium
- Description: 当前只有 `Ns` 和 `Extra` 同时存在时才缓存委派，NS-only referral 仍会导致下一次重复从上层迭代。
- Acceptance:
  NS-only 响应也缓存 NS RRset。
  下次同区域解析能直接从缓存找到委派起点。

#### [O-022] Response ID 未校验

- Priority: Low
- Description: 当前未严格校验响应 ID 与请求 ID 一致，仍存在乱序或伪造响应被误接受的风险。
- Acceptance:
  校验 `newResponse.Id == newQuery.Id`。
  补充单元测试覆盖 ID mismatch。

#### [O-016] Add AAAA (IPv6) Support

- Priority: High
- Description: `getIPListFromResponse()` 仍只提取 IPv4 `A` 记录，缺少 IPv6 `AAAA` 路径支持。
- Acceptance:
  提取 `AAAA` 记录。
  更新地址选择和预取逻辑以兼容 IPv6。
  补充 AAAA 查询测试。

#### [O-006] TCP Retry for Truncated Responses

- Priority: High
- Description: UDP 响应被截断时还没有自动切到 TCP 重试。
- Acceptance:
  检测 `TC` 标志。
  截断后自动走 TCP 重试。
  覆盖大响应场景。

#### [O-005] Negative Cache TTL 配置化

- Priority: Medium
- Description: 负缓存能力已经存在，但 TTL 策略和配置化仍可继续收敛。
- Acceptance:
  明确 NXDOMAIN / NODATA TTL 来源与配置边界。
  补足相关测试场景。

#### [O-018] 状态机死循环保护增强

- Priority: Medium
- Description: 当前仍主要依赖 `MaxIterations=50`，递归深度与 delegation 深度保护可以更明确。
- Acceptance:
  添加 NS 递归深度限制。
  添加 delegation 深度跟踪。
  覆盖死循环回归测试。

### Test Coverage

#### [T-003] Negative Cache E2E Test

- Priority: High
- Description: 验证 NXDOMAIN/NODATA 被缓存后，后续相同查询直接命中 negative cache。

#### [T-004] Query Budget Exhaustion Test

- Priority: High
- Description: 构造深层 referral 链，验证 budget 耗尽时进入 `FAIL_SERVFAIL`。

#### [T-005] Timeout Retry And Server Switch Test

- Priority: High
- Description: 验证单个 NS 超时后重试，再自动切换到备用 NS 并最终成功。

#### [T-006] SERVFAIL / REFUSED Server Blacklist Test

- Priority: High
- Description: 验证 bad rcode 会标记坏服务器并切换到备用上游。

#### [T-007] Response ID Mismatch Test

- Priority: Medium
- Description: 验证错误 ID 的响应会被丢弃并继续等待正确响应。

#### [T-008] CNAME + NXDOMAIN Combination Test

- Priority: Medium
- Description: 验证含 CNAME 链且最终 `RCODE=NXDOMAIN` 的组合响应被正确保留和返回。

#### [T-009] Referral Loop Detection Test

- Priority: Medium
- Description: 验证循环委派能够被检测并返回 SERVFAIL，而不是陷入无限迭代。

#### [T-010] All NS Unreachable Test

- Priority: Medium
- Description: 验证一个区域下所有 NS 都超时或拒绝时，最终正确返回 SERVFAIL。

## Future

- DNS over QUIC (DoQ)
- Response Policy Zones (RPZ)
- 查询日志与分析
- Web 管理面板
- 多节点协调能力
