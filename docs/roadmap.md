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
| v1.1.3 | 2026-03 | done | TUI detail 信息层、发布介绍文档与开发排障语义 |
| v1.1.4 | 2026-03 | done | TUI 导航焦点、Enter 进入 detail 与键位可发现性 |
| v1.1.5 | 2026-03 | done | TUI UX polish、State Machine 收敛、单域名 trace 落地 |
| v1.1.6 | 2026-03 | done | TUI 信息密度与价值提升 |
| v1.2.0 | planned | queued | 运行韧性与节点级高可用 |
| v1.2.1 | planned | queued | DNSSEC 设计与预研 |

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

**目标**：在 `v1.1.x` 完成可观测性与本地运维收口之后，把主线切回运行韧性：增强单节点在重启、冷启动、上游抖动、配置变更时的稳定性和可判断性。

**当前判断**

- `v1.1.x` 已完成 observability/local-ops 收敛：指标体系、TUI、detail、导航、State Machine summary 和 trace mode 已闭环
- `rec53top` 已完成一轮信息密度压缩：detail 标签固定为 `Now / Window / Totals / Next / Trend`，`Next` 更接近“面板编号 + 动作”
- 当前主矛盾不再是 TUI 信息呈现，而是节点在异常、重启和冷启动时的行为是否足够稳、足够可判断
- `v1.2.0` 的收益会比继续做 TUI 小修更直接，因为它影响真实运行边界而不是界面易读性

**下一步焦点**

- [x] 收口 `v1.1.6`：detail 文案继续压短，信息密度和排版稳定性提升
- [x] 保持 aggregate TUI 与 request-scoped trace 的边界，不再把单请求路径塞回 `rec53top`
- [ ] 定义 `readiness / liveness / degraded` 的可操作语义
- [ ] 复核 warmup / snapshot / 冷启动的真实恢复路径和告警口径
- [ ] 评估多 listener、限流和资源保护是否应进入实现期

## v1.1 版本线

`v1.1` 现在明确收敛为“可观测性与本地运维”版本线：先完成指标体系，再交付本地 TUI MVP，随后完成 detail 信息层和导航闭环，最后再用一个小版本收掉 UX polish 与后续增强排序。运行韧性和 DNSSEC 不再与 `v1.1` 并行争抢主线。

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
- [x] 轻量趋势提示：只保留最近 `10-20` 个 scrape 的内存序列，辅助判断当前信号是在继续变坏还是已经回落
- [ ] 排序 / 筛选：当后续引入更多细分项时，支持按失败原因或 stage 聚焦
- [ ] 更完整的键位体系：页签、返回、帮助浮层、target 切换

### v1.1.3 — TUI 增强、发布介绍文档与开发排障能力

**状态**：已完成。原先挂在本版本尾部、但实际未交付的 roadmap/UX 收尾项，已移到 `v1.1.5`。

**目标**：把 `rec53top` 从“本地可用的 MVP 看板”推进到“可发布介绍、能辅助开发定位问题、并为后续 UX polish 留出稳定语义基础”的阶段。

**任务**

- [x] detail 增加累计计数器与 bounded since-start 视图
- [x] 统一各 detail 面板的语义层次：当前判断、累计计数、top labels、next checks
- [x] 补一份面向发布的 TUI 介绍文档，可被 README / release notes 直接引用

**不做**

- [ ] 不在这一版把 TUI 扩成完整监控平台
- [ ] 不引入多 target 聚合、外部时序库或复杂可视化历史

### v1.1.4 — TUI 导航完成度与交互体验收口

**状态**：已完成。

**目标**：让 `rec53top` 的默认使用路径从“记住 `1-6`”变成“概览聚焦 -> `Enter` -> 查看 detail -> 返回概览继续排查”，把 TUI 从可用 MVP 收口到更完整的终端操作面板。

**完成结果**

- [x] 为 overview 六面板增加稳定可见的当前焦点
- [x] 支持方向键、`j/k/l`、`Tab/Shift-Tab` 导航，并保留 `h` 为 help
- [x] 支持 `Enter` 打开当前焦点 detail，并在返回概览后保留焦点
- [x] 收口 footer/help，让当前焦点和下一步动作更易发现
- [x] 更新 TUI 用户文档和 roadmap 主线说明
- [x] 补齐 `tui/` 导航测试

**不做**

- [ ] 不在这一版引入多层 drill-down
- [ ] 不引入历史图、排序筛选或多 target

### v1.1.5 — TUI UX polish、State Machine 收敛与后续增强优先级重排

**状态**：已完成。

**目标**：把 `rec53top` 从“信息层和导航都已具备”推进到“版本线清晰、文案和布局更稳、下一批增强顺序明确”的状态；同时把 `State Machine` 从难以解释的聚合路径图收敛回易读的状态/终态信号，并把真正高价值的单域名路径追踪从 TUI 里拆出来。

**完成结果**

- [x] 梳理 TUI 后续增强候选的优先级与依赖关系
- [x] 处理文案压缩、布局密度和快捷键提示等 UX polish
- [x] 明确 `panel drill-down` 的边界、数据层次和是否作为下一优先实现项
- [x] 为 `Cache`、`Upstream`、`XDP` detail 增加轻量 drill-down 子视图
- [x] 为 detail 补充子视图切换键位与当前子视图标识
- [x] 明确“轻量趋势提示”在 `v1.1.5` 落地，并将其限制为 session-local 的当前诊断辅助
- [x] 在 detail 中加入轻量趋势提示，明确它与 Prometheus 历史监控的边界
- [x] 明确 `排序 / 筛选` 何时值得引入，而不是过早增加交互复杂度
- [x] 明确“更完整的键位体系”应拆分为页签、帮助浮层、target 切换等哪些独立子项
- [x] 收敛 `State Machine` 面板默认展示：以各状态计数器、关键终态计数器和少量摘要替代聚合路径图叙事
- [x] 更新 TUI 文案与页面说明，明确 `State Machine` 面板只回答“哪里变热/哪里结束”，不负责回答“某个域名具体怎么走”
- [x] 产出“单域名真实解析路径”能力的最小方案，作为后续独立 change 候选

**版本结论**

- `State Machine` 不再走聚合路径图方向，近期定位固定为状态热度、终态出口和失败摘要
- 单域名真实路径改为 TUI 外的 trace mode，而不是继续堆进聚合看板
- footer/help、detail 标题和术语已经压短，但 detail 里的说明文字和信息密度还有继续优化空间
- `排序 / 筛选`、多 target、更深 drill-down、历史图继续留在后续版本或 backlog

### v1.1.6 — TUI 信息密度与价值提升

**状态**：已完成。

**目标**：继续提升 `rec53top` 的实用价值，但不靠堆更多解释文字，而是通过更短的文案、更高的信息密度和更稳定的字段顺序，让页面更像 `top` 一样可扫读。

**任务**

- [x] 继续压缩 detail 文案，把长解释优先收短成短句或短提示
- [x] 固化更短的 detail 结构标签：`Now`、`Window`、`Totals`、`Next`、`Trend`
- [x] 复核 overview/detail 的字段顺序，让高频判断信号固定在最前
- [x] 把部分 `Next` 收短成“面板编号 + 动作”格式，减少阅读负担
- [x] 评估帮助浮层是否值得单独实现，以替代 footer 中过长的键位解释
- [x] 明确哪些 detail 仍然过度解释，哪些字段可以直接让数字说话

**不做**

- [x] 不重新引入聚合路径图
- [x] 不把单请求 trace 塞回 `rec53top`
- [x] 不把 `rec53top` 做成历史监控前端；长期趋势继续由 Prometheus / Grafana 承担
- [x] 不把 runtime resilience 或 DNSSEC 工作重新混入 `v1.1.x`

**版本结论**

- `rec53top` 当前版本定位已经清晰：聚合诊断负责快速扫读，单请求路径交给 trace mode
- detail 区域改成更短标签和更短 `Next` 后，可读性更接近 `top`，说明文字不再压过数字本身
- 帮助浮层暂不单独实现；当前键位量仍可控，继续保持 footer 简短化即可
- 后续若再做 TUI 增强，应优先做低成本价值项，而不是重新扩张信息层

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
