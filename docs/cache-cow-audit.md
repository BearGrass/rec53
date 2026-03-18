# Cache COW 设计审计

## 审计目的

审计 `getCacheCopy` / `getCacheCopyByType` 全部生产调用方的可变性，评估移除防御性 `dns.Msg.Copy()` 的可行性和风险。

## 缓存读写机制

### 当前实现

| 函数 | 位置 | 行为 |
|------|------|------|
| `getCacheCopy(key)` | `server/cache.go:46-53` | 返回 `msg.Copy()` 深拷贝 |
| `getCacheCopyByType(name, qtype)` | `server/cache.go:33-36` | 包装 `getCacheCopy`，构造 key |
| `setCacheCopy(key, value, expire)` | `server/cache.go:62-64` | 存储 `value.Copy()` 深拷贝 |

`dns.Msg.Copy()` 是 `miekg/dns` 库的深拷贝方法，会对每个 RR 调用 `dns.Copy(rr)` 创建独立副本。读写两端均拷贝，确保全局缓存永不被调用方修改。

## 调用方可变性审计清单

| # | 位置 | 函数 | 调用 | 判定 | 依据 |
|---|------|------|------|------|------|
| 1 | `state_cache_lookup.go:37` | `cacheLookupState.handle()` | `getCacheCopyByType(q.Name, q.Qtype)` | **MUTATING（别名）** | L40: `s.response.Answer = append(s.response.Answer, msgInCache.Answer...)` — RR 元素通过 spread 操作移入 `s.response.Answer`。虽然未直接修改 RR 字段，但 RR 指针被别名到 response 中，下游代码（如 TTL 递减、名称压缩）若修改这些 RR 将影响原始数据 |
| 2 | `state_lookup_ns_cache.go:38` | `lookupNSCacheState.handle()` | `getCacheCopy(zone)` | **MUTATING（别名）** | L47: `s.response.Ns = append(s.response.Ns, msgInCache.Ns...)`; L48: `s.response.Extra = append(s.response.Extra, msgInCache.Extra...)` — 与 #1 相同的别名模式，Ns/Extra 切片元素被共享到 response |
| 3 | `state_query_upstream.go:194` | `resolveNSIPs()` | `getCacheCopyByType(nsName, dns.TypeA)` | **READ_ONLY** | L195-198: 仅遍历 `msgInCache.Answer`，类型断言后读取 `a.A.String()` 提取 IP 字符串。无修改、无别名、无副作用 |

### 额外 `.Copy()` 调用（非缓存路径）

| 位置 | 上下文 | 必要性 |
|------|--------|--------|
| `state_query_upstream.go:122` | `go launch(bestAddr, query.Copy())` | **必要** — `dns.Client.ExchangeContext` 会修改 `msg.Id`，两个 Happy Eyeballs goroutine 需要独立副本防止数据竞争 |
| `state_query_upstream.go:123` | `go launch(secondAddr, query.Copy())` | **必要** — 同上 |

## 设计草案：消除防御性 Copy()

### 方案 A：ReadOnlyMsg 包装类型

```go
// ReadOnlyMsg wraps a *dns.Msg and exposes only read-only accessors.
// The underlying message is never exposed directly.
type ReadOnlyMsg struct {
    msg *dns.Msg  // unexported, prevents direct field access
}

func (r ReadOnlyMsg) Answer() []dns.RR { return r.msg.Answer }
func (r ReadOnlyMsg) Ns() []dns.RR     { return r.msg.Ns }
func (r ReadOnlyMsg) Extra() []dns.RR  { return r.msg.Extra }
// ... other read-only accessors

// getCacheReadOnly returns a ReadOnlyMsg without copying.
// Callers MUST NOT modify the returned RR slices or elements.
func getCacheReadOnly(key string) (ReadOnlyMsg, bool) { ... }
```

**问题**：Go 的类型系统无法阻止 `rr.(*dns.A).A = net.IP{...}` 这样的修改——`dns.RR` 接口返回的是指针类型，调用方可以通过类型断言获得可变引用。包装类型只能提供"约定级"保护，不能提供"编译期"保护。

### 方案 B：只读契约 + Linter 规则

不改变数据类型，建立全局只读契约：
1. 新增 `getCacheRef` 函数，返回不经 Copy 的 `*dns.Msg`
2. 所有调用方遵守"不修改返回值"契约
3. CI 中添加 linter 规则检测对 cache 返回值的写操作

**问题**：Go 缺少内置的不可变性 linter，自定义 linter 维护成本高，且无法覆盖所有间接修改路径。

### 方案 C：选择性移除 Copy（仅限 READ_ONLY 调用方）

仅对审计确认为 READ_ONLY 的调用方（#3 `resolveNSIPs`）提供 `getCacheRef` 接口，MUTATING 调用方继续使用 `getCacheCopy`。

**优势**：最小变更，风险可控。
**收益**：每次 NS IP 缓存命中减少一次 `dns.Msg` 深拷贝（通常 1-3 个 Answer RR，~200-500 bytes）。
**风险**：未来新增调用方可能误用 `getCacheRef` 并修改返回值。

## 风险评估

| 风险 | 严重程度 | 概率 | 缓解措施 |
|------|----------|------|----------|
| 移除 Copy 后调用方意外修改缓存 | **高** — 导致缓存污染，影响所有后续查询 | 中（当前调用方安全，但未来新代码可能引入） | 包装类型 + 代码审查流程 |
| 别名导致跨查询数据泄漏 | **高** — 不同查询共享相同 RR 指针 | 低（需要特定的并发访问模式） | 保留 MUTATING 调用方的 Copy |
| 性能收益不显著 | **低** — 仅浪费开发时间 | 中（缓存命中路径可能不是分配热点） | pprof 数据验证门槛 |

## 性能收益预期

- **cache hit 路径**：每次命中节省一次 `dns.Msg.Copy()`，约 500-2000 ns（取决于 RR 数量）
- **cache miss 路径**：无影响（不走缓存读取）
- **总体影响**：取决于缓存命中率。命中率 80% 时，每次查询平均节省 ~1 次 Copy，约 1-2 us
- **与总查询延迟比较**：缓存命中查询总延迟 ~5-15 us，Copy 占 10-30%

## 实施硬门槛

### 条件

**仅当 pprof heap profile 证明 `dns.Msg.Copy()` 调用占总堆分配的 >30% 时，才进入代码实施阶段。**

### 验证步骤

```bash
# 1. 启用 pprof
# config.yaml:
#   debug:
#     pprof_enabled: true
#     pprof_listen: "127.0.0.1:6060"

# 2. 在真实负载下运行至少 5 分钟

# 3. 采集 heap profile
go tool pprof -inuse_objects http://127.0.0.1:6060/debug/pprof/heap

# 4. 在 pprof 交互式界面中检查 Copy 占比
(pprof) top 20
# 查看 dns.(*Msg).Copy 在 inuse_objects 中的排名和占比

# 5. 如果 Copy 占 >30%，记录证据并启动实施
# 如果 <30%，Cache COW 优化的 ROI 不足，不实施
```

### 决策记录模板

```
日期: YYYY-MM-DD
pprof 采集时长: X 分钟
负载描述: Y QPS / Z 个域名
dns.(*Msg).Copy inuse_objects: N (X%)
总 inuse_objects: M
决策: [实施 / 不实施]
```
