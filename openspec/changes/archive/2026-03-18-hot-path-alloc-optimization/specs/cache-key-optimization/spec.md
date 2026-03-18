## ADDED Requirements

### Requirement: getCacheKey uses string concatenation instead of fmt.Sprintf

`getCacheKey` SHALL use string concatenation with `strconv.FormatUint` instead of
`fmt.Sprintf` to generate cache keys, eliminating the `fmt` reflection overhead
on every cache lookup.

#### Scenario: Output format is identical
- **WHEN** `getCacheKey("example.com.", 1)` is called
- **THEN** the result SHALL be `"example.com.:1"`, identical to the previous `fmt.Sprintf` output

#### Scenario: All uint16 qtype values produce correct keys
- **WHEN** `getCacheKey` is called with qtype values 1 (A), 28 (AAAA), 5 (CNAME), 255 (ANY)
- **THEN** each result matches the format `"<name>:<decimal-qtype>"` exactly as `fmt.Sprintf("%s:%d", name, qtype)` would produce

### Requirement: getCacheKey does not import fmt

After the change, `server/cache.go` SHALL NOT import `"fmt"` solely for `getCacheKey`.
If `fmt` is still used by other functions in the file it MAY remain, but `getCacheKey`
itself SHALL NOT call any `fmt` function.

#### Scenario: No fmt.Sprintf in getCacheKey
- **WHEN** the source of `getCacheKey` in `server/cache.go` is inspected
- **THEN** it contains no call to `fmt.Sprintf` or any other `fmt` function

### Requirement: Benchmark validates no regression

A benchmark (`BenchmarkCacheKey` or equivalent) SHALL record before/after `allocs/op`
and `ns/op` for `getCacheKey`. The new implementation MUST NOT regress on `allocs/op`
(must be less than or equal to the `fmt.Sprintf` baseline). For `ns/op`, minor
fluctuations within normal microbenchmark noise are acceptable; only a statistically
significant regression (consistently >10% slower across multiple runs) constitutes a
failure. Both before and after numbers SHALL be reported in `docs/benchmarks.md`.

#### Scenario: Benchmark shows no regression
- **WHEN** `go test -bench BenchmarkCacheKey -benchmem -count=5 ./server/...` is run
- **THEN** the reported `allocs/op` SHALL be less than or equal to the `fmt.Sprintf` baseline, `ns/op` SHALL not show a statistically significant regression, and both before/after numbers are recorded
