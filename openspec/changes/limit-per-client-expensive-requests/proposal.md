## Why

rec53 已经完成了清晰的 readiness 契约，但仍缺少一种有界方式来防止单个客户端长期占用过多昂贵解析路径。在目标企业 IDC 部署模型下，`client IP` 是一个现实可用的第一版公平性维度，因此现在适合先定义一个最小 per-client 保护模型，再继续展开更宽的资源保护工作。

## What Changes

- 增加一个 per-client 昂贵请求并发保护能力，限制单个客户端 IP 同时运行多少个高成本请求。
- 将“昂贵请求”定义为离开本地快路径、进入 forwarding 外部查询或 `cache miss` 后递归解析的请求。
- 要求 limiter 对每个符合条件的前台请求只计数一次，命中策略拒绝时返回 `REFUSED`，并在昂贵解析工作结束后释放槽位。
- 为保护命中增加有界可观测性，通过聚合指标和限频日志暴露状态，同时明确第一版不引入 per-IP TUI 视图。

## Capabilities

### New Capabilities
- `per-client-expensive-request-protection`: 在不惩罚本地快路径命中的前提下，限制每个客户端 IP 的昂贵解析并发。

### Modified Capabilities

## Impact

- 受影响代码：`server/` 状态机请求流、forwarding 路径、递归解析路径、请求生命周期上下文，以及 `monitor/` 指标与日志。
- 受影响行为：昂贵路径上的超限客户端 IP 将被按策略拒绝，同时增加新的运维可见指标与日志字段。
- 受影响文档/规格：roadmap 以及面向 operator 的资源保护预期。
