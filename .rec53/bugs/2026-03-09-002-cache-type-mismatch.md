# Bug Record: 2026.3.9.002

## Summary

E2E tests failing with 2 test failures out of 24 test suites.

## Failed Tests

### 1. TestTruncatedResponse

**Location:** `e2e/error_test.go:349`

**Error Output:**
```
--- FAIL: TestTruncatedResponse (0.11s)
    error_test.go:381: UDP response: rcode=SERVFAIL, truncated=false
    error_test.go:385: Expected truncated response over UDP, got non-truncated
```

---

### 2. TestResolverIntegration

**Location:** `e2e/resolver_test.go:20`

**Status:** Partial failure (3 of 6 sub-tests failed)

**Failed Sub-tests:**

#### AAAA record for google.com
```
--- FAIL: TestResolverIntegration/AAAA_record_for_google.com (0.00s)
    resolver_test.go:121: Expected AAAA record for google.com., got 0 answers
    resolver_test.go:122: Response rcode: SERVFAIL
```

#### TXT record for google.com
```
--- FAIL: TestResolverIntegration/TXT_record_for_google.com (0.00s)
    resolver_test.go:121: Expected TXT record for google.com., got 0 answers
    resolver_test.go:122: Response rcode: SERVFAIL
```

#### NS record for google.com
```
--- FAIL: TestResolverIntegration/NS_record_for_google.com (0.00s)
    resolver_test.go:121: Expected NS record for google.com., got 0 answers
    resolver_test.go:122: Response rcode: SERVFAIL
```

**Passed Sub-tests:**
- A record for google.com ✅
- A record for cloudflare.com ✅
- MX record for gmail.com ✅

---

## Root Cause Analysis

### 问题 1: 缓存键不包含查询类型

**位置:** `server/cache.go`

**问题代码:**
```go
// cache.go:17-23
func getCache(key string) (*dns.Msg, bool) {
    value, found := globalDnsCache.Get(key)
    // ...
}

// state_define.go:78
if msgInCache, ok := getCacheCopy(request.Question[0].Name); ok {
```

**分析:**
缓存键只使用域名 (`request.Question[0].Name`)，不包含查询类型。这导致：

| 查询顺序 | 查询类型 | 缓存状态 | 结果 |
|---------|---------|---------|------|
| 第1次 | A 记录 | 缓存 A 响应 | ✅ 正确 |
| 第2次 | AAAA 记录 | 命中 A 记录缓存 | ❌ 返回错误类型 |

**影响:**
- 对同一域名查询不同记录类型时，返回缓存的错误类型数据
- 状态机 `checkRespState.handle()` 检测到类型不匹配，误判为 CNAME

---

### 问题 2: checkRespState 类型判断逻辑错误

**位置:** `server/state_define.go:113-125`

**问题代码:**
```go
func (s *checkRespState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
    // ...
    if len(response.Answer) != 0 {
        if response.Answer[len(response.Answer)-1].Header().Rrtype == request.Question[0].Qtype {
            return CHECK_RESP_GET_ANS, nil
        }
        //TODO: another type
        return CHECK_RESP_GET_CNAME, nil  // ← 错误：任何类型不匹配都被当作 CNAME
    }
    return CHECK_RESP_GET_NS, nil
}
```

**分析:**
只检查最后一个 Answer 的类型是否匹配查询类型：
- 匹配 → 返回答案
- 不匹配 → **错误地假设为 CNAME**

**错误场景示例:**
```
查询: google.com AAAA
响应: [A: 142.250.71.174]  (来自缓存)
判断: A != AAAA → 当作 CNAME 处理 → 无限循环或错误
```

**根本原因:**
1. 没有检查记录类型是否真的是 CNAME
2. 没有考虑"查询类型无记录但服务器返回其他类型"的情况
3. 没有 CNAME 检测逻辑：`response.Answer[i].Header().Rrtype == dns.TypeCNAME`

---

### 问题 3: 无截断响应 (TC Flag) 处理

**位置:** `server/server.go:28-50`

**问题代码:**
```go
func (s *server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
    // ...
    result, err := Change(stm)
    if err != nil {
        reply.SetRcode(r, dns.RcodeServerFailure)
    } else {
        reply = result
    }
    // ...
    w.WriteMsg(reply)  // ← 直接写入，无截断检查
}
```

**分析:**
服务器没有实现：
1. **响应大小检查** - 没有检查 `reply.Len()` 是否超过 UDP 限制
2. **TC 标志设置** - 没有设置 `reply.Truncated = true`
3. **TCP 回退** - 没有引导客户端使用 TCP

**DNS 协议要求:**
- UDP 响应默认限制 512 字节
- 超过限制应设置 TC=1 并截断响应
- 客户端收到 TC=1 后应使用 TCP 重试

---

### 问题 4: ITER 状态上游响应处理不完整

**位置:** `server/state_define.go:264-268`

**问题代码:**
```go
if newResponse.Rcode != dns.RcodeSuccess {
    //TODO: return servfail
    s.response.Rcode = newResponse.Rcode
    return ITER_COMMEN_ERROR, fmt.Errorf("response.Rcode is not success")
}
```

**分析:**
所有非 NOERROR 响应都被转换为 SERVFAIL：
- NXDOMAIN → SERVFAIL (错误，应保留 NXDOMAIN)
- NOERROR 无数据 → SERVFAIL (错误，应返回空答案)
- REFUSED → SERVFAIL (可能正确)

---

## 数据流分析

### 正常 A 记录查询流程
```
STATE_INIT → IN_CACHE (miss) → IN_GLUE → IN_GLUE_CACHE → ITER → CHECK_RESP → RET_RESP
                ↓
         [root glue]    [上游查询]   [A 记录匹配]  [返回]
```

### AAAA 记录查询失败流程
```
STATE_INIT → IN_CACHE (hit: A record) → CHECK_RESP
                    ↓
              [返回缓存的 A 记录]
                    ↓
              [A != AAAA → CNAME 处理]
                    ↓
              [修改查询域名为 CNAME target]
                    ↓
              IN_CACHE → ... (循环或失败)
```

---

## 解决方案

### 方案 1: 缓存键包含查询类型 (推荐)

**修改文件:** `server/cache.go`, `server/state_define.go`

```go
// cache.go - 添加类型感知的缓存函数
func getCacheKey(name string, qtype uint16) string {
    return fmt.Sprintf("%s:%d", name, qtype)
}

func getCacheByType(name string, qtype uint16) (*dns.Msg, bool) {
    return getCacheCopy(getCacheKey(name, qtype))
}

// state_define.go - inCacheState.handle()
func (s *inCacheState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
    q := request.Question[0]
    if msgInCache, ok := getCacheByType(q.Name, q.Qtype); ok {
        // ...
    }
}
```

### 方案 2: 修复 checkRespState 类型判断

**修改文件:** `server/state_define.go`

```go
func (s *checkRespState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
    if request == nil || response == nil {
        return CHECK_RESP_COMMEN_ERROR, fmt.Errorf("request is nil or response is nil")
    }

    if len(response.Answer) != 0 {
        // 检查是否有匹配的记录类型
        for _, rr := range response.Answer {
            if rr.Header().Rrtype == request.Question[0].Qtype {
                return CHECK_RESP_GET_ANS, nil
            }
            // 检查是否是 CNAME
            if cname, ok := rr.(*dns.CNAME); ok {
                // 保存 CNAME target 供后续处理
                return CHECK_RESP_GET_CNAME, nil
            }
        }
        // 有答案但类型不匹配，需要继续迭代查询
        return CHECK_RESP_GET_NS, nil
    }
    return CHECK_RESP_GET_NS, nil
}
```

### 方案 3: 实现截断响应处理

**修改文件:** `server/server.go`

```go
func (s *server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
    // ... 现有逻辑 ...

    // 检查响应大小
    maxSize := 512 // 默认 UDP 限制
    if opt := r.IsEdns0(); opt != nil {
        maxSize = int(opt.UDPSize())
    }

    // UDP 连接需要检查截断
    if _, ok := w.RemoteAddr().(*net.UDPAddr); ok {
        if reply.Len() > maxSize {
            reply.Truncated = true
            // 截断 Answer 部分
            for reply.Len() > maxSize && len(reply.Answer) > 0 {
                reply.Answer = reply.Answer[:len(reply.Answer)-1]
            }
        }
    }

    w.WriteMsg(reply)
}
```

### 方案 4: 正确处理上游响应码

**修改文件:** `server/state_define.go`

```go
// iterState.handle() 中
if newResponse.Rcode != dns.RcodeSuccess {
    s.response.Rcode = newResponse.Rcode
    s.response.Ns = newResponse.Ns
    // 根据响应码决定是否返回错误
    switch newResponse.Rcode {
    case dns.RcodeNameError: // NXDOMAIN
        return ITER_NO_ERROR, nil  // 正常返回，保留 NXDOMAIN
    case dns.RcodeSuccess:
        return ITER_NO_ERROR, nil
    default:
        return ITER_COMMEN_ERROR, fmt.Errorf("response rcode: %s",
            dns.RcodeToString[newResponse.Rcode])
    }
}
```

---

## 优先级建议

| 优先级 | 问题 | 状态 | 修复方案 |
|-------|------|------|----------|
| P0 | 缓存键不包含类型 | ✅ 已修复 | 方案 1 |
| P0 | checkRespState 类型判断 | ✅ 已修复 | 方案 2 |
| P1 | 截断响应处理 | ✅ 已修复 | 方案 3 |
| P2 | 上游响应码处理 | ✅ 已修复 | 方案 4 |

---

## 测试验证

修复后应通过的测试：
```bash
go test -v ./e2e/... -run "TestResolverIntegration|TestTruncatedResponse"
```

---

## Status

- **Fixed (P0)** - 缓存键问题已修复
- **Fixed (P0)** - checkRespState 类型判断已修复
- **Fixed (P1)** - 截断响应处理已实现
- **Fixed (P2)** - 上游响应码处理已修复 ✅

## Fix Applied (2026.3.9)

### 方案 1: 缓存键包含查询类型 ✅

**server/cache.go:**
- 添加 `getCacheKey(name, qtype)` - 生成包含类型的缓存键
- 添加 `getCacheCopyByType(name, qtype)` - 按类型获取缓存
- 添加 `setCacheCopyByType(name, qtype, msg, ttl)` - 按类型存储缓存

**server/state_define.go:**
- `inCacheState.handle()` - 使用 `getCacheCopyByType()`
- `iterState.handle()` - 使用 `setCacheCopyByType()`

### 方案 2: 修复 checkRespState 类型判断 ✅

**server/state_define.go - checkRespState.handle():**

修改前的问题：
```go
// 只检查最后一个 Answer，不匹配就当作 CNAME
if response.Answer[len(response.Answer)-1].Header().Rrtype == qtype {
    return CHECK_RESP_GET_ANS, nil
}
return CHECK_RESP_GET_CNAME, nil  // 错误！
```

修改后的逻辑：
```go
// 1. 遍历所有 Answer 检查是否有匹配类型
for _, rr := range response.Answer {
    if rr.Header().Rrtype == qtype {
        return CHECK_RESP_GET_ANS, nil
    }
}

// 2. 检查是否真的有 CNAME 记录
if qtype != dns.TypeCNAME {
    for _, rr := range response.Answer {
        if cname, ok := rr.(*dns.CNAME); ok {
            return CHECK_RESP_GET_CNAME, nil
        }
    }
}

// 3. 无匹配且无 CNAME，继续迭代查询
return CHECK_RESP_GET_NS, nil
```

### 验证结果

```
=== RUN   TestResolverIntegration
--- PASS: TestResolverIntegration (1.31s)
    --- PASS: TestResolverIntegration/A_record_for_google.com (0.49s)
    --- PASS: TestResolverIntegration/A_record_for_cloudflare.com (0.42s)
    --- PASS: TestResolverIntegration/AAAA_record_for_google.com (0.06s)  ✅
    --- PASS: TestResolverIntegration/MX_record_for_gmail.com (0.13s)
    --- PASS: TestResolverIntegration/TXT_record_for_google.com (0.05s)  ✅
    --- PASS: TestResolverIntegration/NS_record_for_google.com (0.05s)  ✅

=== RUN   TestCNAMEResolution
--- PASS: TestCNAMEResolution (0.31s)
    --- PASS: TestCNAMEResolution/www.github.com. (0.00s)  ✅ CNAME 跟随正常
    --- PASS: TestCNAMEResolution/www.cloudflare.com. (0.21s)  ✅
```

---

## Remaining Issues

All issues have been fixed! ✅

### Issue 4: 上游响应码处理 (P2) - ✅ 已修复

**server/state_define.go - iterState.handle():**

修改前的问题：
```go
if newResponse.Rcode != dns.RcodeSuccess {
    s.response.Rcode = newResponse.Rcode
    return ITER_COMMEN_ERROR, fmt.Errorf("response.Rcode is not success")
}
```

修改后的逻辑：
```go
if newResponse.Rcode != dns.RcodeSuccess {
    // Copy response code and authority section
    s.response.Rcode = newResponse.Rcode
    s.response.Ns = newResponse.Ns

    // Handle different response codes appropriately
    switch newResponse.Rcode {
    case dns.RcodeNameError: // NXDOMAIN - domain does not exist
        // Return normally with NXDOMAIN code preserved
        return ITER_NO_ERROR, nil
    case dns.RcodeSuccess:
        return ITER_NO_ERROR, nil
    default:
        // Other errors (REFUSED, SERVFAIL, etc.) - return as error
        return ITER_COMMEN_ERROR, fmt.Errorf("response rcode: %s",
            dns.RcodeToString[newResponse.Rcode])
    }
}
```

**修复要点:**
- NXDOMAIN (`RcodeNameError`) 正确保留并返回，不再转换为 SERVFAIL
- 同时复制 Authority Section (`s.response.Ns`) 以保留 SOA 记录
- 其他错误码 (REFUSED, SERVFAIL 等) 仍返回错误

## Fix Applied (方案 3: 截断响应处理) ✅

**server/server.go:**

添加了三个新函数：
- `isUDP(w)` - 检查是否是 UDP 连接
- `getMaxUDPSize(r)` - 从 EDNS0 获取客户端 UDP 缓冲区大小
- `truncateResponse(reply, request, maxSize)` - 截断超过限制的响应

```go
func truncateResponse(reply, request *dns.Msg, maxSize int) *dns.Msg {
    if reply.Len() <= maxSize {
        return reply
    }
    reply.Truncated = true
    // 移除 Answer 直到符合大小限制
    for len(reply.Answer) > 0 && reply.Len() > maxSize {
        reply.Answer = reply.Answer[:len(reply.Answer)-1]
    }
    // 必要时清空 Extra 和 Ns
    if reply.Len() > maxSize {
        reply.Extra = nil
    }
    if reply.Len() > maxSize {
        reply.Answer = nil
    }
    reply.Ns = nil
    return reply
}
```

**DNS 协议规范:**
- UDP 响应默认限制 512 字节
- EDNS0 可扩展缓冲区大小
- 超过限制设置 TC=1 标志
- 客户端收到 TC=1 后应使用 TCP 重试

## Related Files

- `server/state_machine.go` - 状态机逻辑
- `server/state_define.go` - 状态定义和查询构造
- `server/server.go` - DNS 服务器请求处理
- `server/cache.go` - DNS 缓存实现