# AGENTS.md

Practical guidance for agentic coding systems operating on this repository.

## Build & Run

```bash
go build -o rec53 ./cmd          # build binary
./generate-config.sh             # generate default config (first run only)
./rec53 --config ./config.yaml
./rec53 --config ./config.yaml --no-warmup
./rec53 --config ./config.yaml -listen 0.0.0.0:53 -metric :9099 -log-level debug
```

## Test Commands

```bash
# Full suite (always use -race for concurrent code)
go test -race ./...
go test -race -timeout 120s ./... -count=1   # disable test cache

# Single test — most common pattern for agents
go test -v -run TestNameHere ./package/...
# Examples:
go test -v -run TestResolverIntegration ./e2e/...
go test -v -run TestIPPoolSelection ./server/...
go test -v -run TestCacheHitMiss ./server/...

# Package-level runs
go test -v ./server/...
go test -v ./e2e/...
go test -short ./...   # skip long-running integration tests

# Coverage
go test -cover ./...
```

## Code Style

### Formatting & Imports

- `gofmt -w .` before every commit; Go 1.21+
- Import groups: stdlib → external → internal (`rec53/*`), blank line between each:

```go
import (
    "fmt"
    "time"

    "github.com/miekg/dns"
    "github.com/prometheus/client_golang/prometheus"

    "rec53/monitor"
    "rec53/server"
)
```

### Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Packages | lowercase, single word | `server`, `monitor`, `utils` |
| Exported types/funcs | PascalCase | `IPPool`, `NewServer`, `GetBestIPsV2` |
| Unexported types/funcs | camelCase | `inCacheState`, `getBestIPs` |
| Constants | SCREAMING_SNAKE_CASE | `STATE_INIT`, `MAX_IP_LATENCY`, `IN_CACHE_HIT_CACHE` |
| Package-level globals | `global` + PascalCase | `globalDnsCache`, `globalIPPool` |
| Context keys | unexported named type | `type contextKeyType string` |

### Types & Receivers

- Pointer receivers for state-mutating methods: `func (s *inCacheState) handle(...)`
- Value receivers when state is read-only and struct is small
- State structs always embed `request`, `response *dns.Msg` and `ctx context.Context`

### Error Handling

- State `handle()` methods return `(int, error)` — int is the next state code, error is context
- Return codes defined in `server/state_machine.go` (e.g. `IN_CACHE_HIT_CACHE`, `ITER_COMMON_ERROR`)
- Always include context in error strings:

```go
return ITER_COMMON_ERROR, fmt.Errorf("request is nil in %s", s.getCurrentState())
```

- Never swallow errors silently; log at `Debugf` or `Errorf` before returning

### Logging

Use `monitor.Rec53Log` (zap.SugaredLogger):

```go
monitor.Rec53Log.Debugf("[STATE] cache hit for %s (type: %s)", q.Name, dns.TypeToString[q.Qtype])
monitor.Rec53Log.Errorf("[STATE] handler failed: state=%d err=%v", stm.getCurrentState(), err)
```

- Prefix log lines with `[STATE_NAME]` for easy grep
- Include domain, query type, and IP in log messages

### Concurrency

- `sync.RWMutex` for shared maps (`globalIPPool`, `globalDnsCache`)
- `RLock`/`RUnlock` for reads; `Lock`/`Unlock` for writes — always `defer` unlock immediately
- `sync/atomic` for counters/booleans inside `IPQuality` structs (lock-free fast path)
- Context-based cancellation for goroutines; no bare `time.Sleep` in loops
- Semaphore pattern (`make(chan struct{}, N)`) to cap goroutine concurrency

### Comments

- Export comments start with the identifier name; explain *why*, not just what
- Complex algorithms (state transitions, concurrency design) deserve block comments

```go
// IPQualityV2 tracks response latency using a sliding window ring buffer
// and exports P50/P95/P99 to Prometheus. Thread-safe via atomic operations.
type IPQualityV2 struct { ... }
```

## State Machine Pattern

Each state is a struct implementing the `stateMachine` interface:

```go
type stateMachine interface {
    getCurrentState() int
    getRequest()      *dns.Msg
    getResponse()     *dns.Msg
    handle(req, resp *dns.Msg) (int, error)
}
```

Constructor pattern (also provide a `WithContext` variant):

```go
func newInCacheState(req, resp *dns.Msg) *inCacheState {
    return &inCacheState{request: req, response: resp, ctx: context.Background()}
}
```

States: `STATE_INIT → IN_CACHE → CHECK_RESP → IN_GLUE → IN_GLUE_CACHE → ITER → RET_RESP`

## Testing Patterns

- Table-driven tests with `t.Run(tt.name, ...)` — see `server/ip_pool_test.go`
- Always run with `-race`; integration tests use `-timeout 120s`
- E2E tests live in `e2e/`; `e2e/main_test.go` owns `TestMain` — **do not add `init()` to individual e2e files**
- Initialize monitor singletons once in `TestMain`, never per-file:

```go
// e2e/main_test.go — already exists, do not duplicate
func TestMain(m *testing.M) {
    monitor.Rec53Log = zap.NewNop().Sugar()
    monitor.InitMetricForTest() // no Prometheus registration, no HTTP listener
    os.Exit(m.Run())
}
```

- **Do not call `FlushCacheForTest()` / `ResetIPPoolForTest()` indiscriminately** — cold cache causes
  slow iterative resolution (e.g. `www.huawei.com` takes 6-15 s). Only reset state when the test
  correctness explicitly requires a clean slate (e.g. `TestServerUDPAndTCP`, `TestMalformedQueries`).
- Mock authority servers: `NewMockAuthorityServer(t, zone)` from `e2e/helpers.go`
- Test helpers that expose internal state: `SetIterPort` / `ResetIterPort` in `server/state_define.go`

## Key Architecture Notes

- **Cache keys**: `"domain.:qtype"` — use `getCacheCopyByType` / `setCacheCopyByType`; always `msg.Copy()` on retrieval to prevent mutation of cached data
- **IP selection**: `globalIPPool.GetBestIPsV2(ips)` returns `(best, secondary)`; `RecordLatency` / `RecordFailure` update quality; probe loop runs every 30 s
- **CNAME / NS resolution depth**: context key `contextKeyNSResolutionDepth` prevents recursive deadlock in `resolveNSIPsConcurrently`
- **Max iterations**: 50 state machine loops (CNAME loop guard) — see `server/state_machine.go`

## Package Globals

| Variable | Package | Init |
|----------|---------|------|
| `globalDnsCache` | `server` | `newCache()` at package init |
| `globalIPPool` | `server` | `NewIPPool()` at package init |
| `Rec53Metric *Metric` | `monitor` | `InitMetric(addr)` or `InitMetricForTest()` |
| `Rec53Log *zap.SugaredLogger` | `monitor` | set by `cmd/rec53.go` or test `TestMain` |

## Dependencies

- `github.com/miekg/dns` — DNS protocol
- `github.com/patrickmn/go-cache` — TTL cache
- `github.com/prometheus/client_golang` — metrics
- `go.uber.org/zap` — structured logging
- `gopkg.in/natefinch/lumberjack.v2` — log rotation
- `gopkg.in/yaml.v2` — config parsing

## Document Maintenance

Update these docs **in the same commit** as the code change:

- New package/dir → `docs/architecture.md`
- New dependency → this file + `docs/architecture.md`
- New patterns → `.rec53/CONVENTIONS.md`
- User-facing changes → `README.md` + `README.zh.md`
- Benchmark docs update policy (`docs/benchmarks.md` / `docs/perf-regression.md`):
  - When the user asks to "update benchmark docs", first run relevant benchmarks/load tests/pprof commands when feasible, then update docs based on measured results.
  - If tests cannot be run (env/time/dependency limits), explicitly state what could not be executed and avoid presenting unverified numbers as fresh measurements.
- README sync policy:
  - Any change to features, behavior, config, CLI flags, examples, or operational notes in one README must be mirrored in the other (`README.md` and `README.zh.md`) in the same commit.
  - If a change is intentionally language-specific, add a short rationale in the commit/PR description; otherwise treat single-sided README edits as incomplete.

Related: `docs/architecture.md`, `.rec53/CONVENTIONS.md`, `.rec53/ROADMAP.md`

## Communication Language

- Default: Respond to the user in Simplified Chinese.
- Override: If the user explicitly requests another language (for example, English or Japanese), switch to that language.
- Priority: An explicit language request in the current user turn overrides the default rule.
