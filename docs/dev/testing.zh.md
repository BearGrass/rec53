# 测试说明

[English](testing.md) | 中文

rec53 包含单元、包级、基准和端到端测试。迭代时先跑针对性测试，合并或发布前再做更广的验证。

## 常用命令

完整 race suite：

```bash
go test -race ./...
go test -race -timeout 120s ./... -count=1
```

短模式：

```bash
go test -short ./...
```

包级测试：

```bash
go test -v ./server/...
go test -v ./e2e/...
go test -v -run TestServerRunAndShutdown ./server/...
```

覆盖率：

```bash
go test -cover ./...
```

## 期望

- 并发敏感工作要用 `-race`
- 适合的话优先使用 table-driven tests
- 触碰启动、关闭、畸形输入和缓存行为时，要补充针对性测试
- 不要在 e2e 测试里做不必要的全局重置，因为冷缓存会让测试变慢且噪声更大

## E2E 说明

- `e2e/main_test.go` 负责 `TestMain`
- 不要在 `e2e/` 下每个文件都写 `init()` 初始化
- mock authority server 相关 helper 用 `e2e/helpers.go`

## 生命周期变更要测什么

修改 `cmd/` 或 `server/server.go` 时，覆盖：

- 启动成功
- 启动失败
- listener ready 行为
- 优雅关闭
- warmup 取消
- 可选特性的降级路径

## 可观测性检查

修改 metrics 或 label 时，确认：

- metric 名称和 label 数量保持有界且有明确意图
- `docs/metrics.md` 保持准确
- 运维侧查询或面板不会悄悄失效
- feature-gated metric（如 XDP 指标）要明确写出条件

## 性能工作

做 benchmark 或性能结论时：

- 优先使用现有 benchmark 文档和 `tools/` 下工具
- 不要把没测过的改进写成新结果
- 只有真的跑过相关验证时才更新性能文档
