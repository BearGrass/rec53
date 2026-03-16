## Why

当前 README.md 共 622 行，将项目概览、快速上手、详细系统设计（状态机、缓存、IP Pool）、监控、Docker 部署、开发指南等内容全部塞入一个文件，信息密度过高，读者难以快速找到所需内容。对于只想跑起来的用户而言，需要滚动浏览大量内部实现细节；对于开发者而言，深度技术内容又散落在叙述性文本中，不易检索。此外 `.rec53/` 目录同时放着对外技术参考（ARCHITECTURE.md）和内部开发约定（CONVENTIONS.md、ROADMAP.md），职责边界模糊。

## What Changes

- 将 README.md 精简为面向**用户**的入口文档（目标 ≤ 120 行），只保留：项目一句话描述、核心特性列表、快速上手（Build / Run / Test resolution）、CLI Flags 表格、基础配置示例、Docker 一键运行、文档索引
- 新建 `docs/` 目录，统一承载所有对外技术文档
- 将 `.rec53/ARCHITECTURE.md` 迁移至 `docs/architecture.md`，并合并 README 中的系统设计内容（状态机、缓存、IP Pool）
- 将**性能基准数据**（延迟表格、QPS、内存容量表、自定义 benchmark 命令）迁移至新文件 `docs/benchmarks.md`
- 将**Prometheus 指标定义与 PromQL 示例**迁移至新文件 `docs/metrics.md`
- `.rec53/` 目录职责收窄为内部开发约定（CONVENTIONS、ROADMAP、BACKLOG 等）
- 更新 README.md 底部文档索引，指向以上迁移内容
- 更新 AGENTS.md 中对 `.rec53/ARCHITECTURE.md` 的引用路径

## Capabilities

### New Capabilities

- `readme-slim`: 精简后的 README.md，面向用户快速上手，≤ 120 行
- `architecture-doc`: 将 `.rec53/ARCHITECTURE.md` 迁移并合并 README 系统设计内容，形成 `docs/architecture.md`
- `benchmarks-doc`: 新建 `docs/benchmarks.md`，承载所有性能基准数据与自定义 benchmark 指南
- `metrics-doc`: 新建 `docs/metrics.md`，承载 Prometheus 指标定义与 PromQL 示例

### Modified Capabilities

（无现有 spec 级行为变更）

## Impact

- `README.md` — 直接修改，大幅削减行数
- `.rec53/ARCHITECTURE.md` — 删除，内容迁入 `docs/architecture.md`
- `docs/architecture.md` — 新建，合并原 ARCHITECTURE.md + README 系统设计章节
- `docs/benchmarks.md` — 新建
- `docs/metrics.md` — 新建
- `AGENTS.md` — 更新 4 处 `.rec53/ARCHITECTURE.md` 引用路径为 `docs/architecture.md`
- `README.zh.md` — 若存在中文版，需同步精简（范围外，不在本 change 内）
