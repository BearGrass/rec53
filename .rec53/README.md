# rec53 项目文档

## 快速导航

| 文档 | 描述 |
|------|------|
| [编码规范](CONVENTIONS.md) | Go代码风格、命名、错误处理 |
| [版本路线图](roadmap/ROADMAP.md) | v0.1~v0.4 功能规划 |
| [需求列表](roadmap/REQUIREMENTS.md) | 功能/非功能需求清单 |
| [开发进展](progress/PROGRESS.md) | 每日开发日志 |
| [代码质量](quality/CODE_QUALITY.md) | 问题分析 + 优化计划 |
| [Bug追踪](bugs/README.md) | Bug记录索引 |

## 当前状态

- **版本**: v0.1.0
- **Phase 1 (并发安全)**: ✅ 完成
- **Phase 2 (架构重构)**: 待开始
- **测试覆盖率**: ~60%
- **已知问题**: 0

## 目录结构

```
.rec53/
├── README.md              # 本文件
├── CONVENTIONS.md         # 编码规范
├── roadmap/               # 规划文档
│   ├── ROADMAP.md        # 版本路线图
│   └── REQUIREMENTS.md   # 需求列表
├── progress/              # 进度追踪
│   └── PROGRESS.md       # 开发日志
├── quality/               # 代码质量
│   └── CODE_QUALITY.md   # 分析与优化计划
├── bugs/                  # Bug记录
│   └── README.md         # Bug索引
└── decisions/             # 架构决策记录(ADR)
    └── README.md         # ADR索引
```

## 相关文档

- [CLAUDE.md](../CLAUDE.md) - Claude Code 开发指南
- [README.md](../README.md) - 项目说明