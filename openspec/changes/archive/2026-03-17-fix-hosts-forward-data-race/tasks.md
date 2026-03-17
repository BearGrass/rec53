## 1. Core: Atomic Snapshot Infrastructure

- [x] 1.1 在 `server/state_shared.go` 中定义 `hostsForwardSnapshot` 结构体（字段：`hostsMap`、`hostsNames`、`forwardZones`）
- [x] 1.2 将三个裸全局变量替换为 `var globalHostsForward atomic.Pointer[hostsForwardSnapshot]`，并在 `init()` 中存储空快照
- [x] 1.3 将 `setGlobalHostsAndForward` 改为构造 `hostsForwardSnapshot` 并调用 `atomic.Store`
- [x] 1.4 将 `ResetHostsAndForwardForTest` 改为存储空快照（`&hostsForwardSnapshot{}`）
- [x] 1.5 新增包内私有辅助函数 `setSnapshotForTest(snap *hostsForwardSnapshot)`，供内部单元测试使用

## 2. Read Path: State Handlers

- [x] 2.1 在 `server/state_hosts.go` 的 `handle()` 开头添加 `snap := globalHostsForward.Load()`
- [x] 2.2 将 `state_hosts.go` 中三处读取 `globalHostsMap` / `globalHostsNames` 的引用改为 `snap.hostsMap` / `snap.hostsNames`
- [x] 2.3 在 `server/state_forward.go` 的 `handle()` 开头添加 `snap := globalHostsForward.Load()`
- [x] 2.4 将 `state_forward.go` 中三处读取 `globalForwardZones` 的引用改为 `snap.forwardZones`

## 3. Tests: Internal Unit Tests

- [x] 3.1 在 `server/state_hosts_test.go` 中，将 6 处直接赋值 `globalHostsMap` / `globalHostsNames` 改为调用 `setSnapshotForTest`，并将 `defer` 恢复逻辑改为原子恢复
- [x] 3.2 在 `server/state_forward_test.go` 中，将 6 处直接赋值 `globalForwardZones` 改为调用 `setSnapshotForTest`，并将 `defer` 恢复逻辑改为原子恢复

## 4. Tests: E2E Cleanup

- [x] 4.1 在 `e2e/helpers.go` 的 `setupResolverWithMockRoot` 的 `t.Cleanup` 函数中，补加 `server.ResetHostsAndForwardForTest()` 调用

## 5. Verification

- [x] 5.1 运行 `go build ./...` 确认编译无误
- [x] 5.2 运行 `go test -race ./server/...` 确认 server 包单元测试全绿且无 race
- [x] 5.3 运行 `go test -race -timeout 120s ./e2e/...` 确认 e2e 测试全绿且无 race
- [x] 5.4 运行 `go test -race ./...` 确认整体测试套件通过
