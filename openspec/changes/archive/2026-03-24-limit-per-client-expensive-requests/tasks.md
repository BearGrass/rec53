## 1. 请求级 limiter 模型

- [x] 1.1 在 `ServeDNS` 入口尽早提取并规范化 `client IP` 为 `netip.Addr`，实现 request-scoped holder，并通过请求生命周期/上下文传递，标记当前前台请求是否已经持有昂贵路径槽位；同时在 `ServeDNS` 顶部保留防御性 `defer` 兜底释放。
- [x] 1.2 使用分片 `map[netip.Addr]*clientState` 实现 per-client 昂贵请求 limiter；`clientState` 第一版最小字段集为 `inflight int`、`lastLogAt time.Time`、`suppressedCount int`。
- [x] 1.3 让 shard 数量第一版直接取 `runtime.NumCPU()`，并明确这一选择与活跃并发访问/锁竞争相关，而不是与累计 IP 种数直接挂钩。
- [x] 1.4 实现 limiter 的 `disabled` 与启用后的真实拒绝模式。
- [x] 1.5 落实单次 acquire 语义以及“昂贵解析完成即释放”的行为，并为开发期方案 2 对比验证预留 would-refuse 记录路径，确保正式产品模式不暴露额外运行模式。

## 2. 状态机集成

- [x] 2.1 定义轻量 `expensivePath` 枚举，并让 limiter 接口直接使用 `netip.Addr + expensivePath`，避免热路径字符串比较与地址重复解析。
- [x] 2.2 将昂贵路径 acquire 集成到 forwarding 外部查询入口，并通过 request-scoped holder 保证 forwarding miss 恰好消耗一个 per-client 槽位。
- [x] 2.3 将昂贵路径 acquire 集成到 `CACHE_LOOKUP_MISS` 之后的递归路径，并通过 request-scoped holder 保证递归型昂贵请求恰好消耗一个 per-client 槽位。
- [x] 2.4 将统一请求出口放在 `ServeDNS` 中的 `Change(stm)` 返回之后，立即根据 request-scoped holder 显式释放已占用槽位，并保留 `defer` 作为兜底，确保所有终态成功、重试耗尽和错误退出路径都不会重复释放或泄漏。

## 3. 可观测性与验证

- [x] 3.1 为昂贵请求限流命中增加聚合指标和 per-client 限频 warning 日志，且不使用原始 client IP 作为指标标签；开发期对比验证阶段可额外记录 would-refuse 事件。
- [x] 3.2 增加聚焦单元测试，覆盖快路径绕过、forwarding 路径 acquire、递归路径 acquire、超限 `REFUSED`，以及成功/错误退出时的槽位释放。
- [x] 3.3 运行格式化和最小相关测试。
- [x] 3.4 对比验证 cache hit、forwarding miss、iterative miss 在未触发真实拒绝时的吞吐与延迟变化。

## 4. 开发期对比验证与切换条件

- [x] 4.1 使用现有 benchmark / 压测方法，对比 limiter 关闭与方案 2 开发期验证开启时的 cache hit、forwarding miss、iterative miss 吞吐与延迟。
- [x] 4.2 明确“可以切换到真实 `REFUSED` 拒绝”的性能门槛、观察信号与回归验证范围，并记录在设计或开发文档中。

## 5. 文档与规格同步

- [x] 5.1 仅更新必改文档：`docs/user/operations.md`、`docs/user/operations.zh.md`，说明单 IP 昂贵请求并发保护的适用范围、默认阈值来源、日志抑制策略，以及第一版不纳入 TUI 的边界。
- [x] 5.2 仅更新必改文档：`docs/user/troubleshooting.md`、`docs/user/troubleshooting.zh.md`，说明如何区分策略拒绝 `REFUSED` 与真实处理失败 `SERVFAIL`，并明确这是基于 RFC 1035 第 4.1.1 节与 RFC 8499 第 3 节对两者语义的推断。
- [x] 5.3 仅更新必改文档：`docs/metrics.md`、`docs/metrics.zh.md`，补充正式聚合指标、日志抑制口径，以及“不使用 raw client IP label”的约束；如保留开发期 would-refuse 计数，应明确其验证性质而非正式长期契约。
