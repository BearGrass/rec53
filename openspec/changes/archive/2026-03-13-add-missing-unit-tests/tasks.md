## 1. Helper function tests (state_query_upstream_test.go)

- [x] 1.1 Add `TestGetNSNamesFromResponse` — multiple NS records, empty Ns, mixed types
- [x] 1.2 Add `TestResolveNSIPs` — cache hit, cache miss, partial hit
- [x] 1.3 Add `TestUpdateNSIPsCache` — single result cached, multiple results cached, verify via getCacheCopyByType

## 2. classifyRespState branch tests (state_classify_resp_test.go)

- [x] 2.1 Create `server/state_classify_resp_test.go`
- [x] 2.2 Add `TestClassifyRespState_NilInput` — nil request and nil response cases
- [x] 2.3 Add `TestClassifyRespState_NXDOMAIN` — empty answer + SOA + RcodeNameError → GET_NEGATIVE
- [x] 2.4 Add `TestClassifyRespState_NODATA` — empty answer + SOA + RcodeSuccess → GET_NEGATIVE
- [x] 2.5 Add `TestClassifyRespState_NoAnswerNoSOA` — empty answer, no SOA → GET_NS
- [x] 2.6 Add `TestClassifyRespState_MatchingType` — answer matches qtype → GET_ANS
- [x] 2.7 Add `TestClassifyRespState_CNAME_followed` — answer has CNAME, qtype=A → GET_CNAME
- [x] 2.8 Add `TestClassifyRespState_CNAME_is_answer` — answer has CNAME, qtype=CNAME → GET_ANS
- [x] 2.9 Add `TestClassifyRespState_WrongTypeNoMatch` — answers present but wrong type, no CNAME → GET_NS

## 3. queryUpstreamState.handle branch tests (state_query_upstream_test.go)

- [x] 3.1 Add `TestQueryUpstreamState_CancelledContext` — pre-cancelled ctx → COMMON_ERROR immediately
- [x] 3.2 Add `TestQueryUpstreamState_PrimaryFailoverToSecondary` — primary mock fails, secondary succeeds → NO_ERROR
- [x] 3.3 Add `TestQueryUpstreamState_BothIPsFail` — both mock servers fail → COMMON_ERROR
- [x] 3.4 Add `TestQueryUpstreamState_BadRcodeServfail_SecondarySucceeds` — primary returns SERVFAIL, secondary succeeds → NO_ERROR
- [x] 3.5 Add `TestQueryUpstreamState_BadRcodeRefused_NoSecondary` — primary returns REFUSED, no secondary → COMMON_ERROR
- [x] 3.6 Add `TestQueryUpstreamState_QuestionMismatch` — response question name differs → COMMON_ERROR
- [x] 3.7 Add `TestQueryUpstreamState_EmptyQuestionSection` — response has no question → COMMON_ERROR
- [x] 3.8 Add `TestQueryUpstreamState_NXDOMAIN` — upstream returns NXDOMAIN → NO_ERROR

## 4. Verification

- [x] 4.1 Run `go test -race ./server/...` — all tests pass
- [x] 4.2 Run `go test -cover ./server/...` — coverage ≥ 82%
