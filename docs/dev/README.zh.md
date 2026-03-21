# 开发者文档

[English](README.md) | 中文

本章节面向贡献者和维护者，重点说明 rec53 的组织方式、如何安全修改，以及如何准备发布。

## 核心文档

- [架构说明](../architecture.zh.md)
- [贡献指南](contributing.zh.md)
- [测试说明](testing.zh.md)
- [发布清单](release.zh.md)

## 参考资料

- [编码约定](CONVENTIONS.zh.md)
- [路线图](ROADMAP.zh.md)
- [指标说明](../metrics.zh.md)
- [基准测试](../benchmarks.zh.md)

## 工作方式

默认路径是基线：

- 在启用 XDP 优化前先保证 Go 路径正确
- 优先做有针对性的生命周期和可读性修复，而不是大规模重构
- 保持面向用户的文档和面向开发者的文档分开
- 把 Prometheus 指标和 label 当作面向运维的契约，而不是单纯的内部调试输出
