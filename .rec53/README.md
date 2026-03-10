# rec53 项目文档

## 快速导航

| 文档 | 描述 |
|------|------|
| [架构设计](ARCHITECTURE.md) | 系统架构、状态机、数据流 |
| [路线图](ROADMAP.md) | 版本规划、需求清单 |
| [测试计划](TEST_PLAN.md) | 测试覆盖率提升计划 |
| [TODO](TODO.md) | 日常任务管理 |
| [编码规范](CONVENTIONS.md) | Go 代码风格、命名、错误处理 |
| [ADR](decisions/README.md) | 架构决策记录 |

## 当前状态

- **版本**: v0.1.0
- **测试覆盖率**: ~60%
- **已知问题**: 0

## 覆盖率详情

| 包 | 覆盖率 | 状态 |
|----|--------|------|
| server | 76.8% | ✅ 良好 |
| utils | 82.6% | ✅ 良好 |
| monitor | 58.1% | ✅ 已完善 |
| cmd | 20.0% | ⚠️ 待提升 |
| e2e | 28.6% | ⚠️ 需网络 |

## 目录结构

```
.rec53/
├── README.md           # 本文件
├── ARCHITECTURE.md     # 架构设计
├── ROADMAP.md          # 路线图 + 需求
├── TEST_PLAN.md        # 测试计划
├── TODO.md             # 任务管理
├── CONVENTIONS.md      # 编码规范
└── decisions/          # 架构决策记录
    └── README.md
```

## 相关文档

- [CHANGELOG.md](../CHANGELOG.md) - 版本变更记录
- [CLAUDE.md](../CLAUDE.md) - Claude Code 开发指南
- [README.md](../README.md) - 项目说明