## Context

rec53 是一个完整的迭代式 DNS 解析器，定位替换 Ubuntu 桌面的 systemd-resolved。当前冷启动行为：Round 1 预热 30 个 TLD 的 NS 记录，但二级域名（`github.com`、`googleapis.com` 等）的 NS 委托链在重启后仍为空，导致首包查询需要 3-4 次网络往返，实测 309ms。

**启动时序（现状）：**
```
cmd/main()
  ├─ NewServerWithFullConfig(...)
  └─ rec53.Run()
       ├─ goroutine: warmupNSOnStartup()   ← 后台，非阻塞
       ├─ goroutine: udp.ListenAndServe()  ← 同时启动
       ├─ goroutine: tcp.ListenAndServe()
       ├─ <-udpReady / <-tcpReady          ← 等 socket 绑定
       └─ return                           ← 流量已进来，warmup 还在跑
```

快照恢复必须在 `ListenAndServe` 之前完成才有意义。

**相关代码：**
- `server/cache.go`：`globalDnsCache`（patrickmn/go-cache），`Items()` 可遍历全部条目，`setCacheCopy` 可直接写入
- `server/server.go`：`Run()` 启动时序；`Shutdown()` 是写快照的触发点
- `cmd/rec53.go`：`main()` 在 `Run()` 前有同步初始化窗口

## Goals / Non-Goals

**Goals:**
- 停机时将 `globalDnsCache` 中的 NS 委托条目序列化为 JSON 文件
- 启动时在 `ListenAndServe` 之前同步恢复快照，保证首包命中缓存
- 完全可选，`enabled: false` 时对现有行为零影响
- 文件损坏/不存在时静默降级，不影响启动

**Non-Goals:**
- 不持久化 A/AAAA/CNAME 应答记录（NS 层已足够消除委托链迭代）
- 不做跨节点同步
- 不在运行时动态学习、衰减或排序热度

## Decisions

### 决策 1：快照恢复在 `cmd/main()` 的 `Run()` 之前同步调用

**选择**：在 `cmd/rec53.go` 的 `main()` 中，`rec53.Run()` 之前显式调用 `server.LoadSnapshot(cfg.Snapshot)`，直接写入 `globalDnsCache`。

**原因**：
- `Run()` 内部 goroutine 启动后即开始接受 DNS 查询；任何放在 `warmupNSOnStartup()` 里的恢复都无法保证首包前完成
- 快照恢复是纯文件 I/O（`os.ReadFile` + json.Unmarshal + 写缓存），正常路径 < 5ms，完全不需要 context deadline 或并发管理
- 不改变 `server.Run()` 签名，不破坏现有测试
- 语义清晰：main() 初始化阶段负责"准备缓存"，server.Run() 负责"接受请求"

**替代方案（放弃）**：放进 `warmupNSOnStartup()` —— 无法保证首包前完成，核心收益消失

**替代方案（放弃）**：拆分 `Run()` 为 `Prepare()` + `Serve()` —— 改动公共接口，影响所有测试，收益与选项 A 相同

### 决策 2：`snapshot.go` 直接遍历 `globalDnsCache.Items()`，不向 cache.go 添加任何接口

**选择**：`SaveSnapshot` 在 `server` 包内直接调用 `globalDnsCache.Items()`，筛选 NS 委托条目，序列化写文件。`LoadSnapshot` 直接调用 `setCacheCopy`。不在 `cache.go` 中添加 `ExportNSEntries` / `ImportNSEntries` 等中间层。

**原因**：
- 快照是一次性持久化需求，不是 cache 的通用能力；为此向 cache 核心层添加接口是过度抽象
- `snapshot.go` 与 `cache.go` 同属 `server` 包，可直接访问包级变量，无需导出新函数
- 减少接口层，降低维护面

**替代方案（放弃）**：`ExportNSEntries / ImportNSEntries` 接口 —— 侵入 cache 层，增加不必要的抽象

### 决策 3：去掉 `MaxEntries`，保存所有 NS 委托条目

**选择**：`SaveSnapshot` 保存缓存中所有 NS 委托条目，不做截断。

**原因**：
- NS 委托条目数量天然有限（常见部署下几十到几百条），文件大小 < 100KB
- `MaxEntries` 在 map 遍历下是非确定性截断，不保证保留"最热"条目，反而引入错误的安全感
- 去掉一个无效配置项比保留一个误导性配置项更好

**替代方案（放弃）**：`MaxEntries` 截断 —— map 遍历顺序不确定，无法保证热点优先，语义有误导性

### 决策 4：快照写入在 `Shutdown()` 末尾同步执行

**选择**：`server.Shutdown()` 在 UDP/TCP 停止、IP Pool 关闭后，调用 `SaveSnapshot(s.snapshotCfg)`；写失败只记 error log，不影响 Shutdown 返回值。

**原因**：
- Shutdown 已有 5s deadline（cmd 层），写 < 100KB 文件耗时可忽略
- 服务停止后写入，无并发读写竞争
- 无需额外 goroutine 或周期性 flush

### 决策 5：快照文件格式为 JSON，条目含 `saved_at` 时间戳

**选择**：每个条目 `{ "key": "github.com.:2", "msg_b64": "<wire base64>", "saved_at": <unix_sec> }`。`msg_b64` 使用 `dns.Msg.Pack()` wire format 再 base64 编码。恢复时用 `saved_at` + RR TTL 计算剩余有效期，过期条目跳过。

**原因**：
- wire format 是最准确的序列化方式，保留全部 RR 字段和 TTL
- `saved_at` 使 TTL 在重启后仍有意义，不会把已过期条目写入缓存
- JSON 可读、可调试、无额外依赖

### 决策 6：`SnapshotConfig` 定义在 `server` 包，字段仅 `Enabled` + `File`

**选择**：`SnapshotConfig { Enabled bool; File string }`，与 `WarmupConfig` 模式一致，由 `cmd/rec53.go` 的 `Config.Snapshot` 持有并传入。

**原因**：去掉 `MaxEntries`（见决策 3）后只剩两个有意义的字段，结构简洁。

## Risks / Trade-offs

- **[风险] 快照 TTL 全部过期** → 恢复时全部跳过，退化为纯 Round 1 行为，无负面影响
- **[风险] 文件权限 / 磁盘满** → SaveSnapshot 失败只记 error log，Shutdown 正常完成
- **[风险] NS 委托变更（域名迁移 NS）** → NS TTL 通常 172800s（2 天），短期重启内概率极低；TTL 检查自然淘汰
- **[Trade-off] 默认关闭** → 用户需显式配置路径才能启用；File 为空字符串等价于禁用

## Migration Plan

1. 新增 `server/snapshot.go`（`SnapshotConfig`、`SaveSnapshot`、`LoadSnapshot`）
2. `server/server.go`：结构体加 `snapshotCfg` 字段，`NewServerWithFullConfig` 接受参数，`Shutdown()` 末尾调用 `SaveSnapshot`
3. `cmd/rec53.go`：`Config` 加 `Snapshot` 字段，`main()` 在 `rec53.Run()` 之前调用 `server.LoadSnapshot(cfg.Snapshot)`
4. `generate-config.sh` 增加 `snapshot:` 注释示例块

## Open Questions

- 默认 `File` 路径建议留空（= 禁用），由用户根据场景配置：桌面用 `~/.rec53/ns-cache.json`，systemd service 用 `/var/lib/rec53/ns-cache.json`。
