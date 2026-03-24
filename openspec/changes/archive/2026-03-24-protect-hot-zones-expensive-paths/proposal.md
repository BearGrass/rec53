## Why

rec53 已经具备 per-client 昂贵请求并发保护，但这还不足以覆盖“多个 requester 共同把同一个热点名字空间推入昂贵路径”的场景。在企业 IDC / 内网部署模型下，这类压力既可能来自泛域名或随机子域名攻击，也可能来自某个业务域的短时异常热点；它们的共同点不是“谁在打”，而是“都在把同一个 zone 的新请求持续推入昂贵解析路径”。

仅靠 per-client 保护无法解决这个问题，因为攻击或异常流量可以分散在多个 IP 上。另一方面，这次变更也不打算直接扩展成全局保险丝或上游出包保护：如果压力分散在很多不同 zone 上，那更适合留给后续 upstream/global 保护版本处理。

因此现在适合先提出一个最小的 zone 保护 proposal：只针对单热点 zone 的新昂贵路径进入资格做保护，在保持 cheap path 可继续服务的前提下，减少同一热点名字空间对整体解析资源的拖累。

## What Changes

- 增加一个“单热点 zone 的昂贵路径入口保护”能力，用来识别并短期保护当前最突出的热点 zone。
- zone 保护只作用于收包后的昂贵路径入口，不扩展到上游出包并发保护，也不承担多 zone 全局流量治理职责。
- “昂贵路径”定义沿用上一版：forwarding 外部查询，以及 `CACHE_LOOKUP_MISS` 之后进入 iterative / upstream 的请求。
- 处于保护状态的热点 zone，只阻止新的昂贵路径进入；仍可由 `hosts`、forwarding hit、cache hit 等廉价路径完成的请求不受影响。
- 常态轻量记录不追求精确归因，而是先按 coarse business-root key 做粗粒度昂贵占用记录；只有在未命中 forwarding zone 和基础后缀时，才回退到三级域名。
- zone 业务根优先级为：命中的 forwarding zone、默认值加追加值构成的基础后缀集合、以及最后兜底的三级域名。
- 默认基础后缀首版列表与当前 warmup 默认后缀保持一致，不额外内置新的环境 suffix；如部署环境需要更多私有后缀，继续通过追加机制补充。
- 热点 zone 从短时昂贵流量中动态选择，单层下钻基于最近几个短窗口的 occupancy-time 简单求和结果，而不是单窗口瞬时值。
- zone 选择遵循“最小充分热点后缀”方向：若某个 child 在聚合窗口中占据父域昂贵占用的主要部分，则保护 child；否则保护父域本身。若某个 child 占据父域 `100%` 的观察流量，则直接选它并停止继续下钻。
- 热点候选需要在多个连续短窗口内成立后，才进入保护，主要目的是给正常业务的瞬时启动和初始化流量留缓冲。
- observe mode 只在系统接近机器承受上限时触发：主指标使用短窗口内平均昂贵并发数（由 occupancy-time 归一化得到），guardrail 使用总体 CPU 利用率。
- 第一版阈值先固定为内部常量，不向 operator 暴露独立调参项；当前收敛方向是 `5s` 短窗口、最近 `3` 个窗口简单求和、`avg_expensive_concurrency >= 0.75 * NumCPU()` 且整机 CPU `>= 70%` 才进入 observe mode，child 至少占父域聚合 occupancy 的 `80%` 才下钻，候选需连续 `3` 个观察窗口成立后进入保护，并在全局昂贵占用回落到 pre-trigger baseline 的 `1.05x` 以内时退出保护。

## Capabilities

### New Capabilities
- `hot-zone-expensive-path-protection`: 对短时间内持续把同一个热点 zone 推入昂贵路径的流量做入口保护，只拒绝新的昂贵路径，不封禁 cheap path。

## Impact

- 受影响代码：`server/` 请求入口、forwarding / iterative 昂贵路径接入点，以及与热点观测相关的 `monitor/` 指标和日志。
- 受影响行为：当系统进入 observe mode 且某个 zone 在多个连续短窗口内持续表现为最突出的昂贵路径热点时，新的昂贵请求将被 `REFUSED` 拒绝进入昂贵流程，但 cheap path 仍继续服务；当全局昂贵占用回落到 observe 触发前的正常基线附近时，保护退出。
- 受影响文档/规格：roadmap 中 `v1.3.2` 的主线说明，以及后续围绕热点 zone 保护的 OpenSpec design/spec/tasks。
