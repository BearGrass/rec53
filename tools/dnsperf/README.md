# dnsperf

`dnsperf` is a small DNS pressure tool bundled with this repository for validating `rec53` under cache-hit, cache-miss, and rate-limited workloads.

For day-to-day use, prefer the wrapper script [`../run-dnsperf.sh`](../run-dnsperf.sh). It rebuilds `dnsperf` automatically and provides ready-made modes such as `hit`, `miss`, `tcp`, and `limited`.

Quick examples:

```bash
./tools/run-dnsperf.sh hit
./tools/run-dnsperf.sh miss
QPS=2000 ./tools/run-dnsperf.sh limited
./tools/run-dnsperf.sh tcp
```

## Build

From the repository root:

```bash
go build -o ./tools/dnsperf/dnsperf ./tools/dnsperf
```

Then run it either from the repository root or from `tools/dnsperf/`.

Or just let the wrapper build it for you:

```bash
./tools/run-dnsperf.sh build
```

## Basic Usage

```bash
./tools/dnsperf/dnsperf -server 127.0.0.1:53 -f ./tools/dnsperf/queries-sample.txt -c 50 -n 100000
```

Common flags:

- `-server`: target DNS server, default `127.0.0.1:53`
- `-proto`: `udp` or `tcp`, default `udp`
- `-c`: worker concurrency, default `10`
- `-n`: total number of queries; if `-d` is not set and `-n=0`, it falls back to `10000`
- `-d`: duration mode such as `30s`; when set, it overrides `-n`
- `-qps`: rate limit in queries per second; `0` means unlimited
- `-timeout`: per-query timeout, default `5s`
- `-f`: query file, one line per `name [qtype]`
- `-random-prefix`: generate random subdomains under one base domain, used for cache-miss / iterative tests

## Query File Format

Use `-f` to provide a query list. Format:

```text
name [qtype]
```

Rules:

- empty lines and lines starting with `#` are ignored
- `qtype` is optional and defaults to `A`
- names are automatically normalized to FQDN internally

Example:

```text
www.baidu.com
www.github.com A
cloudflare.com AAAA
gmail.com MX
example.com TXT
```

Sample file: [`queries-sample.txt`](queries-sample.txt)

## Common Scenarios

Cache-hit pressure test:

```bash
./tools/dnsperf/dnsperf \
  -server 127.0.0.1:5353 \
  -f ./tools/dnsperf/queries-sample.txt \
  -c 50 \
  -n 100000
```

This reuses queries from the file in a loop, which is suitable for observing cache-hit ratio and stable latency.

Cache-miss / iterative pressure test:

```bash
./tools/dnsperf/dnsperf \
  -server 127.0.0.1:5353 \
  -random-prefix example.com \
  -c 10 \
  -d 30s
```

This generates queries like `<random>.example.com`, making cache reuse unlikely and stressing iterative resolution.

The same scenario through the wrapper:

```bash
./tools/run-dnsperf.sh miss
```

Rate-limited pressure test:

```bash
./tools/dnsperf/dnsperf \
  -server 127.0.0.1:5353 \
  -f ./tools/dnsperf/queries-sample.txt \
  -c 20 \
  -qps 5000 \
  -d 60s
```

TCP pressure test:

```bash
./tools/dnsperf/dnsperf \
  -server 127.0.0.1:5353 \
  -proto tcp \
  -f ./tools/dnsperf/queries-sample.txt \
  -c 20 \
  -d 30s
```

Wrapper equivalent:

```bash
./tools/run-dnsperf.sh tcp
```

## Output

The tool prints:

- a startup summary with target, protocol, concurrency, and workload mode
- a progress line every 5 seconds with current QPS and `p50` / `p95` / `p99`
- a final summary with total queries, duration, average QPS, timeout count, error count, latency percentiles, and response-code distribution

Example progress line:

```text
[   5s] sent=25000    qps=5000     p50=1.2ms   p95=4.8ms   p99=9.7ms   err=0
```

## Notes

- `-f` and `-random-prefix` are mutually exclusive.
- `-random-prefix` currently sends `A` queries for the given base domain.
- `Ctrl+C` stops dispatching new queries and waits for in-flight requests to finish.
- For duration-based tests, prefer combining `-d` with `-qps` so the target load is explicit.
- When testing `rec53`, make sure the `-server` address matches your configured listen address, for example `127.0.0.1:5353`.
