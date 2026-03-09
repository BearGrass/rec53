# Go 代码审查报告 - rec53 项目

> 审查日期: 2026-03-09
> 审查范围: 全部 Go 源文件

## 一、整体架构评估

### 优点 ✅
- 状态机架构清晰，职责分离良好
- 优雅关闭实现正确
- Context 传播使用得当
- 并发控制使用 sync.WaitGroup 和 semaphore

### 需要改进 ⚠️
- 全局变量过多
- 接口设计不够简洁
- 错误处理不一致
- 部分代码存在 race condition 风险

---

## 二、详细优化建议

### 1. 状态机设计 (`server/state_machine.go`)

**问题**: 接口过于庞大，违反接口隔离原则

```go
// 当前设计 - 接口过大
type stateMachine interface {
    getCurrentState() int
    getRequest() *dns.Msg
    getResponse() *dns.Msg
    handle(request *dns.Msg, response *dns.Msg) (int, error)
}
```

**优化方案**: 使用更小的接口组合

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

**问题**: Change 函数过于复杂，重复代码多

```go
// 当前 - 大量重复的错误处理和日志
func Change(stm stateMachine) (*dns.Msg, error) {
    for {
        st := stm.getCurrentState()
        switch st {
        case STATE_INIT:
            if _, err := stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
                monitor.Rec53Log.Errorf("Handle state error %d %v", stm.getCurrentState(), err)
                return nil, fmt.Errorf("handle state error %d %v", stm.getCurrentState(), err)
            }
            // ...重复的代码模式
        }
    }
}
```

**优化方案**: 提取公共逻辑

```go
func Change(ctx context.Context, stm stateMachine) (*dns.Msg, error) {
    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }

        state := stm.getCurrentState()
        ret, err := stm.handle(stm.getRequest(), stm.getResponse())

        if err != nil {
            return nil, fmt.Errorf("state %s: %w", state, err)
        }

        next, err := transition(stm, state, ret)
        if err != nil {
            return nil, err
        }
        stm = next
    }
}

// 状态转换表驱动
var transitions = map[StateID]map[int]func(*dns.Msg, *dns.Msg) stateMachine{
    StateInit: {
        StateInitNoError: newInCacheState,
    },
    StateInCache: {
        InCacheHit:  newCheckRespState,
        InCacheMiss: newInGlueState,
    },
    // ...
}
```

---

### 2. IP Pool 并发安全 (`server/ip_pool.go`)

**问题**: 潜在的 race condition

```go
// 当前代码 - 检查和设置不是原子操作
func (ipp *IPPool) isTheIPInit(ip string) bool {
    ipq := ipp.GetIPQuality(ip)  // 获取
    if ipq == nil {
        ipq = &IPQuality{}
        ipq.Init()
        ipp.SetIPQuality(ip, ipq)  // 设置 - 期间可能被其他 goroutine 修改
    }
    return ipq.IsInit()
}
```

**优化方案**: 使用 sync.Map 或 double-check locking

```go
type IPPool struct {
    pool      sync.Map // 使用 sync.Map 替代 map + RWMutex
    ctx       context.Context
    cancel    context.CancelFunc
    wg        sync.WaitGroup
    sem       chan struct{}
    dnsClient *dns.Client
}

func (ipp *IPPool) GetOrInit(ip string) *IPQuality {
    // 先尝试读取
    if v, ok := ipp.pool.Load(ip); ok {
        return v.(*IPQuality)
    }

    // 创建新实例
    ipq := NewIPQuality()

    // LoadOrStore 保证原子性
    actual, _ := ipp.pool.LoadOrStore(ip, ipq)
    return actual.(*IPQuality)
}
```

**问题**: DNS Client 在每次迭代查询时重复创建

```go
// 当前 - 每次调用都创建新实例
func (s *iterState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
    // ...
    dnsClient := &dns.Client{}  // 每次都创建
    dnsClient.Net = "udp"
    dnsClient.Timeout = 5 * time.Second
}
```

**优化方案**: 使用 sync.Pool 复用

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

### 3. 全局变量重构 (`server/cache.go`, `server/ip_pool.go`)

**问题**: 全局变量使测试困难，且无法运行多个实例

```go
// 当前
var globalDnsCache = newCache()
var globalIPPool = NewIPPool()
```

**优化方案**: 依赖注入

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

---

### 4. 错误处理改进

**问题**: 错误信息不一致，缺少上下文

```go
// 当前
return nil, fmt.Errorf("handle state error %d %v", stm.getCurrentState(), err)
return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
```

**优化方案**: 定义错误类型，使用 errors.Is 和 errors.As

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

// 使用
if request == nil {
    return StateError{State: s.State(), Err: ErrNilRequest}
}
```

---

### 5. Context 传播 (`server/state_machine.go`)

**问题**: 状态机处理缺少 context，无法取消长时间操作

```go
// 当前
func Change(stm stateMachine) (*dns.Msg, error) {
    for {
        // 无限循环，无法取消
    }
}
```

**优化方案**: 添加 context 支持

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

// 在 ServeDNS 中传入 context
func (s *server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // ...
    result, err := Change(ctx, stm)
}
```

---

### 6. Monitor 包重构 (`monitor/log.go`)

**问题**: SetLogLevel 实现不正确

```go
// 当前 - 这个实现是错误的，只是检查 level 是否启用
func SetLogLevel(level zapcore.Level) {
    Rec53Log.Desugar().Core().Enabled(level)  // 只是检查，没有设置
}
```

**优化方案**: 使用 AtomicLevel

```go
var (
    atomicLevel zap.AtomicLevel
    Rec53Log    *zap.SugaredLogger
)

func InitLogger() {
    atomicLevel = zap.NewAtomicLevelAt(zap.InfoLevel)

    encoder := getEncoder()
    writeSyncer := getLogWriter()
    core := zapcore.NewCore(encoder, writeSyncer, atomicLevel)

    logger := zap.New(core, zap.AddCaller())
    Rec53Log = logger.Sugar()
}

func SetLogLevel(level zapcore.Level) {
    atomicLevel.SetLevel(level)
}

func GetLogLevel() zapcore.Level {
    return atomicLevel.Level()
}
```

---

### 7. Utils 包改进 (`utils/zone.go`)

**问题**: GetZoneList 可能 panic

```go
// 当前 - strings.Index 可能返回 -1
func GetZoneList(domain string) []string {
    for {
        domain = domain[strings.Index(domain, ".")+1:]  // 如果没有 "."，Index 返回 -1
        zoneList = append(zoneList, domain)
    }
}
```

**优化方案**: 更安全的实现

```go
func GetZoneList(domain string) []string {
    if domain == "" {
        return nil
    }

    var zones []string
    for {
        zones = append(zones, domain)

        idx := strings.Index(domain, ".")
        if idx == -1 {
            break
        }

        domain = domain[idx+1:]
        if domain == "" {
            break
        }
    }

    // 添加根域
    zones = append(zones, ".")
    return zones
}
```

---

### 8. 性能优化建议

#### 8.1 减少内存分配

```go
// 当前 - 每次创建新消息
func (s *iterState) handle(...) (int, error) {
    newQuery := new(dns.Msg)  // 每次分配
    newQuery.SetQuestion(...)
}

// 优化 - 复用对象池
var msgPool = sync.Pool{
    New: func() interface{} {
        return new(dns.Msg)
    },
}

func (s *iterState) handle(...) (int, error) {
    newQuery := msgPool.Get().(*dns.Msg)
    defer msgPool.Put(newQuery)

    newQuery.SetQuestion(...)
}
```

#### 8.2 预分配切片

```go
// 当前
ipList := []string{}  // 未预分配

// 优化
ipList := make([]string, 0, len(response.Extra))  // 预分配容量
```

#### 8.3 使用 strings.Builder

```go
// 当前 - 多次字符串拼接
t.Logf("...")

// 对于高频拼接，使用 strings.Builder
var sb strings.Builder
sb.WriteString("state: ")
sb.WriteString(state.String())
// ...
```

---

## 三、代码规范建议

### 1. 命名规范

```go
// 当前 - 拼写错误
CHECK_RESP_COMMEN_ERROR  // 应为 COMMON

// 当前 - 风格不一致
func getCache(key string)      // 私有
func GetZoneList(domain string) // 公开但无 doc

// 建议
// CheckRespCommonError - 正确拼写
// GetCache - 如果需要导出
// getCache - 如果是内部使用
```

### 2. 添加文档注释

```go
// 当前缺少文档
func Change(stm stateMachine) (*dns.Msg, error)

// 建议
// Change executes the state machine until a final state is reached.
// It returns the DNS response message or an error if the state machine
// encounters an invalid state or fails to handle a request.
//
// The state machine follows this flow:
//   STATE_INIT -> IN_CACHE -> CHECK_RESP -> ...
func Change(stm stateMachine) (*dns.Msg, error)
```

### 3. 常量分组

```go
// 当前 - 分散定义
const (
    INIT_IP_LATENCY     = 1000
    MAX_IP_LATENCY      = 10000
    MAX_PREFETCH_CONCUR = 10
    PREFETCH_TIMEOUT    = 3
)

// 建议 - 使用更清晰的分组和类型
type Config struct {
    InitIPLatency     time.Duration
    MaxIPLatency      time.Duration
    MaxPrefetchConcur int
    PrefetchTimeout   time.Duration
}

var DefaultConfig = Config{
    InitIPLatency:     1000 * time.Millisecond,
    MaxIPLatency:      10 * time.Second,
    MaxPrefetchConcur: 10,
    PrefetchTimeout:   3 * time.Second,
}
```

---

## 四、重构优先级

| 优先级 | 问题 | 影响 | 工作量 | 文件 |
|--------|------|------|--------|------|
| P0 | IP Pool race condition | 数据正确性 | 中 | `server/ip_pool.go` |
| P0 | SetLogLevel 实现错误 | 功能缺失 | 低 | `monitor/log.go` |
| P1 | 添加 Context 传播 | 可取消性 | 高 | `server/state_machine.go` |
| P1 | 全局变量重构 | 可测试性 | 高 | `server/cache.go`, `server/ip_pool.go` |
| P2 | 状态机重构 | 可维护性 | 高 | `server/state_machine.go` |
| P2 | 错误类型定义 | 调试体验 | 中 | 新建 `server/errors.go` |
| P3 | 性能优化池化 | 性能 | 中 | `server/state.go` |

---

## 五、建议的目录结构

```
rec53/
├── cmd/
│   └── rec53/
│       └── main.go          # 入口
├── internal/                 # 内部包（不对外暴露）
│   ├── cache/               # 缓存实现
│   │   ├── cache.go
│   │   └── cache_test.go
│   ├── resolver/            # DNS 解析器
│   │   ├── resolver.go
│   │   ├── state_machine.go
│   │   └── states.go
│   ├── pool/                # IP 池
│   │   ├── pool.go
│   │   └── quality.go
│   └── config/              # 配置
│       └── config.go
├── pkg/                      # 可对外暴露的包
│   └── api/
└── monitor/                  # 监控（可保持现状）
```

---

## 六、相关文件列表

| 文件 | 主要问题 | 建议操作 |
|------|----------|----------|
| `server/ip_pool.go` | Race condition, 全局变量 | P0 修复 |
| `monitor/log.go` | SetLogLevel 实现错误 | P0 修复 |
| `server/state_machine.go` | 缺少 Context, 重复代码 | P1 重构 |
| `server/cache.go` | 全局变量 | P1 重构 |
| `server/state.go` | 每次创建 DNS Client | P3 优化 |
| `utils/zone.go` | 可能 panic | P2 修复 |
| `server/state_define.go` | 命名拼写错误 | P3 修复 |

---

## 七、参考资料

- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [Go Concurrency Patterns: Context](https://go.dev/blog/context)