## 1. Documentation Information Architecture

- [x] 1.1 重写 `README.md`，将其收敛为项目定位、快速开始、最小配置和文档索引入口
- [x] 1.2 同步重写 `README.zh.md`，确保与英文 README 在结构和推荐路径上保持一致
- [x] 1.3 新建 `docs/user/quick-start.md`，覆盖首次部署、前台运行和基础验证
- [x] 1.4 新建 `docs/user/configuration.md`，整理默认配置、关键配置项和推荐值
- [x] 1.5 新建 `docs/user/operations.md` 与 `docs/user/troubleshooting.md`，覆盖日志、metrics、pprof、常见故障和排障路径

## 2. Developer Documentation

- [x] 2.1 收敛 `docs/architecture.md`，仅保留开发者关心的架构、状态机、缓存和并发边界
- [x] 2.2 新建 `docs/dev/README.md` 或等效索引文档，作为开发者文档入口
- [x] 2.3 新建 `docs/dev/contributing.md`，整理本地开发、构建、提交前检查和文档同步规则
- [x] 2.4 新建 `docs/dev/testing.md`，整理 unit、e2e、race、benchmark 的执行方式和注意事项
- [x] 2.5 新建 `docs/dev/release.md`，给出 v1.0.0 发布检查清单

## 3. Stability And Readability Cleanup

- [x] 3.1.1 为默认启动路径补测试，覆盖 listener bind 失败时 `Run()` 不阻塞且错误可见
- [x] 3.1.2 修复 `server/server.go` 中 ready wait 的启动失败路径，避免 `Run()` 永久阻塞
- [x] 3.1.3 回归验证正常启动时 `UDPAddr()` / `TCPAddr()` 语义不变
- [x] 3.2.1 为关闭路径补测试，覆盖 warmup 与后台 goroutine 不阻塞 `Shutdown()`
- [x] 3.2.2 修复 `server/server.go` 中 warmup、XDP 和其他后台 goroutine 的取消/等待顺序
- [x] 3.2.3 回归验证关闭后 err channel 关闭与资源清理语义不变
- [x] 3.3 收敛 `cmd/rec53.go` 中配置校验与错误提示，确保默认部署路径的失败信息清晰可诊断
- [x] 3.4 对启动、关闭和关键热路径中的重复逻辑做小范围可读性整理，避免引入结构性重构
- [x] 3.5 为上述修改补充或修正 `server/`、`cmd/` 中的针对性测试

## 4. Release Baseline Validation

- [x] 4.1 用默认流程验证 `./generate-config.sh`、构建、前台运行和基本 `dig` 查询
- [x] 4.2 运行 `go test -short ./...` 与受影响包的针对性测试，确认文档与代码收敛未引入回归
- [x] 4.3 更新 `CHANGELOG.md` 中与 v1.0.0 发布准备相关的说明
- [x] 4.4 复核 README、用户文档、开发者文档和架构文档之间的链接与职责边界
