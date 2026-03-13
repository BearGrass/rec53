## Why

解析含多跳跨域 CNAME 的域名（如 www.huawei.com）时，前两次查询必然超时，第三次才成功。根本原因是：每次 CNAME 目标跨入新域（akadns.net → cdnhwc1.com → chinamobile.com），状态机未能正确找到新域的 NS glue，退化为从根服务器重新迭代全套委托链，累计 UDP 往返超过客户端默认超时（5s）。

## What Changes

- **修复 `inGlueState.handle`**：CNAME 跨域后进入 `IN_GLUE` 时，若 `response.Ns` 中保留的是旧域的 NS（与新查询域无关），应视为"无 glue"，返回 `IN_GLUE_NOT_EXIST` 而不是 `IN_GLUE_EXIST`，强制走 `IN_GLUE_CACHE` 路径查找正确的委托。
- **修复 `isNSRelevantForCNAME` 语义**：当前逻辑在 CNAME 第一跳（huawei.com → akadns.net）时保留了 akadns.net 的 NS，导致后续跨域查询时 `IN_GLUE` 误判为有 glue。需确保只有当 Ns 中的区域是新目标域的委托链成员时才保留。
- **新增 `IN_GLUE` 的域相关性检查**：`inGlueState.handle` 在判断 `IN_GLUE_EXIST` 之前，验证 `response.Ns[0].Header().Name` 是否是当前查询域的祖先域（ancestor）；若不是，则清空并返回 `IN_GLUE_NOT_EXIST`。
- **新增集成测试**：覆盖多跳跨域 CNAME 解析场景，使用 mock DNS server，验证首次（冷缓存）也能在合理时间内完成。

## Capabilities

### New Capabilities

- `cname-cross-domain-glue-validation`: `IN_GLUE` 状态对 Ns 记录的域相关性校验——仅当 Ns 区域是当前查询域的祖先时才视为有效 glue。

### Modified Capabilities

- （无需修改现有 spec，本次修改属于实现层 bug 修复，不引入新的对外行为契约）

## Impact

- `server/state_define.go`：`inGlueState.handle` 增加域相关性检查
- `server/state_machine.go`：`isNSRelevantForCNAME` 调用逻辑调整（或在 `IN_GLUE` 中集中处理，可在 design 中决策）
- `server/state_define_test.go`：新增多跳跨域 CNAME 集成测试用例
- 不引入新依赖，不修改配置结构，不影响缓存键格式
