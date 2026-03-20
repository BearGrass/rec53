# 指标说明

Prometheus 是 rec53 的主观测接口。这个文档既给开发，也给运维/部署使用者看，约定了指标名、标签集合和指标语义；这些内容都应被视为稳定的运维接口，而不是临时调试输出。

对应英文版见 [docs/metrics.md](./metrics.md)。

## 暴露地址与采集配置

默认指标地址为 `http://<host>:9999/metric`。可以通过 `-metric` CLI 参数或配置文件中的 `dns.metric` 字段修改。

Prometheus 抓取配置示例：

```yaml
scrape_configs:
  - job_name: "rec53"
    metrics_path: /metric
    scrape_interval: 5s
    static_configs:
      - targets:
          - "127.0.0.1:9999"
```

仓库中的 `etc/prometheus.yml` 也使用同样的模式。

## 标签基数规则

`v1.1.1` 新增的观测指标统一遵循以下约束：

- 只允许使用有限枚举标签，例如 `stage`、`type`、`code`、`result`、`reason`、`path`
- 不允许把原始域名、请求 ID、完整 upstream 列表、自由文本错误消息作为标签
- 单个 IP 标签只保留在像 `rec53_ipv2_*` 这类上游集合受控的指标上

这对两类读者都重要：

- 对开发来说，标签是兼容性契约，不是临时调试字段
- 对运维来说，这保证 dashboard 和告警能长期安全聚合，不会因为高基数失控

## 给运维先看的健康检查

首轮巡检优先看这些信号：

- `rec53_query_counter` 是否在真实流量下持续增长
- `rec53_response_counter{code="SERVFAIL"}` 是否异常偏高
- `rec53_cache_lookup_total` 是否仍有稳定 hit，而不是几乎全 miss
- `rec53_snapshot_operations_total` 是否频繁出现 load/save 失败
- `rec53_upstream_failures_total` 是否被 `timeout` 或 `bad_rcode` 主导
- 只有在开启 XDP 时，才解读 `rec53_xdp_status` 和 `rec53_xdp_*`

推荐面板布局见 [Observability Dashboard](./user/observability-dashboard.md)，事故优先排查顺序见 [Operator Checklist](./user/operator-checklist.md)。

## 给开发先看的诊断入口

当你在排查代码回归、性能变化或行为变化时，先看这些面：

- request/response 总量，确认流量和结果形态
- cache lookup 结果分布，解释延迟和 upstream 压力变化
- snapshot load/save 结果，解释重启后首批查询质量变化
- upstream failure 和 winner path，解释 timeout 与尾延迟变化
- state machine stage/failure 计数，判断变化发生在哪个解析阶段
- XDP sync/cleanup 指标，解释 fast path 和 Go path 的差异

## PromQL 示例

### 给运维/用户看

```promql
# 查询速率
rate(rec53_query_counter[1m])

# SERVFAIL 比例
rate(rec53_response_counter{code="SERVFAIL"}[5m]) / rate(rec53_response_counter[5m])

# 端到端 P99 延迟
histogram_quantile(0.99, sum by (le) (rate(rec53_latency_bucket[5m])))

# cache miss 占比
rate(rec53_cache_lookup_total{result="miss"}[5m]) / rate(rec53_cache_lookup_total[5m])

# 最近 15 分钟 snapshot 失败次数
increase(rec53_snapshot_operations_total{result="failure"}[15m])

# upstream timeout 速率
rate(rec53_upstream_failures_total{reason="timeout"}[5m])

# 开启 XDP 时的 cache hit 比例
rec53_xdp_cache_hits_total / (rec53_xdp_cache_hits_total + rec53_xdp_cache_misses_total)
```

### 给开发看

```promql
# positive hit / negative hit / miss 的分布
sum by (result) (rate(rec53_cache_lookup_total[5m]))

# 重启后 snapshot 跳过条目的原因分布
increase(rec53_snapshot_entries_total{operation="load"}[30m])

# 上游 bad rcode 的类型分布
sum by (rcode) (rate(rec53_upstream_failures_total{reason="bad_rcode"}[5m]))

# Happy Eyeballs 胜出路径分布
sum by (path) (rate(rec53_upstream_winner_total[5m]))

# 最常进入的状态机阶段
topk(10, increase(rec53_state_machine_stage_total[10m]))

# 终态失败原因分布
sum by (reason) (increase(rec53_state_machine_failures_total[10m]))
```

## 指标目录

### 核心请求指标

| 指标 | 类型 | Labels | 主要受众 | 说明 |
|------|------|--------|----------|------|
| `rec53_query_counter` | Counter | `stage`, `type` | 两者 | 请求计数 |
| `rec53_response_counter` | Counter | `stage`, `type`, `code` | 两者 | 响应计数 |
| `rec53_latency` | Histogram | `stage`, `type`, `code` | 两者 | 端到端延迟，单位毫秒 |

### Cache 指标

| 指标 | 类型 | Labels | 主要受众 | 说明 |
|------|------|--------|----------|------|
| `rec53_cache_lookup_total` | Counter | `result` | 两者 | cache lookup 结果，如 `positive_hit`、`negative_hit`、`delegation_hit`、`miss` |
| `rec53_cache_entries` | Gauge | — | 运维 | 当前 Go cache 条目数 |
| `rec53_cache_lifecycle_total` | Counter | `event` | 开发 | cache 生命周期事件，如 `write`、`delete_expired`、`flush` |

### Snapshot 指标

| 指标 | 类型 | Labels | 主要受众 | 说明 |
|------|------|--------|----------|------|
| `rec53_snapshot_operations_total` | Counter | `operation`, `result` | 两者 | snapshot load/save 尝试次数，按 `success`、`failure`、`not_found` 等结果分类 |
| `rec53_snapshot_entries_total` | Counter | `operation`, `result` | 两者 | snapshot 条目数量，如 `saved`、`imported`、`skipped_expired`、`skipped_corrupt`、`skipped_non_dns`、`skipped_pack_error` |
| `rec53_snapshot_duration_ms` | Histogram | `operation`, `result` | 开发 | snapshot load/save 耗时，单位毫秒 |

### Upstream 指标

| 指标 | 类型 | Labels | 主要受众 | 说明 |
|------|------|--------|----------|------|
| `rec53_upstream_failures_total` | Counter | `reason`, `rcode` | 两者 | upstream 失败分类，如 `timeout`、`transport_error`、`context_canceled`、`bad_rcode` |
| `rec53_upstream_fallback_total` | Counter | `result` | 两者 | 备用 upstream fallback 结果，如 `success`、`failure`、`unavailable` |
| `rec53_upstream_winner_total` | Counter | `path` | 开发 | upstream 竞速的胜出路径，如 `single`、`primary`、`secondary` |
| `rec53_ipv2_p50_latency_ms` | Gauge | `ip` | 两者 | 上游 RTT P50 |
| `rec53_ipv2_p95_latency_ms` | Gauge | `ip` | 开发 | 上游 RTT P95 |
| `rec53_ipv2_p99_latency_ms` | Gauge | `ip` | 开发 | 上游 RTT P99 |

### XDP 指标

| 指标 | 类型 | Labels | 主要受众 | 说明 |
|------|------|--------|----------|------|
| `rec53_xdp_status` | Gauge | — | 运维 | XDP fast path 状态，`0` 表示禁用或不可用，`1` 表示激活 |
| `rec53_xdp_cache_hits_total` | Gauge | — | 运维 | XDP BPF cache hit 总量 |
| `rec53_xdp_cache_misses_total` | Gauge | — | 运维 | XDP BPF cache miss 总量 |
| `rec53_xdp_pass_total` | Gauge | — | 开发 | 被转交给 Go 栈的包数量 |
| `rec53_xdp_errors_total` | Gauge | — | 开发 | XDP BPF 处理错误总量 |
| `rec53_xdp_cache_sync_errors_total` | Counter | `reason` | 两者 | Go 到 BPF cache 同步失败，按 `key_build`、`value_build`、`update` 分类 |
| `rec53_xdp_cleanup_deleted_total` | Counter | — | 运维 | cleanup 删除的过期 XDP 条目总数 |
| `rec53_xdp_entries` | Gauge | — | 运维 | cleanup 对账后当前活跃 XDP cache 条目数 |

> `rec53_xdp_cache_hits_total`、`rec53_xdp_cache_misses_total`、`rec53_xdp_pass_total`、`rec53_xdp_errors_total` 是 Gauge，而不是 Counter，因为 Go 会定期从 BPF per-CPU 数组读取绝对值并直接设置。

### State Machine 指标

| 指标 | 类型 | Labels | 主要受众 | 说明 |
|------|------|--------|----------|------|
| `rec53_state_machine_stage_total` | Counter | `stage` | 开发 | 状态机阶段进入次数 |
| `rec53_state_machine_failures_total` | Counter | `reason` | 开发 | 状态机终态失败原因，如 `query_upstream_error`、`cname_cycle`、`max_iterations` |

## 标签稳定性与兼容性说明

> **Breaking change (v0.5.0):** `rec53_query_counter`、`rec53_response_counter`、`rec53_latency` 已移除 `name`（原始 FQDN）标签，以消除无界基数并减少热路径分配。

如果你的 PromQL、Grafana 面板或告警规则还引用 `name`，需要删除该选择器。当前指标不再提供按域名聚合能力；如果确实需要 per-domain 可见性，请使用 DNS 查询日志。

## 给贡献者的约束

修改指标时，请遵守：

- 保持 labels 有界且可复用
- 趋势类信号优先用 counter / histogram，当前状态类信号再用 gauge
- 同时更新本文件和 `docs/metrics.md`
- 同步检查 dashboard、PromQL 示例和运维文档是否也需要更新
