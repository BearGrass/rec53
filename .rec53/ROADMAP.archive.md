# Roadmap Archive

这个文件保存已经完成的版本和历史里程碑摘要，避免主 [ROADMAP.md](./ROADMAP.md) 持续膨胀。
这里保留的是“发生过什么”和“留下了什么”，不再承担当前排期职责。

## Completed Versions

| Version | Date | Highlights |
|---------|------|------------|
| v0.2.0 | 2026-03 | 全量缓存快照，启动恢复未过期条目，降低冷启动延迟 |
| v0.3.0 | 2026-03 | IP Pool stale prune，加入 root IP 豁免和周期清理 |
| v0.4.0 | 2026-03 | alloc baseline、pprof、`updatePercentiles` 零分配优化 |
| v0.6.0 | 2026-03 | XDP Cache 核心：eBPF 程序、Go loader、cache sync |
| v0.6.1 | 2026-03 | XDP 指标、TTL 清理、性能验收与文档 |
| v1.0.0 | 2026-03 | 1.0 发布收敛：tag、发布说明、文档与运行基线 |

## Historical Milestones

这些项多数形成于早期探索阶段，命名和编号不完全等同于正式版本号；后续只作为历史记录保留。

| Milestone | Date | Highlights |
|-----------|------|------------|
| Hosts 本地权威 + Forwarding | 2026-03 | 新增本地权威应答和最长后缀转发规则 |
| rec53ctl 运维脚本 | 2026-03 | `build/install/upgrade/uninstall/run` 单入口运维 |
| IPQualityV2 + Happy Eyeballs | 2026-03-13 | 滑动窗口百分位 RTT、4 状态模型、并发上游竞速 |
| 负缓存 + Bad Rcode 重试 | 2026-03-12 | NXDOMAIN/NODATA 缓存与 SERVFAIL/REFUSED 切换备用服务器 |
| 并发 NS 解析 | 2026-03-16 | 最多 5 worker 并发解析 NS IP，首个成功即返回 |
| NS 预热优化 | 2026-03-12 | 启动时预热高流量 TLD 的 NS 记录 |
| 状态机重构 | 2026-03-13 | 单文件拆分、语义命名、`baseState` 抽取 |

## Archived Notes

### 全量缓存快照

- 关闭时保存全量 DNS 缓存条目，启动时恢复未过期条目
- `remainingTTL` 覆盖 `Answer`、`Ns`、`Extra`
- 配置项：`snapshot.enabled`、`snapshot.file`
- 主要价值是缩短重启后的冷启动窗口

### IP Pool Stale Prune

- 为 `IPQualityV2` 增加 `lastSeen`
- 周期清理长期未使用 IP，限制池无限增长
- 根服务器 IP 永久豁免

### 可观测性与分配基线

- 所有核心 benchmark 补 `b.ReportAllocs()`
- 引入受控 pprof 端点，默认仅监听 loopback
- `updatePercentiles()` 从临时 slice 改为固定数组，降为零分配
- Cache COW 仅完成审计与设计，不直接落地高风险方案

### XDP Cache

- eBPF 在 cache hit 时直接返回 DNS 响应，miss 透传到 Go
- Go 侧负责对象加载、接口挂载、cache sync、指标导出和 TTL 清理
- 已补充 `docs/architecture.md`、`docs/testing/benchmarks.md`、`README*.md`

### v1.0.0 发布收敛

- 仓库已存在 `v1.0.0` tag
- `CHANGELOG.md`、`README.md`、`README.zh.md`、`docs/dev/release.md` 已按 `1.0` 发布口径整理
- 发布基线明确为单机 `node-local recursive resolver`
- XDP 保持可选增强，默认 Go 路径作为发布基线

## Reference Docs

- [benchmarks.md](../docs/testing/benchmarks.md)
- [architecture.md](../docs/architecture.md)
- [metrics.md](../docs/metrics.md)
- [xdp-physical-benchmark-2026-03-19.zh.md](../docs/testing/xdp-physical-benchmark-2026-03-19.zh.md)
