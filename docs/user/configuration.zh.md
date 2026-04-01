# 配置说明

[English](configuration.md) | 中文

本文介绍 rec53 部署时主要使用的配置块。建议先从最小配置开始，等基线路径稳定后再开启可选功能。

## 最小配置

```yaml
dns:
  listen: "127.0.0.1:5533"
  metric: ":9999"
  log_level: "info"

warmup:
  enabled: true
  timeout: 5s
  duration: 5s
```

## `dns`

```yaml
dns:
  listen: "127.0.0.1:5533"
  metric: ":9999"
  log_level: "info"
  upstream_timeout: 1500ms
  upstream_concurrency_limit: 0
  hot_zone_base_suffixes: []
  listeners: 0
```

字段说明：

- `listen`：DNS 绑定地址
- `metric`：Prometheus 指标监听地址
- `log_level`：`debug`、`info`、`warn`、`error`
- `upstream_timeout`：单次上游迭代查询超时
- `upstream_concurrency_limit`：全局上游外发并发预算；`0` 表示使用 `runtime.NumCPU()` 默认值
- `hot_zone_base_suffixes`：热点 zone 业务根识别的追加 suffix 列表；默认内置的精选基础后缀仍然始终生效
- `listeners`：`0` 或 `1` 表示单监听器模式，`>1` 启用 `SO_REUSEPORT`

建议：

- 首次上线时把 `listen` 保持在 loopback
- 没有测过争用前，`listeners` 保持 `0` 或 `1`
- 只有在高延迟网络里才考虑增大 `upstream_timeout`
- 没有量化出明确上游饱和压力前，`upstream_concurrency_limit` 保持 `0`
- 只有当部署环境存在内置列表之外的私有命名后缀时，再追加 `hot_zone_base_suffixes`

## `warmup`

```yaml
warmup:
  enabled: true
  timeout: 5s
  duration: 5s
  concurrency: 0
  tlds: []
```

建议：

- 默认路径保持 `enabled: true`
- 默认情况下 `concurrency: 0`
- `tlds` 留空时使用内置精选列表

## `hosts`

静态本地权威记录会优先于 forwarding、缓存和迭代解析返回。

```yaml
hosts:
  - name: db.internal
    type: A
    value: 10.0.0.5
    ttl: 300
  - name: alias.internal
    type: CNAME
    value: real.internal
```

支持类型：

- `A`
- `AAAA`
- `CNAME`

## `forwarding`

可以把指定 zone 的查询转发到专用上游解析器。

```yaml
forwarding:
  - zone: corp.example.com
    upstreams:
      - 192.168.1.1:53
      - 192.168.1.2:53
```

规则：

- 最长后缀匹配优先
- 上游按顺序尝试
- forwarding 返回的结果不缓存
- 所有 forwarding 上游都失败后，不会再回退到迭代解析

## 可选：`snapshot`

```yaml
snapshot:
  enabled: true
  file: /var/lib/rec53/cache-snapshot.json
```

只有在默认运行路径稳定后才建议开启。它可以降低重启耗时，但不是正确性必需项。

## 可选：`debug`

```yaml
debug:
  pprof_enabled: true
  pprof_listen: "127.0.0.1:6060"
```

不要把 `pprof` 暴露到公网。

## 可选：`xdp`

```yaml
xdp:
  enabled: false
  interface: ""
```

XDP 是平台相关能力：

- 仅支持 Linux
- 需要合适的内核和权限
- 不应作为首个部署步骤

## 校验提示

如果启动失败，先检查：

- `dns.listen`
- `dns.metric`
- forwarding 上游的 `host:port`
- 当 `xdp.enabled: true` 时检查 `xdp.interface`

然后再查看 [故障排查](troubleshooting.zh.md)。
