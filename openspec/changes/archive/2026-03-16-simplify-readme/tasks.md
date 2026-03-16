## 1. 准备工作

- [x] 1.1 新建 `docs/` 目录

## 2. 创建 docs/architecture.md

- [x] 2.1 将 `.rec53/ARCHITECTURE.md` 现有全部内容复制为 `docs/architecture.md` 的基础
- [x] 2.2 将 README.md 中的 "System Design > Directory Structure" 与 `docs/architecture.md` 已有目录结构合并（去重）
- [x] 2.3 将 README.md 中的 "Request Lifecycle" 和 "Component Map" 章节追加至 `docs/architecture.md`
- [x] 2.4 将 README.md 中的 "Core Subsystem: State Machine"（States 表格、转换图、Loop A/B 说明、CNAME 处理、NS 无 Glue 解析、Return Codes）追加至 `docs/architecture.md`
- [x] 2.5 将 README.md 中的 "Core Subsystem: Cache"（设计、负向缓存、Cache API、线程安全）追加至 `docs/architecture.md`
- [x] 2.6 将 README.md 中的 "Core Subsystem: IP Pool (IPQualityV2)"（数据结构、生命周期、评分公式、Score 示例表、选择 API、并发访问、Warmup Bootstrap）追加至 `docs/architecture.md`
- [x] 2.7 删除 `.rec53/ARCHITECTURE.md`

## 3. 创建 docs/benchmarks.md

- [x] 3.1 从 README.md 中提取 "Specifications" 章节（延迟表格含测试环境说明、缓存容量表、QPS 表、IP Pool 容量表）
- [x] 3.2 从 README.md 中提取 "Running Your Own Benchmark" 小节（benchmark 命令和环境变量说明）
- [x] 3.3 将提取内容写入 `docs/benchmarks.md`，保留测试环境注释（Intel i7-1165G7），验证命令路径正确

## 4. 创建 docs/metrics.md

- [x] 4.1 从 README.md 中提取 "Monitoring > Prometheus Metrics" 表格及指标端点地址
- [x] 4.2 从 README.md 中提取 "Useful Queries" PromQL 示例
- [x] 4.3 将提取内容写入 `docs/metrics.md`，补充说明端点地址可通过 `-metric` flag 或 `dns.metric` 配置项修改

## 5. 精简 README.md

- [x] 5.1 重写 README.md，保留：项目描述、Features 列表、Quick Start、CLI Flags 表格、Configuration 基础示例（含 Warmup TLD List 简介）、Docker 快速运行命令
- [x] 5.2 更新 README.md 底部文档索引：`docs/architecture.md`、`docs/benchmarks.md`、`docs/metrics.md`、`.rec53/CONVENTIONS.md`、`.rec53/ROADMAP.md`
- [x] 5.3 验证 README.md 总行数 ≤ 120 行

## 6. 更新 AGENTS.md

- [x] 6.1 将 AGENTS.md 中 4 处 `.rec53/ARCHITECTURE.md` 引用替换为 `docs/architecture.md`

## 7. 验证

- [x] 7.1 检查 README.md 所有文档索引链接指向的文件均存在，且不含 `.rec53/ARCHITECTURE.md` 的引用
- [x] 7.2 确认性能数据、Prometheus 指标、PromQL 示例无丢失（对比原 README）
- [x] 7.3 确认 `docs/architecture.md` 涵盖原 `.rec53/ARCHITECTURE.md` 和 README 系统设计章节的所有二级标题
- [x] 7.4 确认 `.rec53/ARCHITECTURE.md` 已被删除
