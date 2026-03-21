# rec53top Pages And Fields

This page is the product reference for each screen and field in `rec53top`.

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
- `failure reasons`: the dominant terminal failure categories.

## Detail Pages

### Summary

The summary area gives the current verdict first. It should say what is most important now before showing supporting numbers.

### Breakdown Views

`Cache`, `Upstream`, and `XDP` may show subviews such as mix, failures, winners, paths, or cleanup details. These subviews narrow the question from “what is wrong” to “which bounded category is driving it”.

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
