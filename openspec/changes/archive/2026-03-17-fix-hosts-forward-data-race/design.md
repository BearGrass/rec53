## Context

rec53 是一个迭代式 DNS 解析器，采用状态机模式处理每条 DNS 查询。hosts 文件条目和转发区域配置在服务器启动时编译为三个包级全局变量：

```go
var (
    globalHostsMap     map[string]*dns.Msg
    globalHostsNames   map[string]bool
    globalForwardZones []ForwardZone
)
```

每次配置更新（或测试 setup）时，`setGlobalHostsAndForward` 对三个变量执行三次独立赋值。与此同时，DNS 请求处理 goroutine 直接读取这三个变量，不加任何同步原语。

`go test -race` 在并发测试场景下可检测到读写竞争。此外，e2e 测试中 `setupResolverWithMockRoot` 在 cleanup 时未重置这三个变量，导致测试间状态残留。

项目已有 `globalDnsCache`（`sync.RWMutex`）和 `globalIPPool`（`sync.RWMutex`）的并发安全模式，本次修复与该模式对齐，但因 hosts/forward 配置为"写少读多、整体替换"场景，选用更轻量的 `atomic.Pointer` 方案。

## Goals / Non-Goals

**Goals:**
- 消除 `setGlobalHostsAndForward` 与请求处理 goroutine 之间的 data race
- 修复 `setupResolverWithMockRoot` 遗漏的 cleanup，防止 e2e 测试间状态污染
- 与项目现有"状态机读全局变量"架构保持一致，不引入实例注入或 context 传参

**Non-Goals:**
- 不重构 `globalDnsCache` / `globalIPPool` 的并发模型
- 不将 hosts/forward 配置通过构造函数或 context 注入到状态机
- 不修改 `NewServer` / `NewServerWithConfig` 的签名或行为（避免破坏 e2e 测试中"先 SetHostsAndForwardForTest → 再 NewServer"的调用顺序）
- 不修改 `SetHostsAndForwardForTest` / `ResetHostsAndForwardForTest` 的公共签名

## Decisions

### D-1: 使用 `atomic.Pointer[hostsForwardSnapshot]` 而非 `sync.RWMutex`

**选择**：`atomic.Pointer[hostsForwardSnapshot]`

**理由**：hosts/forward 配置是"整体替换"语义——每次更新都是用一个全新的不可变快照替换旧快照，不存在部分字段更新。`atomic.Pointer` 的 `Load/Store` 是无锁操作，在 x86-64 上退化为单条指令，读路径零争用。`sync.RWMutex` 在高并发读场景下虽然正确，但有不必要的加锁开销。

**备选方案**：
- `sync.RWMutex`：正确但略重，适合字段级并发更新场景；此处无此需求
- `sync/atomic` 三个独立原子变量：无法保证三字段的原子性（读者可能看到两个字段来自新快照、一个来自旧快照）
- 实例注入重构：需修改状态机 `handle()` 签名及 14 处调用链，改动过大，被否决

### D-2: 不可变快照结构 `hostsForwardSnapshot`

将三个字段捆绑为一个结构体，确保读者始终看到同一版本的配置：

```go
type hostsForwardSnapshot struct {
    hostsMap     map[string]*dns.Msg
    hostsNames   map[string]bool
    forwardZones []ForwardZone
}
```

快照一旦创建即不修改，读者通过 `Load()` 获得指针后可安全访问其字段，无需额外锁。

### D-3: 测试辅助函数替代直接赋值

`server/state_hosts_test.go` 和 `server/state_forward_test.go` 是 `package server` 内部测试，目前直接赋值裸全局变量（12 处）。引入 `atomic.Pointer` 后，若继续直接赋值已不存在的裸变量将编译失败。

**选择**：在同一包内新增私有辅助函数 `setSnapshotForTest(snap *hostsForwardSnapshot)` 供内部测试使用；e2e 测试继续使用现有导出函数 `SetHostsAndForwardForTest` 和 `ResetHostsAndForwardForTest`。

### D-4: `init()` 发布空快照

`globalHostsForward` 在 `init()` 中存储一个空快照（非 nil 指针），使 `Load()` 结果永远非 nil，读路径无需 nil 检查。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| Go 1.19 前不支持泛型 `atomic.Pointer` | 项目已要求 Go 1.21+（见 go.mod），无风险 |
| 快照指针泄漏导致旧配置 map 无法 GC | hosts/forward 配置极少更新（仅启动时），对象数量恒定，GC 压力可忽略 |
| 内部测试用 `setSnapshotForTest` 绕过校验逻辑 | 仅用于设置已编译好的快照，与 `SetHostsAndForwardForTest` 等价，不引入新风险 |
| `setupResolverWithMockRoot` 补 cleanup 后影响其他 e2e 测试 | 该函数现有 cleanup 已有 `FlushCacheForTest` 和 `ResetIPPoolForTest`，补加 reset 与现有模式完全对称，不影响其他测试 |

## Migration Plan

1. 修改 `server/state_shared.go`：引入快照结构 + atomic.Pointer，删除三个裸变量
2. 修改 `server/state_hosts.go`、`server/state_forward.go`：读路径改用 `Load()` 快照
3. 修改 `server/state_hosts_test.go`、`server/state_forward_test.go`：12 处直接赋值改为快照辅助函数
4. 修改 `e2e/helpers.go`：补加 cleanup 1 行
5. 运行 `go test -race ./...` 验证无 race，测试全绿
6. 无需数据迁移，无需 rollback 策略（纯内存状态，进程重启即重置）

## Open Questions

无。所有技术决策已在本文档中确定。
