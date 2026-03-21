# 路线图

[English](ROADMAP.md) | 中文

## 版本历史

| Version | Date | Highlights |
|---------|------|------------|
| v0.2.0 | 2026-03 | 全量缓存快照——重启后所有域名首次查询即缓存命中 |
| v0.4.1 | planned | SO_REUSEPORT 多 Listener——突破单 socket 吞吐上限 |
| v0.6.0 | 2026-03 | XDP Cache 核心——eBPF 程序、Go loader、cache sync、XDP_TX 响应 |
| v0.6.1 | planned | XDP 生产就绪——Prometheus 指标、TTL 清理、性能验收、文档 |
| dev | 2026-03 | NS 缓存快照持久化，消除重启冷启动延迟 |
| dev | 2026-03 | 并发 NS 解析（O-024），最多 5 worker，首个成功即返回 |
| dev | 2026-03 | Happy Eyeballs 并发上游查询，先到先得 |
| dev | 2026-03 | IPQualityV2 — 滑动窗口环形缓冲 + P50/P95/P99 + 4 状态健康模型 |
| dev | 2026-03 | 负缓存（NXDOMAIN/NODATA）+ Bad Rcode 重试（SERVFAIL/REFUSED 换服务器） |
| dev | 2026-03 | NS 预热 — 30 个高流量 TLD，CPU 感知动态并发 |
| dev | 2026-03 | 状态机重构 — 单文件拆分、baseState 嵌入、语义命名 |
| dev | 2026-03 | Hosts 本地权威、Forwarding 转发规则、rec53ctl 运维脚本、/dev/stderr 日志 |
| dev | 2026-03 | Graceful shutdown、综合测试套件、E2E 测试框架 |
| dev | 2026-03 | IP 质量追踪 + 预取、Prometheus 指标、Docker Compose 部署 |

## 当前版本：dev

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
- **缓存快照持久化** — 关闭时保存全量 DNS 缓存到 JSON 文件，启动时恢复未过期条目

### 本地策略

- **Hosts 本地权威** — A/AAAA/CNAME 静态记录，AA=true 权威应答，优先于缓存和上游
- **Forwarding 转发规则** — 最长后缀匹配转发到指定上游 DNS，结果不写缓存

### 运维与可观测性

- **rec53ctl** — 单入口运维脚本：`build` / `install` / `upgrade` / `uninstall` / `run`
- **Prometheus 指标端点** — 查询计数、延迟、缓存命中率、IP 池状态
- **Docker Compose 部署** — 一键容器化部署
- **并发 NS 解析** — 最多 5 个 worker 并发解析 NS IP，首个成功即返回

## 已完成事项（节选）

保留现有历史记录，只做中文索引同步，具体细节以英文原文为准。
