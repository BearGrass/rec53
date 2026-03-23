# 运维说明

[English](operations.md) | 中文

本文说明 rec53 实例运行后的日常运维操作。

## 运行模式

前台验证：

```bash
./rec53ctl run
```

服务部署：

```bash
sudo ./rec53ctl install
sudo ./rec53ctl upgrade
sudo ./rec53ctl uninstall
sudo ./rec53ctl uninstall --purge
```

## 日志

默认二进制运行时会把日志写到 `./log/rec53.log`。

安装为 systemd 服务后：

- `rec53ctl run` 会把日志写到 stderr，方便直接看到启动失败
- `rec53ctl install` 默认把运行日志写到 `/var/log/rec53/rec53.log`
- 可以通过 `LOG_FILE=/your/path/rec53.log sudo ./rec53ctl install` 覆盖日志路径
- 进程内日志轮转限制为 1 MB 主文件 + 最多 5 个备份，因此应用日志会被限制在几 MB 内，不会无限增长

常用查看方式：

```bash
tail -f /var/log/rec53/rec53.log
tail -f ./log/rec53.log
journalctl -u rec53 -f
```

`journalctl` 仍适合查看服务启动失败、崩溃和 stderr 输出，但正常运行日志以配置的 `LOG_FILE` 为准。

## 指标

默认指标地址是 `http://<host>:9999/metric`。

同一个运维 HTTP 面也提供 readiness probe：

```bash
curl -s -i http://127.0.0.1:9999/healthz/ready
```

返回体刻意保持很小：

```text
ready=true
phase=warming
```

推荐这样理解：

- `phase` 只是有界的生命周期上下文，不是完整 health taxonomy
- `ready=false` 且 `phase=cold-start` 表示 listener bind 还没完成
- `ready=true` 表示 rec53 已经可以接收 DNS 流量
- `phase=warming` 表示 warmup 还在进行，但已经可以服务
- `phase=steady` 表示 listener 启动完成，warmup 已不再活跃
- `phase=shutting-down` 表示 rec53 正在主动退出服务

snapshot restore 也仍在同一个模型内：

- snapshot 文件缺失时，启动仍然走正常的 cold-start 路径
- snapshot restore 失败会降级成 cold-cache 启动，但不会引入新的 probe phase
- listener bind 完成后，readiness 仍只按 warmup 状态进入 `warming` 或 `steady`

建议关注：

- 查询量
- 返回码
- 端到端延迟
- 按客户端昂贵路径拒绝
- nameserver 质量
- 启用 XDP 时的相关计数器

## 单客户端昂贵请求并发保护

rec53 现在可以按客户端 IP 保护昂贵解析工作。

第一版只在请求离开本地快路径时生效：

- forwarding miss，必须真正发出外部 forwarding 查询
- cache miss，必须继续进入 iterative 解析

以下廉价路径不计入限制：

- `hosts` 命中
- 本地即可完成的 forwarding hit
- cache hit

配置方式：

```yaml
dns:
  expensive_request_limit_mode: "enabled"
  expensive_request_limit: 0
```

运维上建议这样理解：

- 正式产品模式只有 `disabled` 和 `enabled`
- `0` 表示使用内置默认阈值：`runtime.NumCPU()`
- 一个前台请求最多只占 1 个槽位，即使内部有 fanout 或子查询
- 超限请求返回 `REFUSED`
- 日志按客户端 IP 限频，并带 suppressed-event count
- 第一版只提供指标和日志观测，不进入 TUI 的 per-IP 展示

### Prometheus 抓取示例

```yaml
scrape_configs:
  - job_name: "rec53"
    metrics_path: /metric
    scrape_interval: 5s
    static_configs:
      - targets:
          - "127.0.0.1:9999"
```

仓库中也提供了示例文件 `etc/prometheus.yml`。

### 启动后先检查什么

```bash
curl -s http://127.0.0.1:9999/metric | head
curl -s http://127.0.0.1:9999/healthz/ready
```

然后在 Prometheus 里确认：

- target 状态为 `UP`
- `rec53_query_counter` 持续增长
- `rec53_response_counter` 的返回码符合预期
- 真实查询后 `rec53_latency` 的桶有数据

### Probe 示例

适合 systemd 周边脚本的本地检查：

```bash
until curl -fsS http://127.0.0.1:9999/healthz/ready >/tmp/rec53.ready; do sleep 1; done
cat /tmp/rec53.ready
```

容器风格的 readiness probe：

```yaml
readinessProbe:
  httpGet:
    path: /healthz/ready
    port: 9999
```

建议把这个 probe 只当 readiness 使用：

- 是否接流量，主要看 HTTP status
- 返回体只用来区分有限几个生命周期状态
- 不要把 `phase` 当成日志、指标或事故诊断的替代品

### 首发部署关注点

- 查询速率
- SERVFAIL 比例
- p99 延迟
- `rec53_expensive_request_limit_total` 的策略压力
- `rec53_ipv2_p50_latency_ms` 中退化的上游 IP
- 仅在明确启用 XDP 时关注 XDP 指标

详见 [指标说明](../metrics.zh.md)。

## pprof

仅在排障时开启：

```yaml
debug:
  pprof_enabled: true
  pprof_listen: "127.0.0.1:6060"
```

示例：

```bash
go tool pprof http://127.0.0.1:6060/debug/pprof/heap
```

请把 `pprof_listen` 绑定在 loopback。

## 缓存快照

快照可以降低重启后的冷启动影响：

```yaml
snapshot:
  enabled: true
  file: /var/lib/rec53/cache-snapshot.json
```

建议：

- 确认快照路径可写
- 把快照视为启动优化，而不是正确性来源
- 确认关闭流程正常结束，确保快照能真正写出

## 可选功能

只有在默认 Go 路径稳定后再考虑：

- `dns.listeners > 1` 启用 `SO_REUSEPORT`
- `snapshot.enabled: true`
- `debug.pprof_enabled: true`
- `xdp.enabled: true`

## 升级策略

推荐：

```bash
sudo ./rec53ctl upgrade
```

这会保留配置并更新二进制。升级后建议验证：

```bash
systemctl status rec53
dig @127.0.0.1 -p 5353 example.com
```

## 安装与卸载安全性

`rec53ctl` 默认是保守模式：

- `install` 不会覆盖现有 systemd unit 或二进制，除非它们已被标记为 `rec53ctl` 管理
- `uninstall` 会移除受管的 unit 和二进制，但默认保留配置和日志
- 只有在你明确要删除受管配置和日志文件时，才使用 `sudo ./rec53ctl uninstall --purge`

这样可以减少误删用户自管文件或覆盖无关系统资源的风险。
