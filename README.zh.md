# rec53

用 Go 实现的迭代 DNS 解析器，采用状态机架构，内置 IP 质量追踪和 Prometheus 指标。
rec53 的产品定位是轻量级端侧递归解析器，面向个人设备与生产集群终端节点（含宿主机）部署。它用于替代节点上的操作系统内置递归解析器，增强本地解析能力并分担企业内或运营商集中式递归 DNS 基础设施压力，而非作为集中式递归集群本身。

[English](README.md) | 中文

## 功能特性

- **完整迭代解析** — 从根服务器出发逐级解析，不依赖上游转发
- **本地 Hosts 权威应答** — 从配置文件直接返回 A/AAAA/CNAME 静态记录（带 AA 标志），优先于缓存和上游查询
- **转发规则** — 将指定域名后缀的查询转发至专用上游 DNS 服务器（最长后缀匹配）
- **UDP/TCP 双协议** — 同一端口同时监听 UDP 和 TCP
- **SO_REUSEPORT 多监听器** — 通过 `SO_REUSEPORT` 在同一地址绑定 N 个 UDP+TCP 监听器对，实现内核级负载均衡（仅 Linux；`dns.listeners` 配置项）
- **状态机架构** — 清晰可审计的 9 状态解析流水线
- **IPQualityV2** — 基于滑动窗口的延迟直方图，支持自动故障恢复
- **Happy Eyeballs 并发查询** — 同时向最优和次优 NS 发起查询，取最先响应的结果
- **Bad Rcode 故障切换** — 主 NS 返回 SERVFAIL / REFUSED / FORMERR 时自动切换备用 NS 重试
- **EDNS0 与 UDP 截断** — 4096 字节 EDNS0 缓冲区；UDP 超限时设置 TC 标志并逐步裁剪 Answer
- **基于 TTL 的缓存** — 深拷贝安全缓存，支持否定缓存（NXDOMAIN/NODATA）
- **NS 预热** — 启动时预填充 IP 池，降低冷启动延迟
- **缓存快照** — 优雅关闭时持久化全量 DNS 缓存，重启时自动恢复，消除冷启动延迟
- **Prometheus 指标** — 每次查询和每个 NS 服务器均可观测
- **优雅关闭** — 基于 context 的取消机制，5 秒超时

---

> **v0.5.0 破坏性变更：** Prometheus 指标 `rec53_query_counter`、
> `rec53_response_counter` 和 `rec53_latency` 已移除 `name` 标签（原始 FQDN）。
> 此变更消除了无界标签基数，降低了热路径内存分配。**迁移方式：** 从引用这些指标的
> PromQL 查询或 Grafana 面板中移除 `name` 选择器。
> 详见 [CHANGELOG.md](CHANGELOG.md)。

## 快速开始

### 使用 rec53ctl（推荐）

`rec53ctl` 是 rec53 的单入口运维脚本，覆盖构建、运行、安装、升级、卸载等完整生命周期。
推荐流程是先通过 `./generate-config.sh` 生成配置模板，检查并修改 `config.yaml`，再使用 `rec53ctl` 运行或安装服务。

```bash
# 1. 生成默认配置（首次运行）
./generate-config.sh

# 2. 按环境修改 config.yaml

# 3. 构建二进制（输出到 dist/rec53）
./rec53ctl build

# 4. 前台运行，便于验证配置
./rec53ctl run

# 5. 安装为 systemd 服务（需要 root）
sudo ./rec53ctl install

# 6. 升级运行中的服务（构建 + 热替换 + 自动回滚）
sudo ./rec53ctl upgrade

# 7. 卸载服务和文件（需要 root）
sudo ./rec53ctl uninstall
```

常用选项：

```bash
# 安装时强制覆盖已有 /etc/rec53/config.yaml
sudo ./rec53ctl install --force-config

# 跳过编译，直接用已有 dist/rec53 升级
SKIP_BUILD=1 sudo ./rec53ctl upgrade

# 使用自定义配置文件前台运行
CONFIG_FILE=./my-config.yaml ./rec53ctl run
```

环境变量可覆盖默认路径：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `INSTALL_DIR` | `/usr/local/bin` | 二进制安装目录 |
| `CONFIG_DIR` | `/etc/rec53` | 配置目录 |
| `BINARY_NAME` | `rec53` | 二进制文件名 |
| `SERVICE_NAME` | `rec53` | systemd 服务名 |
| `BUILD_OUTPUT` | `dist/rec53` | 构建输出路径 |

### 手动运行（不使用 rec53ctl）

手动运行时也建议沿用同样的配置流程：先生成 `config.yaml`，按环境修改后，再通过 `--config` 启动 `rec53`。

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

# 查看日志（日志写入文件，不输出到 stdout）
tail -f ./log/rec53.log
```

---

## CLI 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--config` | *(必填)* | YAML 配置文件路径 |
| `-listen` | `127.0.0.1:5353` | DNS 监听地址 |
| `-metric` | `:9999` | Prometheus 指标地址 |
| `-log-level` | `info` | 日志级别：`debug`、`info`、`warn`、`error` |
| `-no-warmup` | `false` | 禁用启动时 NS 预热 |
| `-rec53.log` | `./log/rec53.log` | 日志文件路径（日志仅写入文件，不输出到 stdout） |
| `-version` | `false` | 打印版本号后退出 |

> **注意**：`-listen`、`-metric`、`-log-level` 仅在其值与默认值不同时才覆盖配置文件。例如，若配置文件中已设置 `log_level: debug`，使用 `-log-level info` 无法将其覆盖回 `info`。

---

## 配置文件

```yaml
dns:
  listen: "127.0.0.1:5353"
  metric: ":9999"
  log_level: "info"
  # upstream_timeout: 1500ms  # 迭代解析时每次上游查询的超时时间。
                              # 默认值：1.5s。高延迟网络可调高至 3-5s。
                              # 最小值：100ms。
  # listeners: 0             # UDP+TCP 监听器对数，通过 SO_REUSEPORT 绑定。
                              # 0 或 1 = 单监听器对（经典模式）。>1 = N 个并行监听器对。
                              # 建议值：与 CPU 核心数一致。仅 Linux 有效，其他平台自动忽略。

warmup:
  enabled: true
  timeout: 5s        # 预热阶段每次查询的超时时间
  duration: 5s       # 预热总时间预算
  concurrency: 0     # 0 = 自动（min(NumCPU*2, 8)）；>0 = 手动指定
  tlds:              # 留空则使用内置 30 个 TLD 默认列表
    - com
    - net
    - org

# 本地静态 DNS 记录 — 权威应答（AA=true），优先于缓存和迭代解析。
# 优先级：hosts > forwarding > cache > 迭代解析。
# 支持类型：A、AAAA、CNAME。TTL 默认 60 秒。
hosts:
  - name: db.internal
    type: A
    value: 10.0.0.5
    ttl: 300
  - name: ipv6.internal
    type: AAAA
    value: "fd00::1"
  - name: alias.internal
    type: CNAME
    value: real.internal

# 将指定域名后缀的查询转发至专用上游 DNS 服务器。
# 最长后缀匹配。转发结果不写入缓存。按顺序尝试所有上游；
# 全部失败时返回 SERVFAIL（不回退迭代解析）。
forwarding:
  - zone: corp.example.com
    upstreams:
      - 192.168.1.1:53
      - 192.168.1.2:53
  - zone: internal
    upstreams:
      - 10.0.0.53:53

# 缓存快照：优雅关闭时持久化全量 DNS 缓存，下次启动时自动恢复。
# 消除重启后的冷启动延迟（通常 300ms+）。默认禁用；需设置 file 路径方可生效。
# snapshot:
#   enabled: false
#   file: ""   # 例如 /var/lib/rec53/cache-snapshot.json 或 ~/.rec53/cache-snapshot.json
```

| `snapshot` 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `enabled` | bool | `false` | 启用快照保存/恢复 |
| `file` | string | `""` | 快照文件路径；为空则即使 `enabled: true` 也不生效 |

默认预热 30 个高流量 TLD，覆盖全球 85%+ 的域名注册量。如需自定义列表，请在 `warmup.tlds` 中指定；留空则使用内置默认值。

---

## 性能分析 / pprof

rec53 内置受控的 pprof HTTP 端点，支持堆内存、CPU 和 goroutine 分析。**默认关闭**，使用独立 HTTP server（与 metrics 端口分离）。

**配置** (`config.yaml`):

```yaml
debug:
  pprof_enabled: true              # 默认: false
  pprof_listen: "127.0.0.1:6060"   # 默认: 127.0.0.1:6060
```

> **安全提示**: 切勿在生产环境将 `pprof_listen` 绑定到 `0.0.0.0`。pprof 会暴露运行时内部数据。远程访问请使用 SSH 隧道: `ssh -L 6060:127.0.0.1:6060 user@server`。

**使用方式**:

```bash
# 堆内存分析（内存分配热点）
go tool pprof http://127.0.0.1:6060/debug/pprof/heap

# CPU 分析（采样 30 秒）
go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30

# Goroutine 转储
go tool pprof http://127.0.0.1:6060/debug/pprof/goroutine

# 浏览器查看所有 profile
open http://127.0.0.1:6060/debug/pprof/
```

pprof server 参与优雅关闭流程——当 rec53 收到 SIGINT/SIGTERM 时停止接受新请求。

---

## Docker

```bash
# 构建镜像
docker build -t rec53 .

# 独立运行（挂载日志目录以便从宿主机访问日志）
docker run -d \
  -p 5353:5353/udp \
  -p 5353:5353/tcp \
  -p 9999:9999 \
  -v $(pwd)/log:/dist/log \
  rec53

# 使用 Docker Compose 运行（含 Prometheus + node-exporter）
cd single_machine && docker-compose up -d
```

---

## 已知限制

- 未实现 DNSSEC 验证
- 不支持 DoT / DoH
- `www.huawei.com` 等复杂 CNAME 链在最终 A/AAAA 解析失败时可能返回 SERVFAIL
- 日志**仅写入文件**（默认路径 `./log/rec53.log`），不输出到 stdout/stderr。可通过 `-rec53.log /path/to/file.log` 修改路径。Docker 部署时需挂载日志目录（如 `-v $(pwd)/log:/dist/log`）才能从宿主机访问日志。

---

## 文档

- [`docs/architecture.md`](docs/architecture.md) — 系统设计、状态机、缓存、IP 池
- [`docs/benchmarks.md`](docs/benchmarks.md) — 延迟、QPS、内存基准数据
- [`docs/recursive-dns-test-plan.md`](docs/recursive-dns-test-plan.md) — 完整递归 DNS 测试方案（功能 + 性能 + 发布门禁）
- [`docs/perf-regression.md`](docs/perf-regression.md) — 性能回归流程与验收标准
- [`tools/validate-v050.sh`](tools/validate-v050.sh) — 一键性能验证脚本（dnsperf + pprof，Linux 开发流程）
- [`docs/metrics.md`](docs/metrics.md) — Prometheus 指标与 PromQL 示例
- [`docs/sdns-comparison.md`](docs/sdns-comparison.md) — 与 sdns 的功能对比
- [`.rec53/CONVENTIONS.md`](.rec53/CONVENTIONS.md) — 代码规范与模式
- [`.rec53/ROADMAP.md`](.rec53/ROADMAP.md) — 路线图与计划特性

## 参考资料

- [miekg/dns](https://github.com/miekg/dns) — Go DNS 协议库
- [RFC 1034](https://datatracker.ietf.org/doc/html/rfc1034) — DNS 概念与设施
- [RFC 1035](https://datatracker.ietf.org/doc/html/rfc1035) — DNS 实现与规范
- [RFC 2308](https://datatracker.ietf.org/doc/html/rfc2308) — DNS 查询否定缓存
