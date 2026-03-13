## Context

`server/ip_pool.go`（665 行）包含三个逻辑层次：V1 `IPQuality`（简单原子延迟，已废弃）、V2 `IPQualityV2`（滑动窗口百分位延迟 + 状态机）、`IPPool`（选路、探针、prefetch 管理）。

V1 路径的废弃情况已通过代码分析确认：
- `getBestIPs`/`UpIPsQuality`/`updateIPQuality`/`isTheIPInit`：生产路径零调用
- `GetPrefetchIPs`/`PrefetchIPs`：虽在 `getBestAddressAndPrefetchIPs` 中被调用，但因 `IPPool.pool`（V1 map）从未被 V2 写入路径填充，`GetPrefetchIPs` 始终返回空列表，是 no-op
- `prefetchIPQuality`：仅由 `PrefetchIPs` 内部调用，随之成为死代码
- V1 Prometheus 指标 `rec53_ip_quality`：仅由 `prefetchIPQuality` 写入，同样成为死指标

## Goals / Non-Goals

**Goals:**
- 将 `ip_pool.go` 按职责拆分为 `ip_pool_quality_v2.go` 和 `ip_pool.go`
- 删除全部 V1 代码（类型、方法、字段、测试）
- 删除 monitor 包中的 V1 Prometheus 指标
- 确保拆分后测试全部通过（`go test -race ./...`）

**Non-Goals:**
- 不实现 V2 版本的 prefetch 功能（可作为后续独立变更）
- 不修改 V2 选路逻辑或算法
- 不将 ip_pool 迁移至独立包
- 不改变任何对外可见的 DNS 行为

## Decisions

### 决策一：拆分为 2 个文件而非 3 个

**选择**：`ip_pool_quality_v2.go` + `ip_pool.go`（不单独提取 V1 文件）

**理由**：V1 代码将被整体删除，无需为其创建临时文件。常量（`INIT_IP_LATENCY`、`MAX_IP_LATENCY` 等）与 `IPQualityV2` 强相关，放入 `ip_pool_quality_v2.go`；`IPPool` 特有的常量（`MAX_PREFETCH_CONCUR`、`PREFETCH_TIMEOUT`）随 V1 prefetch 一并删除。

**备选方案**：3 文件（`ip_pool_quality.go` + `ip_pool_quality_v2.go` + `ip_pool.go`）—— 因 V1 已删除，第三个文件无意义。

### 决策二：同步删除 monitor V1 指标

**选择**：删除 `IPQuality` GaugeVec 和 `IPQualityGaugeSet`

**理由**：V1 指标 `rec53_ip_quality` 随 prefetch 链删除后不再有写入方，保留会造成空注册。V2 已有完整替代指标（`rec53_ipv2_p50/p95/p99_latency_ms`）。

**风险**：若有外部监控系统依赖 `rec53_ip_quality` 告警规则，需同步更新。

### 决策三：直接删除 V1 prefetch 调用，不替换

**选择**：从 `getBestAddressAndPrefetchIPs` 删除 2 行调用，不实现 V2 prefetch

**理由**：当前 prefetch 已是 no-op，删除不影响功能。V2 prefetch 涉及基于评分的 IP 候选筛选，是独立的功能设计，不应混入本次重构变更。

## Risks / Trade-offs

- **测试覆盖率下降** → 删除 V1 测试函数后需验证 V2 测试仍覆盖核心路径（`IPQualityV2` 已有完整单元测试 `e2e/ippool_v2_test.go` 和 `server/ip_pool_test.go` 中的 V2 部分）
- **`rec53_ip_quality` 指标消失** → 若部署了依赖此指标的 Prometheus 告警，需在上线前更新告警规则；V2 指标 `rec53_ipv2_p50_latency_ms` 提供等价信息
- **state_query_upstream_test.go 中部分辅助测试使用 V1 API** → 删除时需确认这些测试覆盖的路径已由 V2 测试覆盖，不能遗漏

## Migration Plan

1. 创建 `server/ip_pool_quality_v2.go`（将 `IPQualityV2` 相关代码移入）
2. 清理 `server/ip_pool.go`（删除 V1 代码，保留 `IPPool` + `globalIPPool` + `ResetIPPoolForTest`）
3. 更新 `server/state_query_upstream.go`（删除 V1 prefetch 调用）
4. 清理 `monitor/` 包（删除 V1 指标）
5. 清理测试文件（删除 V1 测试函数）
6. 运行 `go test -race ./...` 验证全部通过
7. 运行 `go build ./cmd` 验证编译成功

**回滚**：本次为纯删除重构，git revert 即可完整回滚，无数据迁移风险。

## Open Questions

无。V1 废弃情况已通过代码分析完整确认，无需进一步调研。
