## Why

`globalHostsMap`、`globalHostsNames`、`globalForwardZones` 三个包级全局变量通过三次独立赋值设置，而 DNS 请求处理 goroutine 在同一时刻可能正在读取这些变量，构成 data race，`go test -race` 可检测到。此外，e2e 测试中 `setupResolverWithMockRoot` 缺少对这三个变量的 cleanup，导致测试间状态污染。

## What Changes

- 引入不可变快照结构 `hostsForwardSnapshot`，将三个变量捆绑为一个原子单元
- 用 `atomic.Pointer[hostsForwardSnapshot]` 替换三个裸全局变量，读写均通过原子操作
- `setGlobalHostsAndForward` 改为单次 `atomic.Store`（消除写侧 race）
- `state_hosts.go` / `state_forward.go` 的 `handle()` 读路径改为先 `Load()` 快照再访问字段（消除读侧 race）
- `server/state_hosts_test.go` / `server/state_forward_test.go` 中直接赋值裸变量的 12 处改为调用包内辅助函数（保持 `-race` 安全）
- `e2e/helpers.go` 的 `setupResolverWithMockRoot` cleanup 补加 `server.ResetHostsAndForwardForTest()`（修复测试间状态污染）

## Capabilities

### New Capabilities

- `atomic-hosts-forward-snapshot`: 通过 `atomic.Pointer` 对 hosts/forward 配置做原子快照读写，确保并发安全

### Modified Capabilities

（无需求层面变更，仅实现层面修复）

## Impact

- **修改文件**：`server/state_shared.go`、`server/state_hosts.go`、`server/state_forward.go`、`server/state_hosts_test.go`、`server/state_forward_test.go`、`e2e/helpers.go`
- **依赖**：新增标准库 `sync/atomic`（Go 1.19+ 泛型原子指针，项目已用 Go 1.21+，无需升级）
- **API 兼容性**：`SetHostsAndForwardForTest` / `ResetHostsAndForwardForTest` 签名不变，e2e 调用方无需修改
- **性能**：`atomic.Pointer.Load()` 在 x86-64 上为单条 MOV 指令，无可测量的性能影响
- **无破坏性变更**
