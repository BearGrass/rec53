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

建议关注：

- 查询量
- 返回码
- 端到端延迟
- nameserver 质量
- 启用 XDP 时的相关计数器

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
```

然后在 Prometheus 里确认：

- target 状态为 `UP`
- `rec53_query_counter` 持续增长
- `rec53_response_counter` 的返回码符合预期
- 真实查询后 `rec53_latency` 的桶有数据

### 首发部署关注点

- 查询速率
- SERVFAIL 比例
- p99 延迟
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
