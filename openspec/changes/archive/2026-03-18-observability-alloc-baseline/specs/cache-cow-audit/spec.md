## ADDED Requirements

### Requirement: Cache COW 调用方可变性审计清单
openspec specs 或 docs 中 SHALL 存在一份 `getCacheCopy` / `getCacheCopyByType` 全部生产调用方的可变性审计清单，明确标注每个调用方对返回的 `*dns.Msg` 是只读还是可变使用。

#### Scenario: 审计清单覆盖全部生产调用方
- **WHEN** 查阅审计清单
- **THEN** 清单 SHALL 列出 `getCacheCopy` 和 `getCacheCopyByType` 在 `server/` 包中的全部生产调用方（不含测试代码）
- **AND** 每个调用方 SHALL 标注为 READ_ONLY 或 MUTATING，附带代码位置和判断依据

---

### Requirement: Cache COW 不可变包装设计草案
审计文档 SHALL 包含 Cache COW（Copy-on-Write → Read-Only）设计草案，描述消除防御性 `Copy()` 的技术方案。

#### Scenario: 设计草案包含不可变包装方案
- **WHEN** 查阅设计草案
- **THEN** 草案 SHALL 包含至少一种不可变包装类型方案（如 `ReadOnlyMsg` 包装类型或接口限制）
- **AND** SHALL 包含风险评估（移除 `Copy()` 后未来调用方意外修改缓存的风险）
- **AND** SHALL 包含对性能收益的预期分析

---

### Requirement: Cache COW 实施硬门槛
文档 SHALL 明确定义 Cache COW 代码实施的前置条件，防止在无数据支撑时盲目优化。

#### Scenario: 实施门槛明确定义
- **WHEN** 查阅实施门槛定义
- **THEN** 文档 SHALL 声明：仅当 pprof heap profile 证明 `dns.Msg.Copy()` 调用占总堆分配的 >30% 时，才进入代码实施阶段
- **AND** SHALL 描述如何使用 pprof 验证此门槛（具体命令或步骤）
