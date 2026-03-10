# TODO

## 当前任务

暂无进行中的任务。

---

## 待办

| 优先级 | 标签 | 任务 | 备注 |
|--------|------|------|------|
| P0 | TEST | 补充 monitor/metric.go 单元测试 | TEST_PLAN.md 第1批，预计+5% |
| P1 | TEST | 补充 cmd/rec53.go 信号处理测试 | TEST_PLAN.md 第2批 |
| P1 | TEST | 补充 state_machine.go Change 完整路径测试 | CNAME循环、MaxIterations |
| P1 | TEST | 补充 iterState 成功查询路径测试 | 需要 mock DNS 服务器 |
| P1 | REQ | 依赖注入重构（消除全局变量） | ROADMAP.md 技术债务 |
| P1 | REQ | 状态机类型安全（StateID 替代 int） | ROADMAP.md 技术债务 |
| P2 | TEST | 补充 utils/net.go Hc 函数测试 | TEST_PLAN.md 第4批 |
| P2 | TEST | 修复 E2E 测试（Mock 完整解析链） | TEST_PLAN.md 第5批 |
| P2 | OPT | DNS Client 连接池 | ROADMAP.md 技术债务 |
| P2 | OPT | 性能基准测试 | ROADMAP.md 技术债务 |

---

## 已完成

| 日期 | 任务 |
|------|------|
| 2026-03-10 | 文档重构：新增 ARCHITECTURE.md, TEST_PLAN.md, TODO.md, CHANGELOG.md |
| 2026-03-10 | Question Section Mismatch 修复 |
| 2026-03-09 | E2E 测试修复（缓存类型、CNAME 循环等） |
| 2026-03-04 | Phase 1: 并发安全修复 |
| 2026-03-04 | v0.1.0 发布 |

---

## 覆盖率进度

```
当前:    ~30%
目标:    >60%

server:   75.2% ✅
utils:    82.6% ✅
cmd:      20.0% ⚠️
monitor:   3.2% ⚠️
```

---

## 标签说明

| 标签 | 含义 |
|------|------|
| REQ | 功能需求 |
| BUG | 缺陷修复 |
| OPT | 性能优化 |
| TEST | 测试相关 |

## 优先级说明

| 优先级 | 含义 |
|--------|------|
| P0 | 当前版本必须完成 |
| P1 | 下个版本完成 |
| P2 | 想到了先记着 |