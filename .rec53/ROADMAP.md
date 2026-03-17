# Roadmap

## Version History

| Version | Date     | Highlights                                                                        |
|---------|----------|-----------------------------------------------------------------------------------|
| dev     | 2026-03  | NS 缓存快照持久化，消除重启冷启动延迟                                             |
| dev     | 2026-03  | 并发 NS 解析（O-024），最多 5 worker，首个成功即返回                               |
| dev     | 2026-03  | Happy Eyeballs 并发上游查询，先到先得                                              |
| dev     | 2026-03  | IPQualityV2 — 滑动窗口环形缓冲 + P50/P95/P99 + 4 状态健康模型                     |
| dev     | 2026-03  | 负缓存（NXDOMAIN/NODATA）+ Bad Rcode 重试（SERVFAIL/REFUSED 换服务器）             |
| dev     | 2026-03  | NS 预热 — 30 个高流量 TLD，CPU 感知动态并发                                       |
| dev     | 2026-03  | 状态机重构 — 单文件拆分、baseState 嵌入、语义命名                                  |
| dev     | 2026-03  | Hosts 本地权威、Forwarding 转发规则、rec53ctl 运维脚本、/dev/stderr 日志           |
| dev     | 2026-03  | Graceful shutdown、综合测试套件、E2E 测试框架                                      |
| dev     | 2026-03  | IP 质量追踪 + 预取、Prometheus 指标、Docker Compose 部署                           |

## Current Version: dev

### 核心功能

- 从根服务器开始的完整递归 DNS 解析
- UDP/TCP 双协议支持
- LRU 缓存 + TTL 过期（默认 5 分钟）
- CNAME 链追踪与循环检测（MaxIterations=50）
- Glue 记录处理
- EDNS0 支持（4096 字节缓冲区）
- UDP 截断响应（TC 标志）
- 优雅关闭（5 秒超时）

### IP 质量与上游选择

- **IPQualityV2** — 64 样本滑动窗口环形缓冲区，P50/P95/P99 百分位延迟
- **4 状态健康模型** — ACTIVE → DEGRADED → SUSPECT → RECOVERED，指数退避 + 30s 后台探测
- **Happy Eyeballs** — 并发向最优 + 次优 IP 发送查询，先到先得（超时默认 1.5s）
- **IP 预取** — 候选服务器预取加速后续查询
- **Bad Rcode 重试** — SERVFAIL/REFUSED 时标记 bad server，自动切换备用 IP

### 缓存与预热

- **负缓存** — NXDOMAIN/NODATA 检测与缓存（基于 SOA Minttl，默认回退 60s）
- **NS 预热** — 启动时查询 30 个高流量 TLD 的 NS 记录，CPU 感知动态并发
- **NS 缓存快照持久化** — 关闭时保存 NS 委派缓存到 JSON 文件，启动时恢复未过期条目

### 本地策略

- **Hosts 本地权威** — A/AAAA/CNAME 静态记录，AA=true 权威应答，优先于缓存和上游
- **Forwarding 转发规则** — 最长后缀匹配转发到指定上游 DNS，结果不写缓存

### 运维与可观测性

- **rec53ctl** — 单入口运维脚本：`build` / `install` / `upgrade` / `uninstall` / `run`
- **Prometheus 指标端点** — 查询计数、延迟、缓存命中率、IP 池状态
- **Docker Compose 部署** — 一键容器化部署
- **并发 NS 解析** — 最多 5 个 worker 并发解析 NS IP，首个成功即返回

---

## ~~v0.1.0~~ ✅ 已完成 — Hosts 本地权威 + Forwarding 转发规则

**完成于**：2026-03

- `server/state_hosts.go` — `HOSTS_LOOKUP` 状态，支持 A / AAAA / CNAME，AA=true 权威应答
- `server/state_forward.go` — `FORWARDING_CHECK` 状态，最长后缀匹配，结果不写缓存
- 原子快照替换（`atomic.Pointer`）消除 hosts/forward 配置读写数据竞争
- 配置格式：`hosts:` 和 `forwarding:` 块，支持精确域名匹配

---

## ~~v0.5.0~~ ✅ 已完成 — rec53ctl 运维脚本

**完成于**：2026-03

- `rec53ctl`（项目根目录）：单入口 bash 脚本，子命令覆盖完整运维生命周期
- `build`：go 工具链检查 + `dist/rec53` 编译，支持 `GOOS`/`GOARCH` 交叉编译
- `install`：build → 复制二进制 → 处理配置（`--force-config` 强制覆盖）→ 写 systemd unit → enable/start
- `upgrade`：备份旧二进制 → build（`SKIP_BUILD=1` 可跳过）→ 热替换 → 启动失败自动回滚
- `uninstall`：stop/disable → 删 unit/二进制/config.yaml → 保留非空 CONFIG_DIR
- `run`：前台启动，`-rec53.log /dev/stderr` 日志输出到终端，`exec` 确保信号直传
- `monitor/log.go` 修复：检测 `/dev/stderr`/`/dev/stdout` 时直接写对应 fd，跳过 lumberjack 轮转

---

## ~~F-003~~ ✅ 已完成 — IPQualityV2 + Happy Eyeballs

**完成于**：2026-03-13

- **IPQualityV2** — 全面重写 IP 质量追踪系统
  - 64 样本滑动窗口环形缓冲区，替代简单均值
  - P50/P95/P99 百分位延迟计算，导出到 Prometheus
  - 复合评分 + 4 状态健康模型（ACTIVE → DEGRADED → SUSPECT → RECOVERED）
  - 指数退避 + 30 秒后台探测循环
  - 线程安全：原子操作（无锁快速路径）
- **Happy Eyeballs 并发查询** — 同时向最优 + 次优 IP 发送查询，先到先得，超时可配置（默认 1.5s）

---

## ~~B-012~~ ✅ 已完成 — 负缓存 + Bad Rcode 重试

**完成于**：2026-03-12

- **负缓存（B-012）** — NXDOMAIN/NODATA 检测和缓存，基于 SOA Minttl，默认回退 60 秒
- **Bad Rcode 重试（B-013）** — SERVFAIL/REFUSED 时标记 bad server 并尝试备用 IP

---

## ~~O-024~~ ✅ 已完成 — 并发 NS 解析

**完成于**：2026-03-16

- 最多 5 个并发 worker 解析 NS IP，首个成功立即返回
- 后台更新缓存，减少后续查询延迟
- NS 递归解析深度限制，防止栈溢出

---

## ~~O-025/026/027~~ ✅ 已完成 — NS 预热优化

**完成于**：2026-03-12

- 启动时查询 30 个高流量 TLD 的 NS 记录（Round 1 之后）
- CPU 感知动态并发控制
- 精选 TLD 列表，优化内存使用

---

## ✅ 已完成 — NS 缓存快照持久化

**完成于**：2026-03-17

- 关闭时保存 NS 委派缓存条目到 JSON 文件
- 启动时恢复未过期条目，消除冷启动延迟
- 配置项：`dns.cache_snapshot_path`

---

## ✅ 已完成 — 状态机重构

**完成于**：2026-03-13

- 状态文件拆分为单文件（每个状态独立源文件）
- 语义命名（`state_hosts.go`、`state_forward.go`、`state_check_resp.go` 等）
- `baseState` 嵌入，消除状态结构体重复字段
- 辅助函数提取，提升可读性

---

## v0.2.0 — 全量缓存快照

**目标**：快照从仅保存 NS 委派扩展到保存全部缓存条目，重启后所有域名首次查询即缓存命中。

> **背景**：当前快照仅保存 NS 委派条目，恢复后每个域名的最终 A/AAAA 答案仍需 1-2 次上游往返。
> 对于单机生产部署（应用 + Docker + 爬虫），重启后爬虫批量请求数千域名时，
> 全部缓存 miss 会导致前几分钟吞吐量下降。全量快照可让爬虫重启后第一秒即全速运行。
>
> 原「学习型预热 Round 2」方案（衰减 LFU + `publicsuffix` + ~500 行代码）已废弃，
> NS 缓存快照持久化已覆盖其 80-90% 的价值，剩余增量由本改动以 ~20 行代码完成。

### 设计

改动集中在 `server/snapshot.go` 的保存过滤逻辑：

- **保存时**：移除"必须含 NS RR"的过滤条件，改为保存所有缓存条目（A/AAAA 答案、NS 委派、CNAME、负缓存等）
- **恢复时**：逻辑不变——遍历条目、计算剩余 TTL、丢弃过期条目、`setCacheCopy()` 写入缓存
- **无新配置项**：复用现有 `dns.cache_snapshot_path`

### 规模评估（单机生产场景）

| 负载类型 | 缓存条目 | 快照文件 | 恢复耗时 |
|----------|---------|---------|---------|
| 纯应用（50-200 域名） | 1,000-4,000 | 1-4 MB | < 50 ms |
| 应用 + 垂类爬虫（200-500 站点） | 4,000-10,000 | 4-10 MB | < 100 ms |
| 应用 + 多垂类爬虫（500-2,000 站点） | 10,000-40,000 | 10-40 MB | < 500 ms |

### 任务清单

- [ ] `server/snapshot.go` — 移除 NS-only 过滤，保存全部缓存条目
- [ ] 单元测试 `server/snapshot_test.go` — 验证 A/AAAA/CNAME/负缓存条目的保存与恢复
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

## v1.0.0 — DNSSEC + DoT/DoH

**目标**：达到生产可用的安全性和传输加密标准。

### 任务清单

- [ ] DNSSEC 验证（完整信任链，从根到叶）
- [ ] DNS over TLS (DoT) 支持（RFC 7858）
- [ ] DNS over HTTPS (DoH) 支持（RFC 8484）
- [ ] 并发向多个 nameserver 查询（减少单点超时影响）
- [ ] 查询频率限制（防滥用）

---

## 开放 Backlog

以下为已识别但未排期的工作项，详见 `.rec53/BACKLOG.md`：

### 功能增强（高优先级）

| ID    | 标题                                    | 说明                                           |
|-------|-----------------------------------------|------------------------------------------------|
| O-016 | AAAA (IPv6) 上游选择支持                | 当前仅使用 A 记录选择上游 IP                   |
| O-006 | TCP 重试截断响应（RFC 1035）            | UDP 响应被截断时自动 TCP 重试                  |

### 安全与健壮性

| ID    | 标题                                    | 说明                                           |
|-------|-----------------------------------------|------------------------------------------------|
| B-014 | Glue 无 bailiwick 校验                  | 防止缓存投毒风险                               |
| O-022 | Response ID 未校验                      | 应验证 DNS 响应 ID 匹配请求                    |
| O-018 | 状态机死循环保护增强                    | 当前仅 MaxIterations=50，可增加更细粒度检测    |

### 缓存优化

| ID    | 标题                                    | 说明                                           |
|-------|-----------------------------------------|------------------------------------------------|
| O-021 | 无 glue 时委派 NS 不缓存               | 避免缓存不完整的委派信息                       |
| O-005 | 负缓存 TTL 可配置化                    | `DefaultNegativeCacheTTL=60` 应从配置读取      |

### 测试覆盖增强

| ID    | 标题                                    | 优先级 |
|-------|-----------------------------------------|--------|
| T-003 | 负缓存 E2E 测试                        | High   |
| T-004 | 查询预算耗尽测试                        | High   |
| T-005 | 超时重试与服务器切换测试                | High   |
| T-006 | SERVFAIL 与服务器黑名单测试             | High   |
| T-007 | Response ID 不匹配测试                  | Medium |
| T-008 | CNAME + NXDOMAIN 组合测试               | Medium |
| T-009 | Referral 循环检测测试                   | Medium |
| T-010 | 全部 NS 不可达测试                      | Medium |

---

## Future

### 长期目标

- DNS over QUIC (DoQ) 支持
- Response Policy Zones (RPZ)
- 高可用集群（多节点协调缓存）
- 查询日志与分析（可接入 ELK / ClickHouse）
- Web 管理面板
