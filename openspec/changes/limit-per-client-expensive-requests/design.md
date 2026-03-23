## Context

rec53 现有实现已经区分了廉价本地路径（`hosts`、forwarding hit、cache hit）和会触发外部 DNS 工作的慢路径。当前状态机顺序是先进入 `HOSTS_LOOKUP`，然后 `FORWARD_LOOKUP`，再进入 `CACHE_LOOKUP`，只有这些快路径 miss 之后，请求才会继续进入 forwarding 外部查询或递归解析。这个顺序天然提供了一个保护昂贵工作的切点，同时不会影响高 QPS 的 cache hit 流量。

目标部署模型是企业 IDC / 内网使用，因此 `client IP` 可以作为第一版公平性维度。目标不是构建一个通用的公网流量治理系统，而是阻止某一个内部调用方同时占用过多昂贵解析路径。这个设计还需要与既有 roadmap 决策保持一致：per-client 昂贵路径保护优先于更宽泛的 upstream 并发保护，第一版明确不做 TUI 集成，且 operator 侧行为必须保持容易解释。

## Goals / Non-Goals

**Goals:**
- 限制每个客户端 IP 的昂贵解析并发，同时不影响本地快路径。
- 将 forwarding 外部查询和 `cache miss` 后的递归解析都视为昂贵路径。
- 每个符合条件的前台请求最多只计数一次，并在昂贵解析工作完成时释放槽位。
- 当 per-client 昂贵路径限制被触发时，返回策略型拒绝（`REFUSED`）。
- 增加聚合指标和限频日志，使 operator 能发现保护事件，同时避免引入高基数指标标签。
- 在未触发限速的正常业务下，把性能影响压到最小，并在真正启用拒绝前先通过开发期对比验证确认吞吐与延迟影响可接受。

**Non-Goals:**
- 这一版不实现 per-client QPS 限速。
- 不引入子网、VPCID、多维配额，或公网/NAT 公平性逻辑。
- 不把 upstream query fanout 控制混进这一版的计数模型。
- 不增加 per-IP TUI 面板或其他第一版高基数 UI 展示。

## Decisions

### 统计前台昂贵请求，而不是逐个统计 upstream query

limiter 表示的是“这个客户端当前有多少个昂贵请求正在进行”，而不是“已经发出了多少个 upstream 包”。一个请求第一次进入昂贵路径时占用一个槽位，并一直持有到昂贵解析工作完成。这样可以避免对 Happy Eyeballs、NS 子查询以及其他内部 fanout 细节重复计数。

备选方案：
- 逐个统计 upstream query 在出站资源核算上会更精确，但那属于单独的 upstream 并发工作，也会让 per-client 公平性更难解释。

### 将 forwarding 外部查询和递归解析都视为昂贵路径

第一版同时覆盖两个昂贵入口：
- 需要发出 forwarding 外部查询的路径
- `CACHE_LOOKUP_MISS` 之后进入的递归解析路径

这样可以保持策略一致：任何离开本地快路径、开始消耗外部解析能力的请求，都应纳入 per-client 保护。

备选方案：
- 只限制递归解析会更简单，但会让 forwarding 型昂贵流量处于未保护状态，并在两种同样昂贵的路径之间制造难以解释的行为差异。

### 在最早进入昂贵路径的时刻，每个请求只 acquire 一次

实现上应在能够确认请求已经离开廉价路径时，尽早占用 per-client 槽位：
- 在真正发起 forwarding 外部查询之前
- 在 `CACHE_LOOKUP_MISS` 之后、请求确认进入递归解析时

一旦一个请求已经占用了槽位，后续昂贵状态就不能重复占用。这要求有 request-scoped 的记录方式，以便 limiter 判断当前请求是否已经持有槽位。

第一版推荐使用 request-scoped holder 来承载这层语义：每个前台请求拥有一个轻量 holder，至少记录规范化后的 `clientIP netip.Addr` 以及当前请求是否已经持有昂贵路径槽位。昂贵路径入口只负责尝试 acquire，holder 负责保证“每个请求最多 acquire 一次”，而统一请求出口负责在 holder 显示已持有槽位时执行 release 兜底。这样可以减少 forwarding / iterative 各自手写 acquire/release 带来的重复与泄漏风险。

统一出口第一版建议直接放在 `ServeDNS` 中：在 `Change(stm)` 返回后立刻显式执行 `holder.ReleaseIfHeld(limiter)`，而不是等到 `WriteMsg()` 完成后再释放。这样可以保证昂贵请求槽位只覆盖真正的解析阶段，不覆盖后续的 question 恢复、UDP 截断和响应写回阶段。同时，`ServeDNS` 顶部仍保留一个防御性 `defer holder.ReleaseIfHeld(limiter)` 作为兜底，防止未来路径调整或异常返回导致槽位泄漏。

备选方案：
- 在更晚的位置、靠近每个出站调用点再 acquire，局部实现会更简单，但会让计数模型变得不一致，也更容易在 retry/fallback 场景下出错。
- 让每个状态自己分别管理 acquire/release 会增加遗漏释放、重复 acquire 和路径分裂的风险，不适合作为第一版主方案。

### 在昂贵解析结束时释放，而不是等 socket 写回结束

槽位应在昂贵解析工作完成、最终答案已经确定后立即释放，而不应继续覆盖响应组装尾部或慢客户端写回阶段。这样可以让限制真正绑定 resolver 资源占用，而不是绑定网络 I/O 行为。

实现上应优先采用“入口 acquire、统一出口兜底 release”的组织方式：昂贵路径入口决定是否第一次 acquire，而请求统一结束点根据 request-scoped holder 判断是否需要 release。这样即使中途出现错误、提前返回、重试耗尽或其他终态分支，也能降低槽位泄漏概率。

备选方案：
- 一直持有到响应完全写回虽然更容易清理，但会错误地把慢读客户端视为昂贵解析压力。

### 命中限制时使用 `REFUSED`

这种拒绝是一个绑定到 requester 维度的策略决策，因此返回码应为 `REFUSED`。这里的判断依据来自 RFC 1035 第 4.1.1 节对 `REFUSED` 的定义：name server 因 policy reasons 拒绝执行指定操作，并明确提到可能针对 particular requester 拒绝服务。RFC 8499 第 3 节延续了同样语义。相对地，`SERVFAIL` 在 RFC 1035 第 4.1.1 节和 RFC 8499 第 3 节中的语义更接近“resolver 试图处理，但因为服务端自身问题而无法处理查询”。

备选方案：
- 复用 `SERVFAIL` 会更省事，但会把主动 admission control 错误地表达成内部处理失败。

### 保持观测以聚合为先，并对日志进行限频

指标应记录每一次限制命中，但必须保持聚合形式，避免使用 `client IP` 标签。日志应按客户端 IP 限频，并包含 suppression count，这样 operator 能定位问题，同时不会因为日志本身制造第二层过载。

备选方案：
- 第一版不采用 per-IP 指标或 TUI 汇总，因为它们会在核心策略价值尚未验证前，先引入高基数运维面。

### 分片数量第一版取 `runtime.NumCPU()`

第一版的分片 `map` 数量建议直接取 `runtime.NumCPU()`。这个数量主要与“同时有多少 goroutine 会并发访问 limiter”以及“昂贵路径请求的并发度”相关，而不是与系统见过多少个客户端 IP 总量直接相关。换句话说，分片数量主要用于分散活跃并发访问下的锁竞争，而不是为历史 IP 基数做预分桶。

选择 `runtime.NumCPU()` 的原因：
- 实现简单，便于解释和维护
- 与机器实际并发执行能力保持同量级
- 第一版足以验证 limiter 在昂贵路径上的真实锁竞争情况
- 如果后续压测证明 limiter 成为热点，再考虑放大到 `2 * NumCPU()` 或更复杂的分片策略

备选方案：
- 分片数量直接与“IP 种数”绑定并不合适，因为 limiter 压力更取决于活跃昂贵请求并发，而不是累计见过多少个 IP。

### 先做开发期对比验证，再启用真实拒绝

这项能力属于负向需求，默认前提是“在未触发限速时，不能明显拉低正常业务吞吐”。因此第一版实施应先采用方案 2 作为开发期对比验证策略：先完整执行 per-client 昂贵请求并发计数、阈值判断和命中观测，但在验证阶段命中阈值时只记录“本应被拒绝”的事件，不立即返回 `REFUSED`。该策略用于开发、benchmark 和上线前验证，不作为正式产品运行模式暴露。正式能力只保留 `disabled` 与启用后的真实拒绝语义。

方案 2 的实现重点：
- 正常未超限路径仍然只保留一次 acquire / release 的固定小成本
- 命中阈值时先记 aggregate metrics 和 rate-limited logs，记录 would-refuse 事件
- 不引入排队、等待或重试扩张，保持 admission 逻辑是常数级判断
- 对比压测时，优先比较“未触发真实限速时”的 cache hit、forwarding miss、iterative miss 吞吐与延迟变化

备选方案：
- 直接启用真实 `REFUSED` 拒绝虽然实现更直接，但在没有基线压测和开发期对比数据前，难以判断是否会在未触发限速时明显伤害正常业务吞吐。

## Risks / Trade-offs

- [Forwarding 与 iterative 入口点分裂] -> 使用一个共享的 request-scoped limiter 持有模型，让两条路径遵循同一套语义。
- [异常错误/提前返回路径导致槽位泄漏] -> 确保清理逻辑集中化，并用针对成功、拒绝、重试和错误退出的测试覆盖。
- [默认阈值过低或过高] -> 基于现有昂贵路径 benchmark / 压测数据设定初始值，并提供可配置能力。
- [operator 看不出请求为何被拒绝] -> 增加聚合计数器和 per-IP 限频告警日志，包含当前 inflight、limit、path 和 suppression count。
- [未来 upstream 保护工作在语义上重叠] -> 明确这一版只约束前台昂贵请求并发，把出站 query fanout 限制留给后续 upstream 专项版本。
- [未触发限速时正常吞吐下降过多] -> 先采用方案 2 做开发期对比验证，对 cache hit、forwarding miss 和 iterative miss 做对比压测，只有退化可接受时才开启真实拒绝。
