## 1. server/snapshot.go — 核心模块

- [x] 1.1 新建 `server/snapshot.go`，定义 `SnapshotConfig` 结构体（`Enabled bool`、`File string`），无 `MaxEntries`
- [x] 1.2 定义 `snapshotEntry` 内部结构体（`Key string`、`MsgB64 string`、`SavedAt int64`）和 `snapshotFile` 包装结构体（`Entries []snapshotEntry`）
- [x] 1.3 实现 `SaveSnapshot(cfg SnapshotConfig) error`：遍历 `globalDnsCache.Items()`，筛选 Ns section 含 `*dns.NS` 记录的条目，将 `dns.Msg` pack 为 wire format 再 base64 编码，记录 `saved_at`（unix 秒），序列化为 JSON，`os.MkdirAll` 确保目录存在，`os.WriteFile` 覆写文件；`cfg.Enabled` 为 false 或 `cfg.File` 为空时直接返回 nil
- [x] 1.4 实现 `LoadSnapshot(cfg SnapshotConfig) (int, error)`：读取 JSON 文件，遍历条目，用 `saved_at` + RR TTL 计算剩余 TTL，跳过已过期条目，调用 `setCacheCopy` 写入 `globalDnsCache`，返回导入条目数和 error；文件不存在（`os.IsNotExist`）时返回 0, nil；JSON 解析失败返回 0, err；`cfg.Enabled` 为 false 或 `cfg.File` 为空时返回 0, nil

## 2. server/server.go — Shutdown 触发快照写入

- [x] 2.1 在 `server` 结构体中添加 `snapshotCfg SnapshotConfig` 字段
- [x] 2.2 更新 `NewServerWithFullConfig` 接受 `SnapshotConfig` 参数，赋值给 `s.snapshotCfg`
- [x] 2.3 在 `Shutdown()` 末尾（UDP/TCP 停止、IP Pool shutdown 之后），若 `s.snapshotCfg.Enabled` 为 true，调用 `SaveSnapshot(s.snapshotCfg)`；失败时 `monitor.Rec53Log.Errorf` 记录，不影响 Shutdown 返回

## 3. cmd/rec53.go — 配置集成与启动时恢复

- [x] 3.1 在 `Config` 结构体中新增 `Snapshot server.SnapshotConfig` 字段（yaml tag: `snapshot`）
- [x] 3.2 将 `cfg.Snapshot` 传入 `server.NewServerWithFullConfig`（更新函数签名和所有调用点）
- [x] 3.3 在 `main()` 中 `rec53.Run()` 之前同步调用 `server.LoadSnapshot(cfg.Snapshot)`；成功时 `Infof` 记录导入条目数，失败时 `Warnf` 记录并继续（不 fatal）

## 4. generate-config.sh — 配置示例

- [x] 4.1 在 `generate-config.sh` 中新增注释掉的 `snapshot:` 示例块，含 `enabled`、`file` 字段及说明注释

## 5. 单元测试

- [x] 5.1 新建 `server/snapshot_test.go`，测试 `SaveSnapshot` 只保存含 NS 记录的条目，不保存 A/AAAA/SOA 条目
- [x] 5.2 测试 `LoadSnapshot` 跳过过期条目，正确写入未过期条目
- [x] 5.3 测试 `SaveSnapshot` → `LoadSnapshot` 往返：写入文件后读取，条目数和内容一致
- [x] 5.4 测试 `LoadSnapshot` 在文件不存在时返回 0, nil（不 panic，不报错）
- [x] 5.5 测试 `LoadSnapshot` 在 JSON 损坏时返回 0, err（调用方可降级）
- [x] 5.6 测试 `SnapshotConfig.Enabled = false` 时 `SaveSnapshot` 和 `LoadSnapshot` 立即返回 nil/0，不创建文件
- [x] 5.7 运行 `go test -race ./server/...` 验证无数据竞争

## 6. 文档更新

- [x] 6.1 在 `README.md` 的 Configuration 部分新增 `snapshot:` 配置块说明（字段表格 + 示例）
- [x] 6.2 更新 `docs/architecture.md`，描述快照恢复的启动时序（`LoadSnapshot` 在 `Run()` 前同步执行）
