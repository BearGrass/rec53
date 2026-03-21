# 运维检查清单

当 rec53 看起来退化时，用这份清单。先从你已经能观察到的症状入手，再看第一层指标，最后再去看日志或代码。

## SERVFAIL 比例高

先看：

- `rate(rec53_response_counter{code="SERVFAIL"}[5m]) / rate(rec53_response_counter[5m])`
- `sum by (reason) (rate(rec53_upstream_failures_total[5m]))`
- `sum by (reason) (increase(rec53_state_machine_failures_total[10m]))`

再看：

- `journalctl -u rec53 -n 100 --no-pager`
- `/var/log/rec53/rec53.log`

## 延迟回退

先看：

- `histogram_quantile(0.99, sum by (le) (rate(rec53_latency_bucket[5m])))`
- `sum by (result) (rate(rec53_cache_lookup_total[5m]))`
- `sum by (path) (rate(rec53_upstream_winner_total[5m]))`
- `rec53_ipv2_p50_latency_ms`

再看：

- 缓存条目是否异常偏少
- cleanup 或 flush 活动是否突然飙升
- 上游失败是否让更多请求走冷路径

## Snapshot 恢复不对

先看：

- `increase(rec53_snapshot_operations_total{operation="load"}[1h])`
- `increase(rec53_snapshot_entries_total{operation="load"}[1h])`
- `histogram_quantile(0.99, sum by (le) (rate(rec53_snapshot_duration_ms_bucket{operation="load"}[1h])))`

再看：

- snapshot 文件路径权限
- 上一个进程的关闭行为
- snapshot load 日志里是否有损坏条目或缺失文件

## 上游超时或响应不稳定

先看：

- `sum by (reason) (rate(rec53_upstream_failures_total[5m]))`
- `sum by (rcode) (rate(rec53_upstream_failures_total{reason="bad_rcode"}[5m]))`
- `sum by (result) (rate(rec53_upstream_fallback_total[5m]))`
- `rec53_ipv2_p50_latency_ms`

再看：

- 到上游或根服务器的网络可达性
- 是否有一个或多个上游 IP 长期退化
- fallback 是成功了还是也失败了

## XDP 看起来坏了

先看：

- `rec53_xdp_status`
- `rec53_xdp_cache_hits_total`
- `rec53_xdp_cache_misses_total`
- `sum by (reason) (rate(rec53_xdp_cache_sync_errors_total[5m]))`
- `rec53_xdp_entries`

再看：

- attach 或退回 Go 路径时的日志
- 网卡接口名和权限配置
- Go 路径缓存面板是否仍然健康

## 还是不清楚

如果没有某一类症状能直接解释问题：

- 看 `topk(10, increase(rec53_state_machine_stage_total[10m]))`
- 看 `sum by (reason) (increase(rec53_state_machine_failures_total[10m]))`
- 把当前仪表盘和一个已知健康的时间窗口对比
