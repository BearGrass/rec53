# Testing Docs

This directory contains rec53 test strategy, performance methodology, baseline
data, and benchmark reports.

## Core Docs

- [Recursive DNS Test Plan](recursive-dns-test-plan.md)
- [Performance Regression Rules](perf-regression.md)
- [Benchmarks](benchmarks.md)
- [dnsperf Tool Guide](../../tools/dnsperf/README.md)

## Reports

- [Physical NIC XDP Benchmark Report (Chinese, 2026-03-19)](xdp-physical-benchmark-2026-03-19.zh.md)

## Usage

- start with `recursive-dns-test-plan.md` for the full testing strategy
- use `perf-regression.md` when running benchmark/load/pprof comparisons
- use `./tools/run-dnsperf.sh hit` for the fastest local replay test
- use `./tools/run-dnsperf.sh miss` for cache-miss / iterative pressure
- update `benchmarks.md` only with measured numbers
