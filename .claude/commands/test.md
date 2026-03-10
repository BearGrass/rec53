# 单元测试工作流

你现在进入单元测试工作模式，严格按照以下流程执行，不要跳步。

## Phase 1：规划

1. 阅读 `CLAUDE.md` 和 `.rec53/TEST_PLAN.md`
2. 扫描项目所有 `.go` 文件（排除 `_test.go`、`vendor/`）
3. 对比已有测试，找出缺失或覆盖不足的文件
4. 运行 `go test -coverprofile=coverage.out ./...` 获取当前覆盖率基线
5. 生成或更新 `.rec53/TEST_PLAN.md`：
   - 按依赖顺序分批：protocol > cache > resolver > upstream > handler > middleware > zone > config > utils
   - 每个条目包含：源文件路径、测试文件路径、核心测试点、难度、状态
   - 更新进度总览中的覆盖率基线
6. 将计划展示给我，**等待我确认后再进入下一阶段**

## Phase 2：分批执行

从 TEST_PLAN.md 中找到第一个状态为「待开始」的批次，然后：

对该批次中的每一个文件，逐个执行以下循环：

### 单文件循环
1. 编写测试文件
2. 运行 go test -race 确认通过
3. 更新 .rec53/TEST_PLAN.md 该条目状态
4. **自检清单（每个文件完成后必须逐条过一遍）：**
   - [ ] TEST_PLAN.md 是否已更新本条目状态？
   - [ ] TODO.md 是否有相关条目需要更新？（完成的标完成，发现新问题的加新条目）
   - [ ] 是否新增了包或目录？如是 → 更新 CLAUDE.md 目录结构
   - [ ] 是否改了接口或 mock 方式？如是 → 更新 CLAUDE.md 测试规范
   - [ ] 是否新增了用户可感知功能？如是 → 更新 README.md
5. 在回复中列出自检结果，然后等待我确认

### 批次完成后
1. 运行 `go test -coverprofile=coverage.out ./...`
2. 汇报本批次覆盖率变化
3. 更新 TEST_PLAN.md 进度总览
4. 检查本次改动是否触发 CLAUDE.md、TODO.md ，README.md 的文档自维护规则，如触发则一并更新
5. 告诉我本批次全部完成，**等待我指示是否继续下一批**

## Phase 3：收尾（当所有批次完成后）

1. `go test -race ./...` 全量通过
2. `go test -coverprofile=coverage.out ./...` 最终覆盖率
3. 找出覆盖率低于目标的包，提出补充建议
4. 对 protocol 包运行 `go test -fuzz=Fuzz -fuzztime=30s`
5. 更新 TEST_PLAN.md 最终数据
6. 更新 CLAUDE.md 和 README.md（如有变化）
7. 输出总结报告

## 强制规则
- **每个文件完成后必须等待我确认**，不要连续执行多个文件
- **永远不要一次性生成所有批次的测试**
- 如果上下文接近 60%，提醒我开新会话，并确保 TEST_PLAN.md 已保存最新状态
- 如果发现源代码有 bug，记录到 `.rec53/TODO.md` 但不要自行修改源码，先告诉我