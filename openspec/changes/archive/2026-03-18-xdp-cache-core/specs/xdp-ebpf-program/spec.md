## ADDED Requirements

### Requirement: DNS packet parsing and identification
XDP 程序 SHALL 解析 Ethernet、IPv4、UDP header，识别目标端口为 53 的 DNS 查询包。非 UDP、非端口 53、非 IPv4 的包 SHALL 直接返回 `XDP_PASS` 透传到内核网络栈。

#### Scenario: Standard DNS query on port 53
- **WHEN** 一个 UDP 目标端口 53 的 IPv4 DNS 查询包到达 XDP hook
- **THEN** XDP 程序 SHALL 继续解析 DNS header 并尝试缓存查找

#### Scenario: Non-DNS traffic passthrough
- **WHEN** 一个 TCP 包或目标端口非 53 的 UDP 包到达 XDP hook
- **THEN** XDP 程序 SHALL 返回 `XDP_PASS`，不做任何处理

#### Scenario: Non-IPv4 traffic passthrough
- **WHEN** 一个 IPv6 或非 IP 协议包到达 XDP hook
- **THEN** XDP 程序 SHALL 返回 `XDP_PASS`

### Requirement: DNS query validation
XDP 程序 SHALL 验证 DNS header 的 `QDCOUNT == 1`（标准查询），非标准查询 SHALL 直接 `XDP_PASS`。`QR` 位 MUST 为 0（query），否则 `XDP_PASS`。

#### Scenario: Standard single-question query
- **WHEN** DNS 包的 QDCOUNT == 1 且 QR == 0
- **THEN** XDP 程序 SHALL 提取 qname 并进行缓存查找

#### Scenario: Multi-question query passthrough
- **WHEN** DNS 包的 QDCOUNT != 1
- **THEN** XDP 程序 SHALL 返回 `XDP_PASS`

#### Scenario: DNS response passthrough
- **WHEN** DNS 包的 QR == 1（response）
- **THEN** XDP 程序 SHALL 返回 `XDP_PASS`

### Requirement: Qname extraction with inline lowercase
XDP 程序 SHALL 使用 bounded loop 从 DNS question section 提取 qname（wire format 长度前缀标签序列），并在提取过程中将大写字母 inline 转换为小写。qname 总长度 MUST 不超过 `MAX_QNAME_LEN`（255 字节）。

#### Scenario: Simple domain qname extraction
- **WHEN** DNS question 包含 qname `\x07example\x03com\x00`（wire format）
- **THEN** XDP 程序 SHALL 提取完整 qname 并转为小写存入 lookup key

#### Scenario: Mixed case qname normalization
- **WHEN** DNS question 包含 qname `\x07Example\x03COM\x00`
- **THEN** XDP 程序 SHALL 将 qname 归一化为 `\x07example\x03com\x00` 后用于缓存查找

#### Scenario: Qname exceeds MAX_QNAME_LEN
- **WHEN** DNS question 的 qname 长度超过 255 字节
- **THEN** XDP 程序 SHALL 返回 `XDP_PASS`（透传到 Go）

### Requirement: BPF map cache lookup with TTL check
XDP 程序 SHALL 使用提取的 `{qname, qtype}` 作为 key 调用 `bpf_map_lookup_elem` 查找 cache_map。查找命中时 MUST 检查 `expire_ts`：若 `bpf_ktime_get_ns() / 1_000_000_000 > expire_ts`，视为过期，按 cache miss 处理。

#### Scenario: Cache hit with valid TTL
- **WHEN** BPF map 中存在匹配的 key 且 `expire_ts` 未过期
- **THEN** XDP 程序 SHALL 构建响应并返回 `XDP_TX`

#### Scenario: Cache miss
- **WHEN** BPF map 中不存在匹配的 key
- **THEN** XDP 程序 SHALL 返回 `XDP_PASS`，递增 miss 计数器

#### Scenario: Cache hit but expired
- **WHEN** BPF map 中存在匹配的 key 但 `bpf_ktime_get_ns() / 1e9 > expire_ts`
- **THEN** XDP 程序 SHALL 返回 `XDP_PASS`，按 cache miss 处理

### Requirement: XDP_TX response construction
Cache hit 时，XDP 程序 SHALL 构建完整的 DNS 响应包：swap Ethernet src/dst MAC、swap IP src/dst、swap UDP src/dst port、调整包大小（`bpf_xdp_adjust_tail`）、memcpy 预序列化响应到 DNS payload 区域、patch Transaction ID（从原始查询复制）、设置 IP TTL=64、重算 IP checksum、UDP checksum 设为 0。

#### Scenario: Successful XDP_TX response
- **WHEN** cache hit 且响应构建成功
- **THEN** XDP 程序 SHALL 返回 `XDP_TX`
- **AND** 响应包的 Ethernet/IP/UDP header MUST 正确 swap
- **AND** DNS Transaction ID MUST 匹配原始查询
- **AND** IP checksum MUST 正确

#### Scenario: Response size bounds check
- **WHEN** cache value 的 `resp_len` 超过 `MAX_DNS_RESPONSE_LEN`（512）
- **THEN** XDP 程序 SHALL 返回 `XDP_PASS`（安全回退）

#### Scenario: Packet adjust tail failure
- **WHEN** `bpf_xdp_adjust_tail()` 返回错误
- **THEN** XDP 程序 SHALL 返回 `XDP_PASS` 并递增 error 计数器

### Requirement: Per-CPU statistics counters
XDP 程序 SHALL 维护 per-CPU 统计计数器（`BPF_MAP_TYPE_PERCPU_ARRAY`）：hit、miss、pass（非 DNS 流量）、error。每个代码路径 MUST 在返回前更新对应计数器。

#### Scenario: Hit counter increments on cache hit
- **WHEN** XDP 程序成功从缓存返回响应（`XDP_TX`）
- **THEN** hit 计数器 SHALL 递增 1

#### Scenario: Miss counter increments on cache miss
- **WHEN** XDP 程序未命中缓存或 TTL 过期
- **THEN** miss 计数器 SHALL 递增 1

#### Scenario: Pass counter increments on non-DNS traffic
- **WHEN** XDP 程序因非 DNS 流量返回 `XDP_PASS`
- **THEN** pass 计数器 SHALL 递增 1

#### Scenario: Error counter increments on processing failure
- **WHEN** XDP 程序因 tail adjust 失败或其他错误返回 `XDP_PASS`
- **THEN** error 计数器 SHALL 递增 1

### Requirement: BPF maps definition
XDP 程序 SHALL 定义两个 BPF maps：
1. `cache_map` — `BPF_MAP_TYPE_HASH`，max_entries=65536，key=`cache_key`（qname + qtype），value=`cache_value`（expire_ts + resp_len + 预序列化 response）
2. `xdp_stats` — `BPF_MAP_TYPE_PERCPU_ARRAY`，max_entries=4（hit/miss/pass/error），key=u32，value=u64

#### Scenario: Cache map capacity
- **WHEN** cache_map 已满（65536 entries）且新条目写入
- **THEN** `bpf_map_update_elem` SHALL 返回 `-ENOSPC`（由 Go 侧处理）

#### Scenario: Stats map always available
- **WHEN** XDP 程序加载成功
- **THEN** xdp_stats map MUST 存在且可读写，初始值全为 0
