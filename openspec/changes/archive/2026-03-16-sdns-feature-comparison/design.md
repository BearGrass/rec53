## Context

rec53 当前已具备完整的递归 DNS 解析能力（状态机迭代、缓存、IP 质量跟踪、启动预热、Prometheus 监控），但缺少系统性的横向对比文档，外部用户和内部开发者难以快速判断 rec53 与其他主流 DNS 解析器（如 sdns）的差异和定位。

sdns（github.com/semihalev/sdns）是 Go 生态中功能最为全面的开源递归 DNS 解析器（~1k Stars），提供了 DoT/DoH/DoQ 加密传输、DNSSEC 验证、Kubernetes 中间件、反射攻击防御、域名封锁等 rec53 目前尚未实现的功能。

## Goals / Non-Goals

**Goals:**
- 产出一份结构清晰的功能对比文档 `docs/sdns-comparison.md`
- 按功能维度逐项列出 rec53 vs sdns 的差异
- 标注每个差距的建议优先级（High / Medium / Low / Out-of-scope）
- 将文档纳入 OpenSpec capability spec 管理，便于持续更新

**Non-Goals:**
- 不实现任何新功能（本 change 是文档性质，不修改代码）
- 不与其他 DNS 软件（CoreDNS、BIND、Unbound）做对比（单独 change 处理）
- 不包含性能基准数字（已有 `docs/benchmarks.md`）

## Decisions

**决策 1：文档放在 `docs/` 而非 `.rec53/`**
- 理由：对比文档是面向外部贡献者和潜在用户的参考，应与 `docs/architecture.md`、`docs/benchmarks.md` 等对外文档放在同一目录
- 替代方案：放在 `.rec53/ROADMAP.md` 的附录 → 不利于独立检索和引用

**决策 2：对比维度以 sdns 的功能分类为锚**
- 理由：sdns 的功能分类较完整（10 个类别），用它做行索引，rec53 的状态作为列，差距一目了然
- 替代方案：以 RFC 标准为维度 → 过于学术，不直观

**决策 3：每个差距标注优先级而非仅列差异**
- 理由：差距本身不代表方向，需要结合 rec53 定位（轻量递归解析器）给出取舍建议
- 优先级判断标准：
  - **High**：影响核心用户场景（生产部署、安全合规）
  - **Medium**：增强竞争力，技术可行
  - **Low**：Nice-to-have，资源充足时可做
  - **Out-of-scope**：与 rec53 定位不符，明确不做

## Risks / Trade-offs

- [文档过时风险] sdns 迭代较快，对比数据可能在 3-6 个月后失效 → 在文档头部注明对比日期和 sdns 版本号，并提示定期更新
- [优先级主观性] 功能优先级判断带有主观色彩 → 在文档中说明判断依据，可在后续 change 中按优先级逐项实现
