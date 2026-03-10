# 代码质量分析与优化计划

**分析日期**: 2026-03-04
**更新日期**: 2026-03-10
**项目版本**: v0.1.0

---

## 1. 项目概览

| 指标 | 状态 |
|------|------|
| Go 版本 | 1.18 (建议升级至 1.21+) |
| 测试覆盖率 | ~1% |
| 代码行数 | ~600 LOC |
| 主要依赖 | miekg/dns, zap, prometheus, go-cache |

---

## 2. 关键问题

### 2.1 并发安全 - P0 (严重) ✅ 已修复

| 文件 | 行号 | 问题 | 状态 |
|------|------|------|------|
| `ip_pool.go` | 16-18 | `IPQuality.isInit` 无同步访问，存在 data race | ✅ 已修复 |
| `ip_pool.go` | 142 | `GetPrefetchIPs` 直接访问 `ipp.pool[bestIP].latency` 未加锁 | ✅ 已修复 |
| `state_define.go` | 206-217 | Prefetch goroutine 无生命周期管理 | ✅ 已修复 |

**修复方案**:
- 使用 `atomic.Bool` 替代 `bool` 类型
- 添加 `RLock/RUnlock` 保护
- 添加 `context.Context` 取消机制
- 添加 semaphore 限制并发数 (MAX_PREFETCH_CONCUR=10)
- 添加 `IPPool.Shutdown()` 方法

### 2.2 全局变量滥用 - P1 (高)

```go
// 这些全局变量阻碍了测试和依赖注入
var globalDnsCache = newCache()        // cache.go:10
var globalIPPool = NewIPPool()         // ip_pool.go:50
var Rec53Log *zap.SugaredLogger        // log.go:13
var Rec53Metric *Metric                // var.go:37
var dnsClient *dns.Client              // utils/net.go:14
```

**影响**:
- 无法进行单元测试隔离
- 无法运行多个实例
- 隐藏了依赖关系

**优化方案**: 依赖注入模式

```go
// 定义接口
type Cache interface {
    Get(key string) (*dns.Msg, bool)
    Set(key string, value *dns.Msg, ttl uint32)
    Delete(key string)
    Flush()
}

type DNSResolver struct {
    cache  Cache
    ipPool *IPPool
    // ...
}

func NewDNSResolver(opts ...Option) *DNSResolver {
    r := &DNSResolver{
        cache:  NewDefaultCache(),
        ipPool: NewIPPool(),
    }
    for _, opt := range opts {
        opt(r)
    }
    return r
}

// 使用函数选项模式
type Option func(*DNSResolver)

func WithCache(c Cache) Option {
    return func(r *DNSResolver) {
        r.cache = c
    }
}
```

### 2.3 错误处理问题 - P2 (中)

| 文件 | 行号 | 问题 |
|------|------|------|
| `monitor/log.go` | 42 | `SetLogLevel` 函数逻辑错误，不会生效 |
| `state_machine.go` | 多处 | 错误消息首字母大写，不符合 Go 惯例 |
| 多处 | - | 部分错误未使用 `fmt.Errorf("%w", err)` 包装 |

**优化方案**: 定义错误类型

```go
import "errors"

var (
    ErrNilRequest   = errors.New("request is nil")
    ErrNilResponse  = errors.New("response is nil")
    ErrInvalidState = errors.New("invalid state")
    ErrNoUpstream   = errors.New("no upstream servers available")
)

type StateError struct {
    State StateID
    Err   error
}

func (e *StateError) Error() string {
    return fmt.Sprintf("state %s: %v", e.State, e.Err)
}

func (e *StateError) Unwrap() error {
    return e.Err
}
```

### 2.4 性能问题 - P2 (中)

| 位置 | 问题 | 状态 |
|------|------|------|
| `state_define.go:237-239` | 每次请求创建新 `dns.Client`，无连接池 | 待优化 |
| `cache.go` | Cache key 未包含查询类型 | ✅ 已修复 |
| `ip_pool.go:140-148` | `GetPrefetchIPs` 遍历整个 pool | 待优化 |

**DNS Client 连接池优化**:

```go
var dnsClientPool = sync.Pool{
    New: func() interface{} {
        return &dns.Client{
            Net:     "udp",
            Timeout: 5 * time.Second,
        }
    },
}

func getDNSClient() *dns.Client {
    return dnsClientPool.Get().(*dns.Client)
}

func putDNSClient(c *dns.Client) {
    dnsClientPool.Put(c)
}
```

---

## 3. 状态机重构建议

### 当前问题

```go
// 当前设计 - 接口过大
type stateMachine interface {
    getCurrentState() int
    getRequest() *dns.Msg
    getResponse() *dns.Msg
    handle(request *dns.Msg, response *dns.Msg) (int, error)
}
```

### 优化方案

```go
// 优化后 - 接口分离
type State interface {
    State() StateID
}

type Handler interface {
    Handle(ctx context.Context, req *dns.Msg, resp *dns.Msg) (StateID, error)
}

type StateHandler interface {
    State
    Handler
}

// 使用状态类型而非 int
type StateID int

const (
    StateInit StateID = iota
    StateInCache
    StateCheckResp
    // ...
)

func (s StateID) String() string {
    // 实现字符串表示，便于调试
}
```

### Context 传播

```go
func Change(ctx context.Context, stm stateMachine) (*dns.Msg, error) {
    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }
        // 处理逻辑...
    }
}
```

---

## 4. 优化计划

### Phase 1: 并发安全修复 (P0) ✅ 已完成

- [x] 修复 `IPQuality.isInit` 的 data race
- [x] 修复 `GetPrefetchIPs` 的无锁访问
- [x] 管理 prefetch goroutine 生命周期

### Phase 2: 架构重构 (P1) - 待开始

- [ ] 引入依赖注入模式
- [ ] 状态机类型安全
- [ ] 日志级别设置修复 (`zap.AtomicLevel`)

### Phase 3: 性能优化 (P2) - 待开始

- [ ] DNS Client 连接池
- [ ] 减少内存分配 (`sync.Pool`)

### Phase 4: 测试与文档 (P2) - 待开始

- [ ] 补充单元测试 (目标覆盖率 > 80%)
- [ ] 添加基准测试
- [ ] 文档改进

### Phase 5: 安全加固 (P3) - 待开始

- [ ] 输入验证
- [ ] 资源限制
- [ ] Rate limiting

---

## 5. 任务执行策略

### 依赖关系图

```
Phase 1 ✅
├── Prefetch goroutine 生命周期
│   └── 已完成: context.Context + semaphore + Shutdown()
│
Phase 2: 架构重构
├── 依赖注入模式 (消除全局变量)
│   └── ⚠️ 会修改几乎所有核心文件
│   └── 依赖: Phase 1 完成 ✅
│
Phase 3-5
└── 依赖: Phase 2 完成 (否则重构后代码又要重写)
```

### 推荐执行顺序

```
优先级   任务                           原因                     状态
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
P0      Phase 1: 并发安全修复          建立稳定基线             ✅ 完成
        ↓
P1      依赖注入重构                   架构根本性改进，一次到位  待开始
        ↓
P1      补充单元测试 (>60%)            为后续优化提供安全网      待开始
        ↓
P2      性能优化                       有测试保障，可大胆优化    待开始
        ↓
P2      继续提高测试覆盖率 (>80%)                                待开始
        ↓
P3      安全加固                                                待开始
```

---

## 6. 快速修复清单

立即可做的小改动:

- [x] Cache key 包含查询类型
- [ ] 修正 `COMMEN` → `COMMON` 拼写错误 (`state.go`)
- [ ] 删除未使用的 `MAX_TIMEOUT` 常量 (`utils/net.go`)
- [ ] 修复 `SetLogLevel` 实现 (`monitor/log.go`)
- [ ] 升级 Go 版本至 1.21+
- [ ] 运行 `golangci-lint` 并修复所有警告

---

## 7. 变更日志

| 日期 | 变更 |
|------|------|
| 2026-03-10 | 合并 CODE_REVIEW.md 内容，重新组织文档结构 |
| 2026-03-04 | Phase 1 全部完成 |
| 2026-03-04 | 初始分析报告创建 |