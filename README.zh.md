# rec53

用 Go 实现的迭代 DNS 解析器，采用状态机架构，内置 IP 质量追踪和 Prometheus 指标。

[English](README.md) | 中文

## 功能特性

- **完整迭代解析** — 从根服务器出发逐级解析，不依赖上游转发
- **UDP/TCP 双协议** — 同一端口同时监听 UDP 和 TCP
- **状态机架构** — 清晰可审计的 7 状态解析流水线
- **IPQualityV2** — 基于滑动窗口的延迟直方图，支持自动故障恢复
- **基于 TTL 的缓存** — 深拷贝安全缓存，支持否定缓存（NXDOMAIN/NODATA）
- **NS 预热** — 启动时预填充 IP 池，降低冷启动延迟
- **Prometheus 指标** — 每次查询和每个 NS 服务器均可观测
- **优雅关闭** — 基于 context 的取消机制，5 秒超时

---

## 快速开始

```bash
# 构建
go build -o rec53 ./cmd

# 生成默认配置（首次运行）
./generate-config.sh

# 使用配置文件运行
./rec53 --config ./config.yaml

# 带参数覆盖运行
./rec53 --config ./config.yaml -listen 0.0.0.0:53 -metric :9099 -log-level debug

# 测试解析
dig @127.0.0.1 -p 5353 google.com
dig @127.0.0.1 -p 5353 google.com AAAA
```

---

## CLI 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--config` | *(必填)* | YAML 配置文件路径 |
| `-listen` | `127.0.0.1:5353` | DNS 监听地址（覆盖配置文件） |
| `-metric` | `:9999` | Prometheus 指标地址（覆盖配置文件） |
| `-log-level` | `info` | 日志级别：`debug`、`info`、`warn`、`error` |
| `-no-warmup` | `false` | 禁用启动时 NS 预热 |
| `-version` | `false` | 打印版本号后退出 |

CLI 参数优先级高于配置文件。

---

## 配置文件

```yaml
dns:
  listen: "127.0.0.1:5353"
  metric: ":9999"
  log_level: "info"

warmup:
  enabled: true
  timeout: 5s        # 预热阶段每次查询的超时时间
  duration: 5s       # 预热总时间预算
  concurrency: 0     # 0 = 自动（min(NumCPU*2, 8)）；>0 = 手动指定
  tlds:              # 留空则使用内置 30 个 TLD 默认列表
    - com
    - net
    - org
```

默认预热 30 个高流量 TLD，覆盖全球 85%+ 的域名注册量。如需自定义列表，请在 `warmup.tlds` 中指定；留空则使用内置默认值。

---

## Docker

```bash
# 构建镜像
docker build -t rec53 .

# 独立运行
docker run -d \
  -p 5353:5353/udp \
  -p 5353:5353/tcp \
  -p 9999:9999 \
  rec53

# 使用 Docker Compose 运行（含 Prometheus + node-exporter）
cd single_machine && docker-compose up -d
```

---

## 已知限制

- 未实现 DNSSEC 验证
- 不支持 DoT / DoH
- `www.huawei.com` 等复杂 CNAME 链在最终 A/AAAA 解析失败时可能返回 SERVFAIL

---

## 文档

- [`docs/architecture.md`](docs/architecture.md) — 系统设计、状态机、缓存、IP 池
- [`docs/benchmarks.md`](docs/benchmarks.md) — 延迟、QPS、内存基准数据
- [`docs/metrics.md`](docs/metrics.md) — Prometheus 指标与 PromQL 示例
- [`docs/sdns-comparison.md`](docs/sdns-comparison.md) — 与 sdns 的功能对比
- [`.rec53/CONVENTIONS.md`](.rec53/CONVENTIONS.md) — 代码规范与模式
- [`.rec53/ROADMAP.md`](.rec53/ROADMAP.md) — 路线图与计划特性

## 参考资料

- [miekg/dns](https://github.com/miekg/dns) — Go DNS 协议库
- [RFC 1034](https://datatracker.ietf.org/doc/html/rfc1034) — DNS 概念与设施
- [RFC 1035](https://datatracker.ietf.org/doc/html/rfc1035) — DNS 实现与规范
- [RFC 2308](https://datatracker.ietf.org/doc/html/rfc2308) — DNS 查询否定缓存
