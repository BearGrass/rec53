## ADDED Requirements

### Requirement: Inline cache sync on write
当 Go 层 `setCacheCopy()` 或 `setCacheCopyByType()` 写入 DNS 缓存时，若 XDP 已启用，SHALL 同步将该条目写入 BPF cache_map。写入 SHALL 在 `setCacheCopy` 返回前完成（同步，非异步）。

#### Scenario: Cache write triggers BPF map update
- **WHEN** `setCacheCopyByType("example.com.", dns.TypeA, msg, 300)` 被调用且 XDP 已启用
- **THEN** BPF cache_map SHALL 包含 key=`{wireformat("example.com."), TypeA}` 的条目
- **AND** value SHALL 包含预序列化的 DNS 响应和正确的 expire_ts

#### Scenario: XDP disabled skips BPF sync
- **WHEN** `setCacheCopy` 被调用但 XDP 未启用
- **THEN** 不 SHALL 有任何 BPF map 操作
- **AND** Go 缓存写入行为 SHALL 与未引入 XDP 时完全一致

#### Scenario: BPF map update failure is non-fatal
- **WHEN** `bpf_map.Update()` 返回错误（如 `-ENOSPC`）
- **THEN** Go 缓存写入 SHALL 仍然成功
- **AND** 错误 SHALL 以 Debug 级别记录日志（不影响正常服务）

### Requirement: Presentation to wire format domain conversion
Cache sync SHALL 将 Go 层的 presentation format 域名（如 `"example.com."`）转换为 DNS wire format（长度前缀标签序列），并 inline 转小写。转换结果 MUST 与 eBPF 程序从网络包中提取的 qname 格式完全一致。

#### Scenario: Standard domain conversion
- **WHEN** 域名 `"example.com."` 被转换
- **THEN** wire format SHALL 为 `\x07example\x03com\x00`

#### Scenario: Case normalization
- **WHEN** 域名 `"Example.COM."` 被转换
- **THEN** wire format SHALL 为 `\x07example\x03com\x00`（全小写）

#### Scenario: Root domain
- **WHEN** 域名 `"."` 被转换
- **THEN** wire format SHALL 为 `\x00`

### Requirement: Response pre-serialization
Cache sync SHALL 调用 `dns.Msg.Pack()` 将 DNS 响应序列化为 wire format 字节序列。序列化后的响应大小超过 512 字节时 SHALL 跳过 BPF map 写入。

#### Scenario: Normal response serialization
- **WHEN** DNS 响应 Pack() 后为 120 字节
- **THEN** cache sync SHALL 将 120 字节写入 BPF map value 的 response 区域
- **AND** `resp_len` 字段 SHALL 为 120

#### Scenario: Oversized response skipped
- **WHEN** DNS 响应 Pack() 后为 600 字节（超过 512）
- **THEN** cache sync SHALL 跳过 BPF map 写入
- **AND** Go 缓存写入 SHALL 仍然正常完成

#### Scenario: Pack failure skipped
- **WHEN** `dns.Msg.Pack()` 返回错误
- **THEN** cache sync SHALL 跳过 BPF map 写入并记录 Debug 日志

### Requirement: Monotonic clock TTL calculation
Cache sync SHALL 使用 `unix.ClockGettime(unix.CLOCK_MONOTONIC)` 获取当前 monotonic 时间（秒），计算 `expire_ts = monotonic_now + ttl_seconds`。MUST NOT 使用 `time.Now().Unix()`（wall clock）。

#### Scenario: TTL calculation correctness
- **WHEN** monotonic 当前时间为 1000 秒，TTL 为 300 秒
- **THEN** BPF map value 的 `expire_ts` SHALL 为 1300

#### Scenario: Monotonic clock consistency with eBPF
- **WHEN** Go 写入 expire_ts=1300，eBPF 在 monotonic 时间 1200 秒时检查
- **THEN** eBPF SHALL 判定条目未过期（1200 < 1300）
- **AND** 在 monotonic 时间 1301 秒时 SHALL 判定条目已过期（1301 > 1300）

### Requirement: BPF map key/value struct alignment
Cache sync 使用的 Go 侧 key/value 结构体 MUST 与 eBPF C 侧 `dns_cache.h` 定义的结构体在内存布局（大小、字段偏移、padding）上完全一致。

#### Scenario: Key struct binary compatibility
- **WHEN** Go 构造 cache_key 并通过 `bpf_map.Update()` 写入
- **THEN** eBPF 程序的 `bpf_map_lookup_elem` SHALL 能正确匹配该 key

#### Scenario: Value struct binary compatibility
- **WHEN** Go 构造 cache_value 并写入 BPF map
- **THEN** eBPF 程序读取的 `expire_ts`、`resp_len`、`response` 字段值 SHALL 与 Go 写入值一致
