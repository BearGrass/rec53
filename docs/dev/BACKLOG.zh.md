# 需求积压

[English](BACKLOG.md) | 中文

## 模板

每条需求请按以下格式：

> ### [F-xxx] Title
> Priority: High / Medium / Low
> Description: 1-2 句说明需求
> Acceptance criteria:
> - Criterion 1
> - Criterion 2

使用这些前缀：

- `[F-xxx]` 表示 feature
- `[B-xxx]` 表示 bug
- `[O-xxx]` 表示 optimization

## In Progress

## Planned

## Unplanned

### [B-014] Glue 无 bailiwick 校验（安全风险）
Priority: Medium
Description: `getIPListFromResponse()` 直接采信 Additional 中的所有 A 记录作为 NS 地址，未验证 glue 是否在 bailiwick 范围内，存在 DNS cache poisoning 隐患。
Acceptance criteria:
- [ ] 提取 glue 时校验 A/AAAA 记录的名字是否在当前 zone 的 bailiwick 内
- [ ] out-of-bailiwick glue 触发 NS 子查询解析，而非直接使用
- [ ] 添加单元测试验证 bailiwick 校验逻辑

### [O-021] 无 glue 时委派 NS 不缓存
Priority: Medium
Description: ITER 只有在 `len(Ns)>0 && len(Extra)>0` 时才缓存委派信息，NS 没有 glue 时不会缓存 NS RRset，导致同一区域下次解析无法命中委派缓存。
Acceptance criteria:
- [ ] NS-only 响应（无 Extra）也应缓存 NS RRset
- [ ] 下次解析同区域时能从缓存找到委派起点，跳过上层迭代

### [O-022] Response ID 未校验（S7）
Priority: Low
Description: ITER 只校验 response.Question[0].Name，未校验 response.ID 是否与 query.ID 一致。
Acceptance criteria:
- [ ] 校验 `newResponse.Id == newQuery.Id`
- [ ] 添加单元测试验证 ID 校验

### [O-016] Add AAAA (IPv6) Support
Priority: High
Description: `getIPListFromResponse()` 只提取 IPv4（A）记录，缺少 IPv6（AAAA）支持。
Acceptance criteria:
- [ ] 同时提取 AAAA 记录
- [ ] `getBestAddressAndPrefetchIPs()` 支持 IPv6
- [ ] 用 AAAA 查询测试

### [O-006] TCP Retry for Truncated Responses (RFC 1035)
Priority: High
Description: UDP 响应被截断（TC 标志）时实现 TCP 重试。
Acceptance criteria:
- [ ] 检测 TC 标志
- [ ] TC 置位时通过 TCP 重试
- [ ] 处理更大的 TCP 响应

### [O-005] Implement Negative Caching (RFC 2308)
Priority: Medium
Description: 实现 NXDOMAIN 和 NODATA 的响应缓存。
Acceptance criteria:
- [ ] 用 SOA minimum field 的 TTL 缓存 NXDOMAIN
- [ ] 缓存 NODATA（成功但 Answer 为空）
- [ ] 为负缓存场景添加单元测试

## Completed

其余已完成条目与英文版保持一致；如需完整历史请查看 `BACKLOG.md`。
