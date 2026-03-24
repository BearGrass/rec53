## 1. 热点 zone 选择模型

- [x] 1.1 实现基础 zone key 选择顺序：优先命中 forwarding zone，其次“默认值 + 追加值”基础后缀，最后回退到三级域名，并统一成单一 coarse business-root key 语义。
- [x] 1.2 实现常态轻量记录：昂贵请求进入时只绑定一个 coarse business-root key，并按该 key 做 occupancy-time 采样。
- [x] 1.3 实现短窗口聚合与 `avg_expensive_concurrency` 计算，作为 observe mode 的主指标基础。
- [x] 1.4 在 observe mode 下实现“最近 `3` 个 `5s` 窗口简单求和 + 单层下钻”的热点选择逻辑：child 占父域聚合 occupancy 至少 `80%` 则保护 child，否则保护父域；child 占 `100%` 时直接停止下钻。
- [x] 1.5 实现“一次只选一个热点 zone”的规则，并明确候选切换与替换时机。

## 2. 保护状态与请求入口接入

- [x] 2.1 实现 observe mode 触发条件：仅在 `avg_expensive_concurrency >= 0.75 * NumCPU()` 且整机 CPU 利用率 `>= 70%` 时触发观察。
- [x] 2.2 实现热点 zone 的受保护状态模型，支持同一候选连续 `3` 个短窗口确认后进入保护，并记录 observe 触发前短时窗口的正常全局昂贵占用基线。
- [x] 2.3 将 zone 保护接入 forwarding 外部查询入口与 `CACHE_LOOKUP_MISS` 之后的 iterative 入口，确保受保护 zone 的新昂贵请求用 `REFUSED` 被拒绝进入昂贵路径。
- [x] 2.4 确保受保护 zone 仍允许 `hosts` hit、forwarding hit、cache hit 等 cheap path 正常服务。
- [x] 2.5 实现保护退出规则：当全局昂贵占用回落到 observe 触发前基线的 `1.05x` 以内时退出保护。

## 3. 可观测性与验证

- [x] 3.1 增加热点 zone 选择、observe mode、受保护状态与退出事件的聚合指标和限频日志，明确当前保护对象、触发原因与拒绝事件。
- [x] 3.2 增加聚焦测试，覆盖 coarse business-root key 选择顺序、基础后缀优先级、三级域名 fallback、observe mode 触发、简单窗口求和、单层下钻、父域保护、连续窗口确认、cheap-path 绕过与 `REFUSED` 拒绝。
- [x] 3.3 使用现有 benchmark / 压测方法验证：未进入 observe/protect mode 时，轻量记录逻辑不会明显伤害正常 cheap path 与普通 miss 路径吞吐。

## 4. 文档与规格同步

- [x] 4.1 更新 design/spec，明确本版本只处理单热点 zone 的入口保护，不承担 multi-zone 全局压力与上游出包保护。
- [x] 4.2 更新 operator 相关文档，说明基础后缀默认值 + 追加配置、第一版内部常量、observe mode 触发语义、cheap-path 例外、`REFUSED` 行为与退出条件。
- [x] 4.3 在 roadmap / OpenSpec 中同步 `v1.3.2` 的实现边界、默认基础后缀策略，以及后续 `v1.3.3` / `v1.3.4` 的分工关系。
