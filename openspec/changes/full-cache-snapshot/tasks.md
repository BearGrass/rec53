## 1. SaveSnapshot 全量保存

- [x] 1.1 移除 `SaveSnapshot` 中的 `hasNS` 过滤循环（第 53-63 行），改为对所有 `*dns.Msg` 条目执行保存
- [x] 1.2 更新 `SaveSnapshot` 日志，从 `"saved %d NS entries"` 改为 `"saved %d cache entries"`
- [x] 1.3 更新 `SaveSnapshot` 和 `SnapshotConfig` 的 godoc 注释，反映全量缓存语义

## 2. remainingTTL 扩展

- [x] 2.1 在 `remainingTTL` 函数中增加对 `msg.Answer` section 的 TTL 遍历（与现有 `msg.Ns` / `msg.Extra` 逻辑一致）
- [x] 2.2 更新 `remainingTTL` 的 godoc 注释，说明覆盖 Answer + Ns + Extra 三个 section

## 3. 测试

- [x] 3.1 新增测试：A/AAAA 答案记录的保存与恢复（构造纯 Answer 的 `dns.Msg`，验证 save → load 往返）
- [x] 3.2 新增测试：CNAME 记录的保存与恢复
- [x] 3.3 新增测试：纯 Answer 记录的 `remainingTTL` 计算（`msg.Ns` 和 `msg.Extra` 为空）
- [x] 3.4 新增测试：混合 section 的 `remainingTTL` 取最小值
- [x] 3.5 验证现有 NS 委派测试仍然通过（回归）
- [x] 3.6 验证旧版 NS-only 快照文件可被新版本正常加载（向后兼容）

## 4. 文档

- [x] 4.1 更新 `docs/architecture.md` 中快照相关描述

## 5. 补充测试

- [x] 5.1 单元测试：`TestSnapshotEmptyFileNoOp` — `File=""` 时即使 `Enabled=true` 也是 no-op
- [x] 5.2 单元测试：`TestLoadSnapshotCorruptMsgB64` — 非法 base64 和非法 wire format 条目被静默跳过
- [x] 5.3 单元测试：`TestSnapshotFileDirAutoCreated` — 目标目录不存在时 `MkdirAll` 自动创建
- [x] 5.4 单元测试：`TestRemainingTTLEmptyMsg` — 三个 section 均为空时返回 0
- [x] 5.5 e2e 测试：`TestSnapshotE2E_ARecordSurvivesRestart` — A 记录经完整迭代解析 → 快照保存 → 缓存清空 → 恢复 → 缓存命中（不触发上游）
- [x] 5.6 e2e 测试：`TestSnapshotE2E_NSEntrySurvivesRestart` — NS 委派条目经快照保存/恢复后跳过根和 TLD 查询
- [x] 5.7 e2e 测试：`TestSnapshotE2E_ExpiredEntrySkipped` — 全部过期的快照条目被跳过，查询仍触发上游解析
- [x] 5.8 e2e 测试：`TestSnapshotE2E_DisabledNoOp` — `Enabled=false` 时不创建文件且 Load 返回 `(0, nil)`
