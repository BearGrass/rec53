## Context

The `rec53/e2e` package runs integration tests against a real DNS server. The `monitor` package exposes two package-level singletons used throughout server code: `Rec53Log` (a zap.SugaredLogger) and `Rec53Metric` (a Prometheus-backed metrics object). Both are declared as pointer types with `nil` zero values and must be explicitly initialized before use.

In production, `cmd/rec53.go` calls `monitor.InitMetric()` and `monitor.InitLog()` at startup. In tests, individual `init()` functions in some e2e files set `Rec53Log` to a no-op logger, but **no e2e file ever initializes `Rec53Metric`**. This causes a nil-pointer panic in `ServeDNS` and `iterState.handle` whenever a real DNS query is processed, silently killing the response and causing clients to time out.

The three failing tests all share the same call path through the server's `ServeDNS` → `Rec53Metric.InCounterAdd(...)`.

## Goals / Non-Goals

**Goals:**
- Ensure `monitor.Rec53Metric` and `monitor.Rec53Log` are always non-nil before any e2e test runs.
- Fix all three failing tests without modifying production code or test logic.
- Use a single centralized setup (e2e `TestMain`) rather than per-file `init()` duplication.

**Non-Goals:**
- Changing any production code paths.
- Adding mock/fake implementations of `Rec53Metric`; the existing in-process Prometheus registry is sufficient.
- Fixing the `TestServerUDPAndTCP` network-dependence issue (it will pass once the nil-panic is fixed if network is available; marking it with `testing.Short()` is out of scope).

## Decisions

### Decision 1: Use `TestMain` in the e2e package instead of per-file `init()`

**Chosen:** Add a single `e2e/main_test.go` with a `TestMain(m *testing.M)` function that initializes both singletons before `m.Run()`.

**Alternatives considered:**
- *Per-file `init()`*: Already partially done for `Rec53Log`; error-prone because new test files can forget to call it. `TestMain` is the canonical Go pattern for package-wide test setup.
- *Calling `monitor.InitMetricWithAddr("")`*: Would start a real HTTP listener on a random port; unnecessary overhead for tests.

**Rationale:** `TestMain` runs once per package before any test, is impossible to forget for new files in the same package, and is the idiomatic Go approach.

### Decision 2: Initialize `Rec53Metric` with a no-op/in-process registry

The existing `monitor.InitMetric()` function registers Prometheus collectors. To avoid port-binding side effects, call it with the internal constructor that registers to the default registry but doesn't bind an HTTP listener. The existing API (`monitor.InitMetric()` / `monitor.NewMetric()`) accepts this pattern.

## Risks / Trade-offs

- [Risk] Prometheus duplicate-registration panic if multiple test binaries run in the same process → Mitigation: Use `prometheus.NewRegistry()` in a test helper, or wrap `InitMetric` with a `sync.Once`. The existing code already uses `MustRegister` which panics on duplicate — check if `TestMain` is called once per binary (it is; safe).
- [Risk] `TestServerUDPAndTCP/UDP_query` still requires live internet to get `RcodeSuccess` → Mitigation: Out of scope; the nil-panic fix is the minimal change. If the test environment has no outbound DNS, the test will return `SERVFAIL` not a timeout, which is a different (acceptable) failure mode to investigate separately.
