## Why

`Change()` 在 `server/state_machine.go` 中承担了状态机主循环的全部职责，当前 253 行，其中约 80 行是每个 case 重复的相同样板（`handle` 调用 + 错误包装），CNAME 追踪逻辑 40 行内联于 `CLASSIFY_RESP` case 中，导致主干状态转移逻辑难以一眼看清。本次重构在不改变任何行为的前提下，通过提取辅助函数显著提升可读性。

## What Changes

- 提取 `handleState(stm stateMachine) (int, error)` — 统一封装 `stm.handle()` 的调用与 error wrap，消除 6 处重复样板
- 提取 `followCNAME(stm stateMachine, cnameChain *[]dns.RR, visited map[string]bool) error` — 将 CNAME 循环检测、链追加、NS 清理、Question 更新从 `CLASSIFY_RESP` case 中分离
- 提取 `buildFinalResponse(stm stateMachine, origQ dns.Question, chain []dns.RR) *dns.Msg` — 封装 RETURN_RESP 的收尾逻辑（恢复 Question + prepend cnameChain）
- 将 `for {}` + 手动 `iterations` 计数改写为 `for iterations := 1; iterations <= MaxIterations; iterations++`，把溢出检查移入循环条件
- `Change()` 主函数从 ~210 行压缩至 ~95 行，整体文件从 253 行减至约 160 行
- **零逻辑改动**：所有现有测试（`go test -race ./...`）必须无修改通过

## Capabilities

### New Capabilities

无。本次为纯内部重构，不引入新能力。

### Modified Capabilities

无。不涉及任何对外行为或接口变化，现有 spec 均不需要更新。

## Impact

- **修改文件**：`server/state_machine.go`（唯一改动文件）
- **测试**：`server/state_machine_test.go`、`server/state_define_test.go`、`e2e/` 无需改动，全部应原样通过
- **接口**：`Change(stm stateMachine) (*dns.Msg, error)` 签名不变，`stateMachine` interface 不变
- **依赖**：无新依赖
