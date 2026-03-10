# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build
go build -o rec53 cmd/rec53.go

# Build with version info
go build -ldflags "-X main.version=X.X.X -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o rec53 cmd/rec53.go

# Run (DNS server on port 5353, metrics on 9999)
./rec53

# Run with custom ports
./rec53 -listen 0.0.0.0:53 -metric :9099 -log-level debug

# Build Docker image
docker build -t rec53 .

# Run with Docker Compose (includes Prometheus, node-exporter)
cd single_machine && docker-compose up -d
```

### CLI Flags
- `-listen` - DNS server address (default: `127.0.0.1:5353`)
- `-metric` - Prometheus metrics address (default: `:9999`)
- `-log-level` - Log level: debug, info, warn, error (default: `info`)
- `-version` - Show version information

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test -v ./server/...

# Run specific test
go test -v -run TestCacheHit ./server/

# Run E2E tests (require network access)
go test -v ./e2e/...

# Format code before commit
gofmt -w .

# Run linter
go vet ./...
```

### Current Test Coverage

```
rec53/cmd      47.1%  (parseLogLevel, 信号处理已测试) ✅
rec53/monitor  58.1%  (metric 测试已完善) ✅
rec53/server   76.8%  (大部分已覆盖) ✅
rec53/utils    82.6%  (Zone, Root 已测试) ✅
rec53/e2e      28.6%  (需要网络，部分失败)
```

### Test Files

**cmd/signal_test.go** - Signal handling tests
- SIGINT/SIGTERM graceful shutdown
- gracefulShutdown and waitForSignal functions
- Version and log-level flag tests

**monitor/metric_test.go** - Prometheus metrics tests
- Counter, Histogram, Gauge operations
- HTTP endpoint and concurrent access
- InitMetricWithAddr and ShutdownMetric

**server/state_machine_test.go** - State machine tests
- State transitions and error handling
- RET_RESP terminal state behavior

**server/state_define_test.go** - State definition tests
- iterState error handling (nil request/response, empty extra)
- getIPListFromResponse with mixed record types
- getBestAddressAndPrefetchIPs latency-based selection
- IPQuality concurrent access safety
- IPPool getBestIPs and updateIPQuality methods

**server/cache_test.go** - Cache tests
- TTL, concurrency, type-based caching

**server/ip_pool_test.go** - IP pool tests
- Quality scoring, failover logic

See `.rec53/TEST_PLAN.md` for detailed test documentation.

## Architecture

**Recursive DNS resolver** implemented as a state machine with caching and IP quality tracking.

### Package Structure
- `cmd/` - Entry point, flag parsing, signal handling, graceful shutdown
- `server/` - Core DNS logic: state machine, cache, IP pool, UDP/TCP server
- `monitor/` - Prometheus metrics (`metric.go`) and zap logger with rotation (`log.go`)
- `utils/` - Zone resolution (`GetZoneList`), root servers
- `e2e/` - End-to-end integration tests with mock DNS servers
- `single_machine/` - Docker Compose setup for local deployment

### State Machine (`server/state_machine.go`, `server/state_define.go`)

The DNS resolution flow is a state machine where each state implements the `stateMachine` interface and returns a result code that determines the next state.

**States and Transitions:**
```
STATE_INIT → IN_CACHE
                ├─ HIT_CACHE → CHECK_RESP
                └─ MISS_CACHE → IN_GLUE
                                  ├─ EXIST → ITER
                                  └─ NOT_EXIST → IN_GLUE_CACHE
                                                   ├─ HIT_CACHE → ITER
                                                   └─ MISS_CACHE → ITER

CHECK_RESP:
  ├─ GET_ANS → RET_RESP (done)
  ├─ GET_CNAME → IN_CACHE (follow CNAME chain, tracked for cycles)
  └─ GET_NS → IN_GLUE

ITER:
  ├─ NO_ERROR → CHECK_RESP
  └─ ERROR → return SERVFAIL
```

**Key Implementation Details:**
- `MaxIterations = 50` prevents infinite loops (state_machine.go:13)
- CNAME cycle detection via `visitedDomains` map (state_machine.go:27)
- Original question is preserved and restored in response (state_machine.go:31, :174)
- ITER state handles EDNS0 with 4096 buffer size (state_define.go:249)

### Key Components

**Cache (`server/cache.go`):**
- Uses `patrickmn/go-cache` library with 5min default TTL, 10min cleanup interval
- Cache key format: `domain:qtype` (e.g., `google.com.:1` for A record)
- `getCacheCopy()` returns deep copy to prevent concurrent modification
- Global instance: `globalDnsCache`

**IP Pool (`server/ip_pool.go`):**
- Tracks upstream nameserver quality via latency scoring
- Initial latency: 1000ms, max penalty: 10000ms
- Prefetch mechanism with max 10 concurrent checks (`MAX_PREFETCH_CONCUR`)
- Global instance: `globalIPPool`
- Must call `Shutdown()` for graceful termination

**Server (`server/server.go`):**
- Single `ServeDNS` handler for both UDP and TCP
- UDP truncation handling with TC flag (server.go:88-120)
- Graceful shutdown via context with 5-second timeout

**Metrics (`monitor/metric.go`):**
- `rec53_query_counter` - Incoming queries (stage, name, type)
- `rec53_response_counter` - Outgoing responses (stage, name, type, code)
- `rec53_latency` - Query latency histogram (buckets: 10, 50, 200, 1000, 3000 ms)
- `rec53_ip_quality` - Nameserver latency gauge

## Dependencies

- `github.com/miekg/dns` - DNS library for server and message handling
- `github.com/patrickmn/go-cache` - In-memory cache with TTL
- `go.uber.org/zap` - Structured logging
- `github.com/prometheus/client_golang` - Metrics exposition
- `gopkg.in/natefinch/lumberjack.v2` - Log rotation (max 1MB, 5 backups, 30 days)

## E2E Testing Pattern

The `e2e/` package provides integration testing utilities:

```go
// Create mock authority server
zone := &Zone{
    Origin: "example.com.",
    Records: map[uint16][]dns.RR{
        dns.TypeA: {A("example.com.", "192.0.2.1", 300)},
    },
}
mock := NewMockAuthorityServer(t, zone)
defer mock.Stop()

// Create test resolver
handler := server.NewServer("127.0.0.1:0")
tr, _ := NewTestResolver(handler)
defer tr.Stop()

// Query
resp, err := tr.Query("example.com.", dns.TypeA)
```

## Coding Conventions
详细编码规范见 `.rec53/CONVENTIONS.md`。核心原则：遵循 Effective Go，gofmt 格式化，错误必须处理。


### Go Style
- Follow [Effective Go](https://golang.org/doc/effective_go)
- Exported functions must have doc comments
- Handle errors explicitly, do not ignore

### Naming
- Package: lowercase, no underscores
- Exported: PascalCase
- Private: camelCase

### Error Handling
- Never ignore errors
- Wrap with context: `fmt.Errorf("context: %w", err)`
- Error messages: lowercase, no trailing period

## Reference Implementations

| Project | Feature to Reference |
|---------|---------------------|
| BIND 9 | Authoritative + Recursive, DNSSEC |
| Unbound | State machine architecture, DNSSEC validation |
| CoreDNS | Plugin system, Go patterns |

## Known Issues

- `www.huawei.com` resolution may have issues with certain CNAME chains
- Some domains with complex CNAME chains may return SERVFAIL when the final A/AAAA resolution fails

## Documentation

Project documentation in `.rec53/`:
- [`.rec53/README.md`](../.rec53/README.md) - Documentation index
- `.rec53/ARCHITECTURE.md` - System architecture, state machine, data flow
- `.rec53/ROADMAP.md` - Version roadmap and requirements
- `.rec53/TEST_PLAN.md` - Test coverage improvement plan
- `.rec53/TODO.md` - Daily task management
- `.rec53/CONVENTIONS.md` - Go code style, naming, error handling
- `.rec53/decisions/` - Architecture Decision Records (ADR)

## 文档自维护规则

### CLAUDE.md 更新时机
当以下情况发生时，你必须同步更新 CLAUDE.md 对应章节：
- 新增了包或目录 → 更新「目录结构」
- 新增了外部依赖 → 更新依赖说明
- 修改了接口抽象或 mock 策略 → 更新「测试规范」
- 新增了常用命令或构建步骤 → 更新「常用命令」
- 发现 CLAUDE.md 中的描述与实际代码不符 → 当场修正

### README.md 更新时机
当以下情况发生时，你必须同步更新 README.md：
- 新增了用户可感知的功能 → 更新功能列表
- 修改了配置格式或启动参数 → 更新使用说明
- 修改了构建方式或依赖要求 → 更新安装步骤
- 版本号变更 → 更新版本标识

### TODO.md 更新时机
当以下情况发生时，你必须同步更新 .rec53/TODO.md：
- 完成了一个待办任务 → 将该条目从「待办」移到「已完成」，或从「当前任务」中移除并提上下一个
- 发现了新的 bug → 添加一条 BUG 标签的待办，标注发现场景
- 发现源码需要优化但不在当前任务范围内 → 添加一条 OPT 标签的待办
- 当前任务被中断（上下文不足、等待确认等）→ 更新「当前任务」的进展 checkbox 到最新状态
- 完成的代码引入了新的技术债 → 添加一条待办并标注来源

### 执行方式
- 不要单独开一轮来更新文档，在完成相关代码改动的同一次任务中顺手更新
- 更新后在回复中用一行说明改了什么，例如："已更新 CLAUDE.md 目录结构，新增 internal/middleware/"

