## ADDED Requirements

### Requirement: 缓存 glueless NS referral（写侧）
`QUERY_UPSTREAM` 在收到上游响应时，若响应包含 Ns section 且 Extra section 为空（glueless NS referral），SHALL 将该响应写入缓存，key 为 `Ns[0].Header().Name`，TTL 为 `Ns[0].Header().Ttl`。

#### Scenario: glueless NS referral 被写入缓存
- **WHEN** `QUERY_UPSTREAM` 收到上游响应，`len(Ns) > 0` 且 `len(Extra) == 0`
- **THEN** 调用 `setCacheCopy(Ns[0].Header().Name, response, Ns[0].Header().Ttl)` 将该 NS referral 写入缓存

#### Scenario: 带 glue 的 NS referral 写入行为不变
- **WHEN** `QUERY_UPSTREAM` 收到上游响应，`len(Ns) > 0` 且 `len(Extra) > 0`
- **THEN** 调用 `setCacheCopy(Ns[0].Header().Name, response, Ns[0].Header().Ttl)` 写入缓存（现有行为保持不变）

---

### Requirement: 命中 glueless NS 缓存（读侧）
`LOOKUP_NS_CACHE` 在遍历 zone list 时，若缓存中存在仅含 Ns section（Extra 为空）的条目，SHALL 将该条目的 Ns 记录拷贝到 response，并返回 `LOOKUP_NS_CACHE_HIT`。

#### Scenario: 命中 glueless 缓存条目
- **WHEN** `LOOKUP_NS_CACHE` 在 zone list 中找到缓存条目，`len(Ns) > 0`，`len(Extra) == 0`
- **THEN** 将 `Ns` 拷贝到 response，`Extra` 保持为空，返回 `LOOKUP_NS_CACHE_HIT`

#### Scenario: 命中带 glue 的缓存条目（行为不变）
- **WHEN** `LOOKUP_NS_CACHE` 在 zone list 中找到缓存条目，`len(Ns) > 0`，`len(Extra) > 0`
- **THEN** 将 `Ns` 和 `Extra` 均拷贝到 response，返回 `LOOKUP_NS_CACHE_HIT`（现有行为保持不变）

#### Scenario: 未命中时回退到根 glue（行为不变）
- **WHEN** `LOOKUP_NS_CACHE` 遍历所有 zone 均未找到有效 Ns 条目
- **THEN** 使用根 glue，返回 `LOOKUP_NS_CACHE_MISS`

---

### Requirement: EXTRACT_GLUE 允许 glueless NS 通过
`EXTRACT_GLUE` 在收到 glueless NS response（`len(Ns) > 0`，`len(Extra) == 0`）时，若 NS zone 是当前查询域名的祖先域，SHALL 返回 `EXTRACT_GLUE_EXIST`，使状态机继续进入 `QUERY_UPSTREAM`。

#### Scenario: glueless NS zone 匹配查询域名
- **WHEN** `EXTRACT_GLUE` 收到 response，`len(Ns) > 0`，`len(Extra) == 0`，且 `dns.IsSubDomain(nsZone, queryName)` 为 true
- **THEN** 返回 `EXTRACT_GLUE_EXIST`，`Ns` 保留，`Extra` 保持为空

#### Scenario: glueless NS zone 不匹配查询域名
- **WHEN** `EXTRACT_GLUE` 收到 response，`len(Ns) > 0`，`len(Extra) == 0`，且 `dns.IsSubDomain(nsZone, queryName)` 为 false
- **THEN** 清空 `Ns` 和 `Extra`，返回 `EXTRACT_GLUE_NOT_EXIST`

#### Scenario: 带 glue 且 zone 匹配（行为不变）
- **WHEN** `EXTRACT_GLUE` 收到 response，`len(Ns) > 0`，`len(Extra) > 0`，且 `dns.IsSubDomain(nsZone, queryName)` 为 true
- **THEN** 返回 `EXTRACT_GLUE_EXIST`（现有行为保持不变）

#### Scenario: 带 glue 但 zone 不匹配（行为不变）
- **WHEN** `EXTRACT_GLUE` 收到 response，`len(Ns) > 0`，`len(Extra) > 0`，且 `dns.IsSubDomain(nsZone, queryName)` 为 false
- **THEN** 清空 `Ns` 和 `Extra`，返回 `EXTRACT_GLUE_NOT_EXIST`（现有行为保持不变）
