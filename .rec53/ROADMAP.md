# Roadmap

## Version History

| Version | Date     | Highlights                                                   |
|---------|----------|--------------------------------------------------------------|
| dev     | 2026-03  | Graceful shutdown, comprehensive tests, E2E test suite       |
| -       | 2026-03  | IP quality tracking with prefetch                            |
| -       | 2026-03  | Prometheus metrics integration                               |
| -       | 2026-03  | Docker Compose deployment                                    |

## Current Version: dev

### Features

- Recursive DNS resolution from root servers
- UDP/TCP dual protocol support
- LRU cache with TTL-based expiration (5 min default)
- IP quality tracking for optimal upstream server selection
- IP prefetch for candidate servers
- Prometheus metrics endpoint
- Graceful shutdown with 5-second timeout
- CNAME loop detection
- EDNS0 support (4096-byte buffer)

### Known Issues

- [ ] `TestCNAMEChainWithValidNSDelegation` (`e2e/resolver_test.go`) 在网络条件不佳时仍可能
      因 `www.huawei.com` 的 glueless NS 链（cdnhwc1.com）解析超时而失败。
      根本原因：`resolveNSIPsConcurrently` 递归深度截断为 `depth=1`（防死锁设计，Non-Goal）。
      **已修复**：`SERVFAIL` 返回问题通过 B-004 两阶段修复解决
      （`state_machine.go:isNSRelevantForCNAME` + `state_extract_glue.go:handle`）；
      E2E 测试已缓解：客户端超时延长至 15s，A 记录缺失不计为失败，仅 SERVFAIL 触发错误。
      **剩余风险**：极端网络抖动下 15s 内仍可能无法完成 cdnhwc1.com 的 NS 迭代解析。

---

## v0.1.0 — Hosts 本地权威 + Forwarding 转发规则

**目标**：让 rec53 支持本地权威应答（`*.dev.local`）和内网 DNS 转发（`*.corp.example.com`），
成为可替代 systemd-resolved 的本地解析器。

### 状态机变更

在 `STATE_INIT` 之后插入两个新状态：

```
STATE_INIT → HOSTS_LOOKUP → FORWARDING_CHECK → CACHE_LOOKUP → CLASSIFY_RESP → ...
```

新增终止态 `FORWARD_UPSTREAM`：转发查询结果**不写入** `globalDnsCache`，避免内外网数据污染。

### 配置格式（新增 `hosts:` 和 `forwarding:` 块）

```yaml
hosts:
  - domain: "*.dev.local"
    type: A
    value: "192.168.1.100"
  - domain: "gateway.dev.local"
    type: CNAME
    value: "gateway.internal"   # target 由 rec53 继续递归解析，不直接返回 CNAME 记录

forwarding:
  - match: "corp.example.com"      # 匹配本身及所有子域（默认语义）
    upstreams: ["10.0.0.1:53", "10.0.0.2:53"]
    strategy: roundrobin
  - match: "=corp.example.com"     # = 前缀：仅精确匹配本身
    upstreams: ["10.0.0.1:53"]
    strategy: roundrobin
  - match: "*.corp.example.com"    # * 前缀：仅匹配子域，不含本身
    upstreams: ["10.0.0.1:53"]
    strategy: roundrobin
```

### 任务清单

- [ ] `server/state_hosts.go` — 实现 `HOSTS_LOOKUP` 状态：精确匹配 + 通配符匹配
  - A/AAAA 记录：直接构造应答
  - CNAME 记录：将 target 入队递归解析，最终返回完整链
- [ ] `server/state_forwarding.go` — 实现 `FORWARDING_CHECK` 状态：规则匹配 + `FORWARD_UPSTREAM` 终止态
  - 匹配优先级：精确（`=`）> 通配子域（`*`）> 包含子域（默认）
  - `strategy: roundrobin` — 轮询选择 upstream
- [ ] `server/state_machine.go` — 在状态转移表中插入新状态，补充新状态常量
- [ ] `cmd/rec53.go` — 扩展 `Config` 结构体，新增 `Hosts []HostsRule` 和 `Forwarding []ForwardingRule`
- [ ] `config.yaml` — 补充示例配置（注释说明三种匹配语义）
- [ ] 单元测试 `server/state_hosts_test.go` 和 `server/state_forwarding_test.go`
- [ ] 更新 `docs/architecture.md`（状态机图）

---

## v0.2.0 — 学习型预热 Round 2

**目标**：跨重启记忆热点注册域名的 NS 记录，缩短冷启动时的首次查询延迟。

### 设计

- **统计粒度**：eTLD+1（注册域名级别，如 `github.com`）
  - 使用 `golang.org/x/net/publicsuffix` 提取
- **衰减 LFU**：每次查询 `score += 1.0`；后台每小时 `score × decay_factor`（默认 0.9）
- **两级预热**：
  - Round 1（已有）：13 个根服务器 TLD NS 查询
  - Round 2（新增）：从学习文件读取热点域名，并发查询其 NS 记录
- **学习文件**：JSON，覆写（非追加），防止磁盘无限增长；默认路径 `~/.rec53/learned.json`

### 配置格式（新增 `learned_warmup:` 块）

```yaml
learned_warmup:
  enabled: true
  file: "~/.rec53/learned.json"
  top_n: 200            # 只预热得分最高的 N 个域名
  decay_factor: 0.9     # 每小时衰减系数
  flush_interval: 300   # 秒，每隔多久将内存中的统计覆写到文件
```

### 任务清单

- [ ] 引入依赖 `golang.org/x/net/publicsuffix`（`go.mod` + `go.sum`）
- [ ] `server/learned_warmup.go` — 实现衰减 LFU 计数器（读/写/衰减/flush）
- [ ] `server/warmup.go` — 在 Round 1 结束后启动 Round 2 并发 NS 查询
- [ ] `cmd/rec53.go` — 扩展 `Config`，新增 `LearnedWarmup LearnedWarmupConfig`
- [ ] 在 DNS 查询成功返回路径上记录 eTLD+1 命中（`state_query_upstream.go` 或 `state_machine.go`）
- [ ] 单元测试 `server/learned_warmup_test.go`
- [ ] 更新 `docs/architecture.md`

---

## v0.3.0 — IP 池 GC

**目标**：防止 `globalIPPool` 无限增长，避免长期运行内存泄漏。

### 设计

- `IPQualityV2` 新增 `lastSeen time.Time` 字段（原子更新，每次 `RecordLatency` / `RecordFailure` 时刷新）
- 后台 goroutine 定期（默认每 30 min）遍历 IP 池，删除超过阈值未访问的条目
- **根服务器豁免**：`utils/root.go` 中硬编码的 13 组根服务器 IP 不参与 GC
- 阈值通过配置项 `dns.ip_pool_stale_duration` 控制

### 配置格式（`dns:` 块新增字段）

```yaml
dns:
  listen: "127.0.0.53:53"
  ip_pool_stale_duration: "2h"   # 超过此时长未访问的 IP 将被 GC 清除
```

### 任务清单

- [ ] `server/ip_pool_quality_v2.go` — `IPQualityV2` 新增 `lastSeen` 字段
- [ ] `server/ip_pool.go` — 实现 `GCStaleIPs(threshold time.Duration)` 方法（根服务器豁免）
- [ ] `server/ip_pool.go` — 启动后台 GC goroutine（每 30 min 调用一次，可 context 取消）
- [ ] `cmd/rec53.go` — `Config.DNS` 新增 `IPPoolStaleDuration string`，解析并传入 IP 池
- [ ] 单元测试 `server/ip_pool_gc_test.go`（含豁免验证）
- [ ] 更新 `docs/architecture.md`

---

## v0.4.0 — `sync.Pool` 内存优化

**目标**：通过对象池复用 `dns.Msg` 等高频分配对象，降低 GC 压力和 Stop-the-World 暂停时间。

### 背景

当前代码无任何 `sync.Pool` 使用。每次缓存 miss 的完整查询路径上会分配 3~4 个 `dns.Msg`：
入口 `reply`、上游查询 `newQuery`、Happy Eyeballs 两次 `Copy()`、NS 解析每个 goroutine 2 个。
高频写入的 `getCacheCopy()`/`setCacheCopy()` 也会深拷贝整个 `dns.Msg`（含 RR slice），
是内存分配的主要来源。

goroutine 池**不适用**于本项目：查询路径本身是同步状态机，临时 goroutine 生命周期 ms 级，
在终端低 QPS 场景下创建代价可忽略；强行池化会大幅提升代码复杂度而收益极低。

### 优化项（按优先级）

| 优先级 | 对象 | 当前方式 | 优化方式 |
|--------|------|----------|----------|
| 高 | `dns.Msg` | `new(dns.Msg)` / `msg.Copy()` 每次堆分配 | `sync.Pool` + `msg.Reset()` 后归还 |
| 中 | 全局入站 semaphore | 无上限 | `make(chan struct{}, N)` 防洪水攻击 |
| 低 | state 结构体 | `new()` 每次迭代分配（24 字节） | pprof 确认热点后再决定是否池化 |

### 设计约束

- `sync.Pool` 中取出的 `dns.Msg` 必须先调用 `SetReply()` 或手动 `Reset()`，再使用
- 归还前必须确保不再有任何引用持有该 `dns.Msg`（尤其注意 cache 写入路径已做 `Copy()`）
- Happy Eyeballs 的 `query.Copy()` 在 goroutine 竞速结束后归还到 pool

### 任务清单

- [ ] `server/msg_pool.go` — 封装 `dns.Msg` 的 `sync.Pool`，提供 `AcquireMsg()` / `ReleaseMsg()` API
- [ ] `server/server.go` — 入口 `reply` 改用 `AcquireMsg()`，`ServeDNS` 返回后 `ReleaseMsg()`
- [ ] `server/state_query_upstream.go` — `newQuery`、Happy Eyeballs Copy、NS 解析的 `req/resp` 改用 pool
- [ ] `server/state_machine.go` — 全局入站 semaphore（容量可配置，默认 512）
- [ ] 单元测试 `server/msg_pool_test.go`（验证并发场景下无 double-free）
- [ ] 用 `go test -bench` 前后对比 `BenchmarkServeDNS` 分配次数（`-benchmem`）

---

## v0.5.0 — systemd 集成

**目标**：提供一键 install/uninstall 脚本，让 rec53 可作为 systemd 服务替代 systemd-resolved。


### 默认监听地址

从 `127.0.0.1:5353` 改为 `127.0.0.53:53`，与 systemd-resolved 默认地址保持一致。

### 产物

```
deploy/
├── rec53.service    # systemd 单元文件
├── install.sh       # 安装脚本（支持非 root 用户通过 sudo 运行）
└── uninstall.sh     # 卸载脚本
```

### `install.sh` 流程

1. `go build -o /usr/local/bin/rec53 ./cmd`
2. 停止并禁用 `systemd-resolved`
3. 将 `/etc/resolv.conf` 指向 `127.0.0.53`
4. 安装并启动 `rec53.service`

### `uninstall.sh` 流程

1. 停止并禁用 `rec53.service`
2. 恢复并启动 `systemd-resolved`
3. 恢复 `/etc/resolv.conf`

### 任务清单

- [ ] `deploy/rec53.service` — systemd 单元文件（`Restart=on-failure`，`After=network.target`）
- [ ] `deploy/install.sh` — 安装脚本（含 sudo 权限检查）
- [ ] `deploy/uninstall.sh` — 卸载脚本
- [ ] `config.yaml` — 默认监听地址改为 `127.0.0.53:53`
- [ ] 更新 `README.md`（安装步骤说明）

---

## v1.0.0 — DNSSEC + DoT/DoH

**目标**：达到生产可用的安全性和传输加密标准。

### 任务清单

- [ ] DNSSEC 验证（完整信任链，从根到叶）
- [ ] DNS over TLS (DoT) 支持（RFC 7858）
- [ ] DNS over HTTPS (DoH) 支持（RFC 8484）
- [ ] 并发向多个 nameserver 查询（减少单点超时影响）
- [ ] 查询频率限制（防滥用）

---

## Future

### 长期目标

- DNS over QUIC (DoQ) 支持
- Response Policy Zones (RPZ)
- 高可用集群（多节点协调缓存）
- 查询日志与分析（可接入 ELK / ClickHouse）
- Web 管理面板
