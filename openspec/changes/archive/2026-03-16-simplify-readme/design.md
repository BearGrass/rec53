## Context

README.md 目前是单一文件，622 行，混合了面向不同读者的内容：
- **用户**关心：怎么装、怎么跑、怎么配置
- **运维**关心：Prometheus 指标、PromQL、Docker 部署
- **开发者**关心：状态机设计、缓存机制、IP Pool 算法

三类内容叠加在一起，导致任何一类读者都需要跳过大量无关内容。此外，`.rec53/` 目录目前混放了两类性质不同的文件：面向外部读者的技术参考（ARCHITECTURE.md）和面向内部贡献者的开发约定（CONVENTIONS.md、ROADMAP.md），职责边界模糊。

## Goals / Non-Goals

**Goals:**
- README.md 精简至 ≤ 120 行，专注快速上手
- 新建 `docs/` 目录，统一承载所有对外技术文档
- `.rec53/ARCHITECTURE.md` 迁移至 `docs/architecture.md`，并合并 README 中的系统设计内容
- 性能基准数据独立到 `docs/benchmarks.md`
- Prometheus 指标与 PromQL 独立到 `docs/metrics.md`
- `.rec53/` 目录只保留开发内部约定文件（CONVENTIONS.md、ROADMAP.md 等）
- 所有迁移内容不丢失，README 底部文档索引更新指向
- 更新 AGENTS.md 中引用 `.rec53/ARCHITECTURE.md` 的路径

**Non-Goals:**
- 不修改 `README.zh.md`（中文版，范围外）
- 不重写任何技术内容，只迁移和重组
- 不改变项目功能或代码
- 不移动 `.rec53/` 下的其他文件（CONVENTIONS.md、ROADMAP.md、BACKLOG.md 等）

## Decisions

### 决策 1：不删除内容，只迁移

**选项 A（本方案）**：内容全部保留，按读者类型分到不同文件，README 做索引。

**选项 B**：直接删除冗余内容，README 保持唯一信息源。

**选择 A，理由**：技术文档（状态机转换图、IP Pool 算法、性能数据）对开发者和贡献者有价值，删除会丢失信息。迁移而非删除是更安全的做法。

---

### 决策 2：`docs/` 统一承载所有对外技术文档，包括 ARCHITECTURE

**选项 A（本方案）**：新建 `docs/` 目录，将 ARCHITECTURE.md 从 `.rec53/` 迁入 `docs/architecture.md`，同时新建 `docs/benchmarks.md` 和 `docs/metrics.md`。`.rec53/` 只保留开发约定类文件。

**选项 B**：ARCHITECTURE.md 留在 `.rec53/`，仅新建 `docs/benchmarks.md` 和 `docs/metrics.md`。

**选择 A，理由**：ARCHITECTURE.md 是面向所有开发者（包括外部贡献者）的技术参考，不是内部约定。将其与 `docs/` 下的其他技术文档归为一类，结构语义更清晰。`.rec53/` 的职责收窄为：代码规范（CONVENTIONS）、路线图（ROADMAP）、待办事项（BACKLOG/TODO）等内部运营文件，GitHub 仓库根目录显得更整洁。

---

### 决策 3：README 中的系统设计内容合并至 `docs/architecture.md`

README 中的 "System Design"、"Core Subsystem: State Machine / Cache / IP Pool" 三节（约 300 行）与 `.rec53/ARCHITECTURE.md` 现有内容合并，写入 `docs/architecture.md`，消除重复。

---

### 决策 4：README 保留 CLI Flags 和基础配置示例

这两块内容是用户每次查阅最频繁的，不迁移出去，保留在 README 中。

---

### 决策 5：更新 AGENTS.md 中的路径引用

AGENTS.md 中有 4 处引用 `.rec53/ARCHITECTURE.md`，需同步更新为 `docs/architecture.md`，确保 AI agent 拿到正确路径。

## Risks / Trade-offs

- **[风险] 链接失效** → 迁移完成后统一检查 README、AGENTS.md 中所有文档链接的路径是否正确
- **[风险] `.rec53/ARCHITECTURE.md` 旧路径被外部引用** → 可在原路径留一行重定向说明，指向新路径（可选）
- **[取舍] `docs/` 目录从无到有** → 增加目录层级，但换来语义清晰；`.rec53/` 职责收窄，更聚焦

## Migration Plan

1. 新建 `docs/` 目录
2. 将 `.rec53/ARCHITECTURE.md` 内容迁移至 `docs/architecture.md`，并合并 README 中的系统设计章节
3. 创建 `docs/benchmarks.md`（从 README 中剪切性能相关章节）
4. 创建 `docs/metrics.md`（从 README 中剪切监控章节）
5. 删除 `.rec53/ARCHITECTURE.md`（内容已完整迁入 `docs/architecture.md`）
6. 重写 README.md（≤ 120 行），底部文档索引更新
7. 更新 AGENTS.md 中 `.rec53/ARCHITECTURE.md` 的所有引用路径
8. 验证：所有文档索引链接可达，内容无丢失

无回滚风险，纯文档变更，不影响代码逻辑。

## Open Questions

（无）
