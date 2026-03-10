# Bug Record: 2026.3.10.001

## Summary

DNS response Question Section mismatch - response contains wrong domain name in question section.

## Symptom

```bash
$ dig www.qq.com -p 5354 @127.0.0.1
;; ;; Question section mismatch: got ins-r23tsuuf.ias.tencent-cloud.net/A/IN
;; communications error to 127.0.0.1#5354: timed out
```

The response Question Section shows `ins-r23tsuuf.ias.tencent-cloud.net` instead of `www.qq.com`.

## Root Cause

When following CNAME chains, the state machine modifies `request.Question[0].Name` to the CNAME target domain. The original question was only restored in the `RET_RESP` state, but there were other code paths that could return the response without going through this state:

1. `CHECK_RESP_COMMEN_ERROR` case - returns response directly
2. Error paths after request modification
3. Fallback logic in `server.go` used modified `r.Question`

### Code Flow

```
Query: www.qq.com
  ↓
STATE_INIT → IN_CACHE (miss) → IN_GLUE → ITER
  ↓
[Response: www.qq.com CNAME ins-r23tsuuf.ias.tencent-cloud.net]
  ↓
CHECK_RESP_GET_CNAME
  ↓
request.Question[0].Name = "ins-r23tsuuf.ias.tencent-cloud.net"  // Modified!
  ↓
IN_CACHE → ... → RET_RESP
  ↓
[Only restored here, but other paths exist]
```

### Problematic Code

**server/state_machine.go:99**
```go
case CHECK_RESP_GET_CNAME:
    // ...
    stm.getRequest().Question[0].Name = cnameTarget  // Modifies original request
```

**server/state_machine.go:172-175**
```go
case RET_RESP:
    // Only here was the original question restored
    resp.Question[0] = originalQuestion
    return resp, nil
```

## Fix Applied

**server/server.go**

Save original question at entry point and restore before returning:

```go
func (s *server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
    startTime := time.Now()
    reply := &dns.Msg{}

    // Save original question before any modifications by state machine
    var originalQuestion dns.Question
    if len(r.Question) > 0 {
        originalQuestion = r.Question[0]
    }

    // ... state machine execution ...

    // Restore original question to ensure response matches query
    if len(originalQuestion.Name) > 0 {
        if len(reply.Question) == 0 {
            reply.Question = make([]dns.Question, 1)
        }
        reply.Question[0] = originalQuestion
    }
    // ...
}
```

## Verification

```bash
$ ./rec53 -listen 127.0.0.1:5354 &
$ dig www.qq.com -p 5354 @127.0.0.1

; <<>> DiG 9.18.39 <<>> www.qq.com -p 5354 @127.0.0.1
;; QUESTION SECTION:
;www.qq.com.            IN  A   # Correct!

;; ANSWER SECTION:
www.qq.com.     300 IN  CNAME   ins-r23tsuuf.ias.tencent-cloud.net.
```

## Status

- **Fixed** ✅

## Related Files

- `server/server.go` - Fix location
- `server/state_machine.go` - Where question is modified during CNAME following
- `server/state_define.go` - State handlers