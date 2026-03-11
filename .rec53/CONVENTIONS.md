# Coding Conventions

## Language Style

Go 1.21+ with standard Go formatting. Run `gofmt -w .` before commits.

## Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Packages | lowercase, single word | `server`, `monitor`, `utils` |
| Types/Structs | PascalCase | `IPPool`, `stateMachine`, `checkRespState` |
| Functions/Methods | PascalCase (exported), camelCase (private) | `NewServer`, `getBestIPs` |
| Constants | SCREAMING_SNAKE_CASE | `STATE_INIT`, `MAX_IP_LATENCY` |
| Interfaces | -er suffix | `stateMachine` |

## State Machine Pattern

Each state is a struct implementing the `stateMachine` interface:

```go
type stateMachine interface {
    getCurrentState() int
    getRequest() *dns.Msg
    getResponse() *dns.Msg
    handle(request *dns.Msg, response *dns.Msg) (int, error)
}
```

Constructor pattern:

```go
func newInCacheState(req, resp *dns.Msg) *inCacheState {
    return &inCacheState{
        request:  req,
        response: resp,
    }
}
```

## Error Handling

States return integer codes for flow control, not errors:

```go
// Return codes defined in state_define.go
const (
    IN_CACHE_HIT_CACHE  = 0
    IN_CACHE_MISS_CACHE = 1
    IN_CACHE_COMMON_ERROR = -1
)

// Handle method pattern
func (s *inCacheState) handle(request, response *dns.Msg) (int, error) {
    if request == nil || response == nil {
        return IN_CACHE_COMMON_ERROR, fmt.Errorf("request is nil or response is nil")
    }
    // ... logic
    return IN_CACHE_HIT_CACHE, nil
}
```

## Global Instances

Package-level globals for shared state:

```go
// server/cache.go
var globalDnsCache = newCache()

// server/ip_pool.go
var globalIPPool = NewIPPool()

// monitor/var.go
var Rec53Metric *Metric
var Rec53Log *zap.SugaredLogger
```

## Logging

Use the global `Rec53Log` with level methods:

```go
monitor.Rec53Log.Debugf("try to get cache %s (type: %s)", q.Name, dns.TypeToString[q.Qtype])
monitor.Rec53Log.Errorf("Handle state error %d %v", stm.getCurrentState(), err)
monitor.Rec53Log.Infof("rec53 started, listening on %s", *listenAddr)
```

## Testing Patterns

Table-driven tests are preferred:

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

Test helpers in `e2e/helpers.go` provide mock DNS servers:

```go
func TestWithMockServer(t *testing.T) {
    zone := Zone{
        Name: "example.com.",
        Records: []dns.RR{
            A("example.com.", "192.0.2.1"),
        },
    }
    server := NewMockAuthorityServer(zone)
    defer server.Shutdown()
    // ...
}
```

## Code Review Checklist

- [ ] Error messages are descriptive with context
- [ ] Logs include relevant query/domain information
- [ ] Tests cover edge cases and error paths
- [ ] State handlers return appropriate codes
- [ ] Cache operations use copy functions to prevent mutation
- [ ] Graceful shutdown context is properly propagated