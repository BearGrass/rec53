# 编码约定

[English](CONVENTIONS.md) | 中文

## 语言风格

使用 Go 1.21+ 和标准 `gofmt` 格式。提交前运行 `gofmt -w .`。

## 命名约定

| 元素 | 规范 | 示例 |
|------|------|------|
| 包 | 小写，单词 | `server`、`monitor`、`utils` |
| 类型/结构体 | PascalCase | `IPPool`、`stateMachine`、`checkRespState` |
| 函数/方法 | 导出用 PascalCase，私有用 camelCase | `NewServer`、`getBestIPs` |
| 常量 | SCREAMING_SNAKE_CASE | `STATE_INIT`、`MAX_IP_LATENCY` |
| 接口 | -er 后缀 | `stateMachine` |

## 状态机模式

每个状态都是实现 `stateMachine` 接口的结构体：

```go
type stateMachine interface {
    getCurrentState() int
    getRequest() *dns.Msg
    getResponse() *dns.Msg
    handle(request *dns.Msg, response *dns.Msg) (int, error)
}
```

构造器模式：

```go
func newInCacheState(req, resp *dns.Msg) *inCacheState {
    return &inCacheState{
        request:  req,
        response: resp,
    }
}
```

## 错误处理

状态通过整数码做流控制，而不是直接返回错误：

```go
const (
    IN_CACHE_HIT_CACHE  = 0
    IN_CACHE_MISS_CACHE = 1
    IN_CACHE_COMMON_ERROR = -1
)

func (s *inCacheState) handle(request, response *dns.Msg) (int, error) {
    if request == nil || response == nil {
        return IN_CACHE_COMMON_ERROR, fmt.Errorf("request is nil or response is nil")
    }
    return IN_CACHE_HIT_CACHE, nil
}
```

## 全局实例

```go
var globalDnsCache = newCache()
var globalIPPool = NewIPPool()
var Rec53Metric *Metric
var Rec53Log *zap.SugaredLogger
```

## 日志

使用全局 `Rec53Log` 及其 level 方法：

```go
monitor.Rec53Log.Debugf("try to get cache %s (type: %s)", q.Name, dns.TypeToString[q.Qtype])
monitor.Rec53Log.Errorf("Handle state error %d %v", stm.getCurrentState(), err)
monitor.Rec53Log.Infof("rec53 started, listening on %s", *listenAddr)
```

## 测试模式

优先使用 table-driven tests：

```go
func TestGetZoneList(t *testing.T) {
    tests := []struct {
        input    string
        expected []string
    }{
        {"example.com.", []string{"example.com.", "com.", "."}},
        {".", []string{".", "."}},
    }
    for _, tt := range tests {
        result := GetZoneList(tt.input)
        // assert...
    }
}
```

`e2e/helpers.go` 里的 helper 可以提供 mock DNS server：

```go
func TestWithMockServer(t *testing.T) {
    zone := Zone{ Name: "example.com.", Records: []dns.RR{ A("example.com.", "192.0.2.1") } }
    server := NewMockAuthorityServer(zone)
    defer server.Shutdown()
}
```

## 性能回归流程

基准/load/pprof 回归规则以 `docs/testing/perf-regression.md` 为准。
任何性能敏感变更都应按该文档给出前后数据。

## Code review 清单

- [ ] 错误信息包含足够上下文
- [ ] 日志包含相关 query/domain 信息
- [ ] 测试覆盖边界和错误路径
- [ ] 状态处理器返回正确的 code
- [ ] 缓存操作使用 copy 函数避免修改共享数据
- [ ] 读取缓存值时不修改单个 RR 字段（见 Cache Read Safety）
- [ ] 优雅关闭 context 正确传递

## Cache 读安全

`getCacheCopy` / `getCacheCopyByType` 返回的是缓存 `*dns.Msg` 的**浅拷贝**：新的 slice header（`Question`、`Answer`、`Ns`、`Extra`），但 RR 指针与缓存条目共享。

**安全操作**：
- 追加、截断、置空或重设 slice header
- 读取任何 RR 字段
- 调用 `Pack()`（写入时已剥离 OPT，所以 `Pack()` 无副作用）

**禁止操作**：
- 修改单个 RR 结构体字段
- 修改读到的 `Question` 条目字段

违反这些规则会污染缓存并与并发读者产生 race。
如果未来需要修改某个 RR 字段，必须先对该 RR 做深拷贝。

## IP 质量跟踪约定

1. **状态管理**
   - `isInit=true`：IP 尚未测量（默认 latency 1000ms）
   - `isInit=false`：IP 已通过预取测量
   - 做关键决策前先检查 `IsInit()`

2. **延迟更新**
   - 用 `SetLatency()` 更新延迟并保留 init 状态
   - 用 `SetLatencyAndState()` 更新延迟并标记为已测量
   - 并发访问用原子操作，不要手动加锁

3. **最佳 IP 选择**
   - `getBestIPs()` 返回 `(best, secondBest)`
   - 第一个返回值是绝对最低延迟
   - 第二个返回值是第二低延迟
   - 查询至少要用返回的 best IP

4. **预取策略**
   - `GetPrefetchIPs()` 识别后台测量候选
   - 候选条件：`[bestLatency × 0.9, bestLatency]`
   - 这个范围用于发现可能更好的服务器，同时避免过载
   - `PrefetchIPs()` 用 semaphore 限制并发（默认 10 goroutine）

5. **质量提升**
   - `UpIPsQuality()` 只对已测量 IP 降低 latency 10%
   - 成功解析后调用，奖励表现好的上游
   - 跳过未测量 IP（`isInit=true`）以保留基线预期

6. **并发安全**
   - `IPQuality`：只用原子操作，不需要外部锁
   - `IPPool`：用 `GetIPQuality()` / `SetIPQuality()` 访问 map
   - 不要直接访问 `pool` map；始终使用提供的方法
   - 预取 goroutine 需要响应 context cancellation
