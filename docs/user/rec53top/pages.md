# rec53top Pages And Fields

This page is both the field reference and the reading guide for the newer `rec53top` pages.

## Read This First

Do not try to read every field in order.

The practical reading order is:

1. Find the suspicious panel in overview.
2. Open that panel and read `Summary` first.
3. Read the conclusion sentence before the raw numbers.
4. Open subviews only when you need a narrower answer.
5. Treat `State Machine` as an aggregate counters view, not a full resolver call graph.

## Overview Page

The overview page is the first screen. It answers one question: which area looks most suspicious right now.

### Traffic

- `QPS`: request rate over the current window.
- `p99`: tail latency for the current window.
- `response mix`: ratio of response codes.

### Cache

- `hit ratio`: share of cache lookups that hit.
- `positive hit rate`: successful answer reuse.
- `negative hit rate`: cached NXDOMAIN/NODATA reuse.
- `miss rate`: lookups that had to continue resolution.
- `entry count`: current cache size.
- `lifecycle`: writes, flushes, and expired deletions.

### Snapshot

- `load success`: whether startup restore worked.
- `saved/imported/skipped`: what happened to snapshot entries.
- `duration`: how long load or save took.

### Upstream

- `timeout rate`: upstream calls that timed out.
- `bad rcode rate`: SERVFAIL or REFUSED style failures.
- `fallback`: whether a fallback upstream succeeded.
- `winner path`: which upstream path won the race.

### XDP

- `status`: active, disabled, or unavailable.
- `hit ratio`: cache hit share in XDP.
- `sync errors`: Go-to-BPF synchronization failures.
- `cleanup`: expired entries removed by the periodic cleaner.
- `entries`: current active XDP entries.

### State Machine

- `top stage`: the most frequently entered stage.
- `top terminal`: the terminal exit currently growing fastest.
- `failure reasons`: the dominant bounded failure categories.
- `top stage` answers "what is hottest right now", not "what full path did requests take".
- if you need one real request path, use `./dist/rec53 --config ./config.yaml --trace-domain example.com --trace-type A`.

## Detail Pages

### Summary

The summary area gives the current verdict first. It should say what matters now before showing supporting numbers.

### Breakdown Views

`Cache`, `Upstream`, and `XDP` may show subviews such as mix, failures, winners, or cleanup details. These subviews narrow the question from “what is wrong” to “which bounded category is driving it”.

### State Machine Detail

`State Machine` stays summary-only on purpose:

- `Stage mix` shows where aggregate resolver work is concentrating.
- `Terminal exits` shows how sampled flows are ending.
- `Failure reasons` shows whether one bounded failure bucket is clustering.

Read `State Machine` in this order:

- start with `top stage`
- check whether `top terminal` is still `success_exit`
- only leave the TUI when you need one real request path

For exact request-scoped flow, run:

```bash
./dist/rec53 --config ./config.yaml --trace-domain example.com --trace-type A
```

### State Labels

- `OK`: normal and readable
- `DEGRADED`: data exists, but the signal is suspicious
- `DISABLED`: intentionally off
- `UNAVAILABLE`: metric family missing
- `STALE`: the last scrape failed
- `DISCONNECTED`: no successful scrape yet
- `WARMING`: only one good sample exists so far

## Reading Rule

Read the current verdict first, then the supporting counters, then the next-check hint. Do not treat the overview card as a complete diagnosis.
