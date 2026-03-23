# rec53

[English](README.md) | 中文

rec53 是一个用 Go 编写的轻量级递归 DNS 解析器，适合部署在单机、宿主机或集群节点上，作为节点本地解析器替代操作系统自带 resolver，而不是作为中心化递归 DNS 集群。

## 项目定位

- 默认推荐能力：迭代解析、hosts 权威应答、forwarding、缓存、warmup、metrics
- 可选增强能力：缓存快照、pprof、SO_REUSEPORT 多监听器
- 平台相关能力：Linux 下的 XDP/eBPF 缓存快速路径

## 发布范围

`v1.0.0` 的定位是面向个人用户和简单 IT 场景：

- 单机或节点本地递归 DNS
- 开发机、家庭实验环境、小型内网部署
- 由运维显式管理的 Linux systemd 部署

它当前不定位为：

- 公网开放递归服务
- 中心化递归 DNS 集群
- 企业级高可用 DNS 平台

现阶段请把 XDP 当作可选增强，`v1.0.0` 的发布基线仍然是默认 Go 路径。

## 环境准备

- Go `1.25.0` 或更高版本
- 用于克隆仓库的 `git`
- 用于后续验证步骤的 `dig`
- 只有在计划使用 `./rec53ctl install` 时才需要 `systemd`

## 下载依赖

这个仓库不内置 vendor 目录。项目依赖由 `go.mod` / `go.sum` 声明，并由 Go 工具链下载。

如果你想先把依赖全部下载到本地：

```bash
go mod download
```

如果你更希望按需下载，下面任一命令都会自动拉取缺失依赖：

```bash
./rec53ctl build
./rec53ctl top
go test ./...
```

如果当前环境需要模块代理，请先设置 `GOPROXY`，例如 `export GOPROXY=https://proxy.golang.org,direct`。

## 快速开始

推荐流程：

```bash
# 1. 生成配置模板
./rec53ctl config

# 2. 检查并修改 config.yaml

# 3. 构建
./rec53ctl build

# 4. 前台运行
./rec53ctl run

# 5. 验证 DNS 应答
dig @127.0.0.1 -p 5353 example.com
dig @127.0.0.1 -p 5353 example.com AAAA

# 6. 可选：打开本地运维 TUI
./rec53ctl top
```

`./generate-config.sh` 仍然保留，但现在只是 `./rec53ctl config` 的兼容包装。

`./rec53ctl config` 只负责生成初始 `config.yaml`，它本身不会下载 Go 依赖。

手动运行：

```bash
mkdir -p dist && go build -o dist/rec53 ./cmd
./dist/rec53 --config ./config.yaml
```

## 最小配置

```yaml
dns:
  listen: "127.0.0.1:5353"
  metric: ":9999"
  log_level: "info"

warmup:
  enabled: true
  timeout: 5s
  duration: 5s
```

建议的部署路径：

- 先用 `./rec53ctl run` 做本地验证
- 通过 `./rec53ctl install` 部署为 systemd 服务
- 在默认 Go 路径稳定前，不要把 XDP 当作首选上线方案

## 核心能力

- 从根服务器开始的完整迭代解析
- `A` / `AAAA` / `CNAME` 本地 hosts 权威应答
- 最长后缀匹配的 forwarding 规则
- UDP / TCP 双协议 DNS 服务
- 基于 TTL 的缓存和否定缓存
- 用于降低冷启动影响的 NS warmup
- Prometheus 指标和可选 pprof
- 优雅关闭和可选缓存快照恢复

## 运维入口

推荐使用 `rec53ctl`：

```bash
./rec53ctl config
./rec53ctl build
./rec53ctl run
./rec53ctl top
sudo ./rec53ctl install
sudo ./rec53ctl upgrade
sudo ./rec53ctl uninstall
sudo ./rec53ctl uninstall --purge
```

安装后的服务默认把应用日志写到 `/var/log/rec53/rec53.log`。前台 `rec53ctl run` 会把日志打到 stderr，方便直接看到启动失败信息。

主要 CLI 参数：

| 参数 | 默认值 | 说明 |
|---|---|---|
| `--config` | 必填 | YAML 配置文件 |
| `-listen` | `127.0.0.1:5353` | DNS 监听地址 |
| `-metric` | `:9999` | 指标地址 |
| `-log-level` | `info` | `debug`、`info`、`warn`、`error` |
| `-no-warmup` | `false` | 禁用 NS 预热 |
| `-rec53.log` | `./log/rec53.log` | 日志文件路径 |
| `-version` | `false` | 输出版本后退出 |

## 文档导航

用户文档：

- [快速开始](docs/user/quick-start.md)
- [配置说明](docs/user/configuration.zh.md)
- [运维说明](docs/user/operations.zh.md)
- [rec53top](docs/user/rec53top/README.zh.md)
- [故障排查](docs/user/troubleshooting.zh.md)
- [观测看板布局](docs/user/observability-dashboard.zh.md)
- [运维排查清单](docs/user/operator-checklist.zh.md)

开发者文档：

- [开发文档索引](docs/dev/README.md)
- [架构说明](docs/architecture.md)
- [贡献指南](docs/dev/contributing.md)
- [测试说明](docs/dev/testing.md)
- [发布清单](docs/dev/release.md)

参考文档：

- [指标说明](docs/metrics.zh.md)
- [测试文档索引](docs/testing/README.md)
- [基准测试](docs/testing/benchmarks.md)
- [物理直连 XDP 基准测试报告（2026-03-19）](docs/testing/xdp-physical-benchmark-2026-03-19.zh.md)
- [性能回归说明](docs/testing/perf-regression.md)
- [编码约定](docs/dev/conventions.md)
- [路线图](docs/roadmap.md)
