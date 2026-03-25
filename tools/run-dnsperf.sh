#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DNSPERF_BIN="$PROJECT_DIR/tools/dnsperf/dnsperf"
DNSPERF_SRC="./tools/dnsperf"
DEFAULT_QUERY_FILE="$PROJECT_DIR/tools/dnsperf/queries-sample.txt"

usage() {
    cat <<'EOF'
Usage: ./tools/run-dnsperf.sh <mode> [-- extra dnsperf flags]

Modes:
  build     Build tools/dnsperf/dnsperf and exit
  warmup    Short replay run to warm cache with sample queries
  hit       Replay sample queries for cache-hit / steady-state testing
  miss      Random-prefix mode for cache-miss / iterative testing
  tcp       Replay sample queries over TCP
  limited   Replay sample queries with a QPS limit
  custom    Build dnsperf, then pass remaining flags through unchanged

Environment overrides:
  SERVER        Target DNS server                (default: 127.0.0.1:5533)
  CONCURRENCY   Worker count                     (mode-specific default)
  DURATION      Test duration                    (mode-specific default)
  COUNT         Query count                      (unset by default)
  QPS           Rate limit                       (default: 5000 for limited)
  PROTO         udp or tcp                       (mode-specific default)
  QUERY_FILE    Replay query file                (default: tools/dnsperf/queries-sample.txt)
  RANDOM_BASE   Base domain for miss mode        (default: example.com)
  TIMEOUT       Per-query timeout                (default: dnsperf builtin)
  AUTO_BUILD    1 to rebuild before running      (default: 1)

Examples:
  ./tools/run-dnsperf.sh hit
  SERVER=127.0.0.1:53 CONCURRENCY=128 ./tools/run-dnsperf.sh hit
  ./tools/run-dnsperf.sh miss
  QPS=2000 ./tools/run-dnsperf.sh limited
  ./tools/run-dnsperf.sh custom -- -server 127.0.0.1:5533 -f tools/dnsperf/queries-sample.txt -c 64 -d 20s
EOF
}

die() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

build_dnsperf() {
    printf 'Building dnsperf -> %s\n' "$DNSPERF_BIN"
    (cd "$PROJECT_DIR" && go build -o "$DNSPERF_BIN" "$DNSPERF_SRC")
}

print_cmd() {
    printf 'Running:'
    for arg in "$@"; do
        printf ' %q' "$arg"
    done
    printf '\n'
}

mode="${1:-help}"
if [[ "$mode" == "help" || "$mode" == "-h" || "$mode" == "--help" ]]; then
    usage
    exit 0
fi
shift || true

if [[ "${1:-}" == "--" ]]; then
    shift
fi

case "$mode" in
    build|warmup|hit|miss|tcp|limited|custom)
        ;;
    *)
        usage >&2
        die "unknown mode: $mode"
        ;;
esac

if [[ "${AUTO_BUILD:-1}" != "0" ]] || [[ ! -x "$DNSPERF_BIN" ]]; then
    build_dnsperf
fi

if [[ "$mode" == "build" ]]; then
    exit 0
fi

server="${SERVER:-127.0.0.1:5533}"
query_file="${QUERY_FILE:-$DEFAULT_QUERY_FILE}"
random_base="${RANDOM_BASE:-example.com}"
count="${COUNT:-}"
qps="${QPS:-}"
timeout="${TIMEOUT:-}"

proto="udp"
concurrency=""
duration=""
mode_desc=""

case "$mode" in
    warmup)
        concurrency="${CONCURRENCY:-4}"
        duration="${DURATION:-5s}"
        mode_desc="cache warmup replay"
        ;;
    hit)
        concurrency="${CONCURRENCY:-64}"
        duration="${DURATION:-20s}"
        mode_desc="cache-hit replay"
        ;;
    miss)
        concurrency="${CONCURRENCY:-32}"
        duration="${DURATION:-20s}"
        mode_desc="cache-miss random-prefix"
        ;;
    tcp)
        proto="${PROTO:-tcp}"
        concurrency="${CONCURRENCY:-20}"
        duration="${DURATION:-20s}"
        mode_desc="tcp replay"
        ;;
    limited)
        concurrency="${CONCURRENCY:-20}"
        duration="${DURATION:-60s}"
        qps="${qps:-5000}"
        mode_desc="rate-limited replay"
        ;;
    custom)
        mode_desc="custom passthrough"
        ;;
esac

if [[ "$mode" != "custom" ]] && [[ ! -f "$query_file" ]] && [[ "$mode" != "miss" ]]; then
    die "query file not found: $query_file"
fi

if [[ -n "${PROTO:-}" ]]; then
    proto="$PROTO"
fi

cmd=("$DNSPERF_BIN")

if [[ "$mode" == "custom" ]]; then
    if [[ $# -eq 0 ]]; then
        die "custom mode requires dnsperf flags after --"
    fi
    cmd+=("$@")
else
    cmd+=(-server "$server" -proto "$proto" -c "$concurrency")
    if [[ -n "$duration" ]]; then
        cmd+=(-d "$duration")
    fi
    if [[ -n "$count" ]]; then
        cmd+=(-n "$count")
    fi
    if [[ -n "$qps" ]]; then
        cmd+=(-qps "$qps")
    fi
    if [[ -n "$timeout" ]]; then
        cmd+=(-timeout "$timeout")
    fi
    if [[ "$mode" == "miss" ]]; then
        cmd+=(-random-prefix "$random_base")
    else
        cmd+=(-f "$query_file")
    fi
    cmd+=("$@")

    printf 'Mode: %s\n' "$mode_desc"
    printf 'Server: %s\n' "$server"
    printf 'Protocol: %s\n' "$proto"
    printf 'Concurrency: %s\n' "$concurrency"
    if [[ -n "$duration" ]]; then
        printf 'Duration: %s\n' "$duration"
    fi
    if [[ -n "$count" ]]; then
        printf 'Count: %s\n' "$count"
    fi
    if [[ -n "$qps" ]]; then
        printf 'QPS limit: %s\n' "$qps"
    fi
    if [[ "$mode" == "miss" ]]; then
        printf 'Random base: %s\n' "$random_base"
    else
        printf 'Query file: %s\n' "$query_file"
    fi
fi

print_cmd "${cmd[@]}"
exec "${cmd[@]}"
