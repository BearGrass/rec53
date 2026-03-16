## Why

rec53 是一个功能聚焦的递归 DNS 解析器，而 sdns（github.com/semihalev/sdns）是同类开源项目中功能最全的实现之一。通过系统性对比两者的功能差异，可以帮助我们明确 rec53 当前的功能边界、识别高价值的待补齐能力，并为后续 roadmap 排优先级提供依据。

## What Changes

本 change 不修改代码，而是产出一份结构化的功能差异分析文档，并将其作为 capability 纳入 spec 管理。具体产出：

- 新增 `docs/sdns-comparison.md`：rec53 与 sdns 的逐维度功能对比表，含差距说明和建议优先级
- 新增 capability spec `sdns-comparison`：记录对比文档的内容结构与维护约定

## Capabilities

### New Capabilities

- `sdns-comparison`：rec53 与 sdns 项目的功能对比文档，涵盖 DNS 解析、传输协议、缓存、安全、监控、Kubernetes 集成等维度

### Modified Capabilities

（无现有 spec 的需求变更）

## Impact

- 新增文件：`docs/sdns-comparison.md`
- 新增 spec：`openspec/specs/sdns-comparison/spec.md`
- 不影响任何现有代码、API 或运行时行为
