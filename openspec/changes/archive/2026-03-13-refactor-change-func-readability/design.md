## Context

`server/state_machine.go` 中的 `Change()` 函数是 rec53 解析器的核心调度循环，负责驱动整个状态机从 `STATE_INIT` 到 `RETURN_RESP`。当前实现 253 行，逻辑正确，但可读性较差：

- 6 个 case 各自重复相同的 `handle` 调用与 error wrap 样板（约 80 行）
- `CLASSIFY_RESP` case 内联了 ~40 行的 CNAME 追踪逻辑，主干状态转移被淹没
- `for {}` + 手动 `iterations++` + 顶部 overflow 检查的写法比 for-range 风格冗长

重构目标：纯可读性提升，零行为改变。

## Goals / Non-Goals

**Goals:**
- 提取 `handleState()`，消除 6 处重复样板
- 提取 `followCNAME()`，将 CNAME 追踪逻辑从 `CLASSIFY_RESP` case 中分离
- 提取 `buildFinalResponse()`，封装 `RETURN_RESP` 的收尾逻辑
- 将迭代计数改为惯用的 `for iterations := 1; iterations <= MaxIterations; iterations++`
- `Change()` 主体压缩至 ~95 行，整体文件 ~160 行
- 全部现有测试无修改通过（`go test -race ./...`）

**Non-Goals:**
- 不改变任何外部接口（`Change` 签名、`stateMachine` interface）
- 不修改任何业务逻辑或状态转移规则
- 不修改测试文件
- 不引入新依赖

## Decisions

### D1：`handleState` 不内置日志，日志留在调用方

**选择**：`handleState` 只封装 `stm.handle()` 调用与 error wrap，不写日志。调用方在拿到 error 后自行 `Errorf`。

**理由**：日志语境（state 名称、业务含义）在调用方更清晰；`handleState` 保持 thin，职责单一。若把日志放入 `handleState`，则每次调用的日志内容完全相同，反而丢失了 case 级别的语境信息。

**备选**：日志统一到 `handleState` 内 → 否决，会丢失 case 语境，且不同 case 的错误消息无法区分。

---

### D2：`followCNAME` 通过指针参数修改 `cnameChain` 和 `stm` 内部状态

**选择**：`followCNAME(stm stateMachine, cnameChain *[]dns.RR, visited map[string]bool) error`，直接操作 `stm.getResponse()` 和 `stm.getRequest()` 的字段（NS、Extra、Answer、Question），与原内联代码行为完全一致。

**理由**：原代码就是直接修改 `stm.getResponse().Ns` 等字段的，提取函数时保持相同的修改路径，避免引入任何行为差异。返回值仅为 `error`（CNAME 循环检测失败时）。

**备选**：让 `followCNAME` 返回新的 req/resp → 否决，会引入不必要的拷贝和接口变化。

---

### D3：`buildFinalResponse` 为纯函数

**选择**：`buildFinalResponse(stm stateMachine, origQ dns.Question, chain []dns.RR) *dns.Msg`，在函数内完成 Question 恢复和 cnameChain prepend，返回最终 `*dns.Msg`。

**理由**：RETURN_RESP case 的收尾逻辑自包含（无副作用于外部状态），提取为纯函数语义清晰，也便于单独测试。

---

### D4：循环结构改为标准 for 计数器

**选择**：`for iterations := 1; iterations <= MaxIterations; iterations++`，在循环体开头加 `defer`-free 的 debug log。

**理由**：将 overflow 检查从循环体顶部移入循环条件，使主干逻辑不被打断；Go 惯用写法，阅读者无需在循环体内搜索退出条件。

## Risks / Trade-offs

**[风险] 提取函数后行为细节遗漏** → 缓解：三个辅助函数的函数体均逐行对照原内联代码编写，并通过 `go test -race ./...`（含 e2e）全量验证，不允许任何测试失败。

**[风险] `followCNAME` 直接修改 `stm` 内部字段，副作用不直观** → 缓解：函数命名明确（`follow` 暗示修改），注释说明哪些字段被修改；调用方在调用后立即切换 stm，行为与原代码完全对应。

**[权衡] 辅助函数增加了间接层** → 主循环可读性收益远大于一层间接的代价；函数均短小（<30行），阅读者可快速内联理解。
