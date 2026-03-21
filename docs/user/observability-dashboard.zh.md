# 可观测性面板

这份文档定义了 rec53 的基线仪表盘布局。它优先面向运维，但开发者也可以把它当作跨版本比较时的稳定起点。

## 面板顺序

从上到下按这个顺序：

1. 流量和响应质量
2. 缓存效果
3. snapshot 启动质量
4. 上游健康
5. XDP 健康
6. 状态机集中度

## 推荐面板

### 1. 流量和响应质量

- 查询速率：`rate(rec53_query_counter[1m])`
- 响应码分布：`sum by (code) (rate(rec53_response_counter[5m]))`
- P99 延迟：`histogram_quantile(0.99, sum by (le) (rate(rec53_latency_bucket[5m])))`

### 2. 缓存效果

- 缓存 lookup 结果：`sum by (result) (rate(rec53_cache_lookup_total[5m]))`
- 缓存 miss 比例：`rate(rec53_cache_lookup_total{result="miss"}[5m]) / rate(rec53_cache_lookup_total[5m])`
- 当前缓存条目：`rec53_cache_entries`
- 缓存生命周期活动：`sum by (event) (rate(rec53_cache_lifecycle_total[5m]))`

### 3. Snapshot 启动质量

- snapshot 操作结果：`sum by (operation, result) (increase(rec53_snapshot_operations_total[1h]))`
- load 时的条目结果：`sum by (result) (increase(rec53_snapshot_entries_total{operation="load"}[1h]))`
- snapshot 耗时：`histogram_quantile(0.99, sum by (le, operation, result) (rate(rec53_snapshot_duration_ms_bucket[1h])))`

### 4. 上游健康

- 上游失败原因：`sum by (reason) (rate(rec53_upstream_failures_total[5m]))`
- bad rcode：`sum by (rcode) (rate(rec53_upstream_failures_total{reason="bad_rcode"}[5m]))`
- fallback 结果：`sum by (result) (rate(rec53_upstream_fallback_total[5m]))`
- Happy Eyeballs 胜出路径：`sum by (path) (rate(rec53_upstream_winner_total[5m]))`
- 退化的上游 IP：`rec53_ipv2_p50_latency_ms > 500`

### 5. XDP 健康

- XDP 状态：`rec53_xdp_status`
- XDP 命中率：`rec53_xdp_cache_hits_total / (rec53_xdp_cache_hits_total + rec53_xdp_cache_misses_total)`
- XDP 同步错误：`sum by (reason) (rate(rec53_xdp_cache_sync_errors_total[5m]))`
- XDP 清理删除数：`rate(rec53_xdp_cleanup_deleted_total[5m])`
- XDP 条目数：`rec53_xdp_entries`

### 6. 状态机集中度

- 最活跃阶段：`topk(10, increase(rec53_state_machine_stage_total[10m]))`
- 按原因统计的终态失败：`sum by (reason) (increase(rec53_state_machine_failures_total[10m]))`

## 事故排查顺序

- 如果响应质量差，先看流量和响应面板。
- 如果延迟上升而缓存效果差，继续看缓存和上游面板。
- 如果重启后质量差，先看 snapshot 面板，再看上游面板。
- 如果启用了 XDP，但命中率低或同步错误上升，就把 XDP 当作退化状态，再和 Go 路径的缓存面板对比。
- 如果这些都解释不了问题，再看状态机集中度，找出主故障阶段。
