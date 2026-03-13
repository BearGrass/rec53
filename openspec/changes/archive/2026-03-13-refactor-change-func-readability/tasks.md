## 1. 提取 handleState 辅助函数

- [x] 1.1 在 `server/state_machine.go` 的 `Change()` 函数之前，新增 `handleState(stm stateMachine) (int, error)` 函数：调用 `stm.handle(stm.getRequest(), stm.getResponse())`，失败时返回包含 state 编号和原始 error 的包装 error
- [x] 1.2 将 `Change()` 中所有 6 处 `var ret int; var err error; if ret, err = stm.handle(...); err != nil { log + return }` 样板替换为 `ret, err := handleState(stm)` + 调用方的单行日志+return

## 2. 提取 followCNAME 辅助函数

- [x] 2.1 新增 `followCNAME(stm stateMachine, cnameChain *[]dns.RR, visited map[string]bool) error` 函数，将 `CLASSIFY_RESP` case 中 `CLASSIFY_RESP_GET_CNAME` 分支的 40 行逻辑原样移入：CNAME record 查找、循环检测、chain append、`isNSRelevantForCNAME` 条件清理 NS/Extra、Answer 清空、Question 更新
- [x] 2.2 将 `CLASSIFY_RESP` case 的 `CLASSIFY_RESP_GET_CNAME` 分支替换为：`if err := followCNAME(stm, &cnameChain, visitedDomains); err != nil { return nil, err }`，后跟 stm 切换

## 3. 提取 buildFinalResponse 辅助函数

- [x] 3.1 新增 `buildFinalResponse(stm stateMachine, origQ dns.Question, chain []dns.RR) *dns.Msg` 函数，将 `RETURN_RESP` case 的收尾逻辑原样移入：`resp.Question[0] = origQ`，`if len(chain) > 0 { resp.Answer = append(chain, resp.Answer...) }`，返回 `resp`
- [x] 3.2 将 `RETURN_RESP` case 的收尾替换为：`return buildFinalResponse(stm, originalQuestion, cnameChain), nil`

## 4. 重构主循环结构

- [x] 4.1 将 `iterations := 0` + `for {` + 顶部 `iterations++; if iterations > MaxIterations { ... }` 改写为 `for iterations := 1; iterations <= MaxIterations; iterations++`，在循环体内保留 debug log（引用 `iterations`）
- [x] 4.2 在 for 循环结束后（循环正常退出时）添加 `return nil, fmt.Errorf("max iterations exceeded, possible CNAME loop")`，替换原来循环体内的相同逻辑

## 5. 验证

- [x] 5.1 运行 `go build ./...`，确认零编译错误
- [x] 5.2 运行 `go test -race -timeout 120s ./... -count=1`，确认所有包 `ok`，零失败，零 race
