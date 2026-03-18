#!/usr/bin/env bash
# ============================================================================
# v0.5.0 Hot-Path Allocation Optimization — Group 6 Validation Script
# ============================================================================
#
# This script runs the dual-metric acceptance gate for v0.5.0:
#   (a) dnsperf QPS/P99 must not regress vs v0.4.1 baseline (~97K QPS at c=64)
#   (b) pprof alloc_space for metrics path must show measurable reduction vs ~24%
#
# Prerequisites:
#   - rec53 binary built from v0.5.0 code: go build -o rec53 ./cmd
#   - dnsperf tool built: go build -o tools/dnsperf/dnsperf ./tools/dnsperf
#   - Network access to root DNS servers (for cache warmup)
#   - No other process on port 5353 or 6060
#
# Usage:
#   chmod +x tools/validate-v050.sh
#   ./tools/validate-v050.sh
#
# The script will:
#   1. Build rec53 and dnsperf
#   2. Start rec53 with pprof enabled
#   3. Warm the cache
#   4. Run dnsperf 3 times (c=64, 20s each)
#   5. Capture pprof alloc_space during a 4th load run
#   6. Stop rec53
#   7. Print results summary + pass/fail gate check
#
# After running, paste the output back so we can update docs/benchmarks.md
# and tasks.md.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="/tmp/rec53-v050-validation"
DNSPERF="$PROJECT_DIR/tools/dnsperf/dnsperf"
REC53="$PROJECT_DIR/rec53"
QUERIES="$PROJECT_DIR/tools/dnsperf/queries-sample.txt"
SERVER="127.0.0.1:5353"
PPROF_ADDR="127.0.0.1:6060"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

info()  { echo -e "${BOLD}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
ok()    { echo -e "${GREEN}[PASS]${NC} $*"; }
fail()  { echo -e "${RED}[FAIL]${NC} $*"; }

cleanup() {
    info "Cleaning up..."
    if [[ -n "${REC53_PID:-}" ]] && kill -0 "$REC53_PID" 2>/dev/null; then
        kill "$REC53_PID" 2>/dev/null || true
        wait "$REC53_PID" 2>/dev/null || true
        info "rec53 stopped (PID $REC53_PID)"
    fi
}
trap cleanup EXIT

# ── Step 0: Preparation ─────────────────────────────────────────────────────

info "Creating results directory: $RESULTS_DIR"
rm -rf "$RESULTS_DIR"
mkdir -p "$RESULTS_DIR"

# ── Step 1: Build ────────────────────────────────────────────────────────────

info "Building rec53..."
(cd "$PROJECT_DIR" && go build -o rec53 ./cmd)
info "Building dnsperf..."
(cd "$PROJECT_DIR" && go build -o tools/dnsperf/dnsperf ./tools/dnsperf)

# ── Step 2: Generate perf config ─────────────────────────────────────────────

PERF_CONFIG="$RESULTS_DIR/perf-config.yaml"
cat > "$PERF_CONFIG" <<'YAML'
dns:
  listen: "127.0.0.1:5353"
  metric: ":9099"
  log_level: "error"
  listeners: 0

warmup:
  enabled: true
  timeout: 10s
  duration: 8s
  concurrency: 0
  tlds:
    - com
    - cn
    - net
    - org
    - de

snapshot:
  enabled: false

debug:
  pprof_enabled: true
  pprof_listen: "127.0.0.1:6060"
YAML
info "Perf config written to $PERF_CONFIG"

# ── Step 3: Start rec53 ─────────────────────────────────────────────────────

info "Starting rec53 with pprof enabled..."
"$REC53" --config "$PERF_CONFIG" &
REC53_PID=$!
info "rec53 started (PID $REC53_PID)"

# Wait for server to be ready (warmup takes a few seconds)
info "Waiting for warmup to complete (up to 30s)..."
for i in $(seq 1 30); do
    if dig +short +timeout=1 +tries=1 @127.0.0.1 -p 5353 www.baidu.com A >/dev/null 2>&1; then
        info "Server is ready after ${i}s"
        break
    fi
    if [[ $i -eq 30 ]]; then
        fail "Server did not become ready within 30s"
        exit 1
    fi
    sleep 1
done

# Extra warmup pass to fill cache with dnsperf domains
info "Running dnsperf warmup pass (c=4, n=200)..."
"$DNSPERF" -server "$SERVER" -f "$QUERIES" -c 4 -n 200 -proto udp > "$RESULTS_DIR/warmup.txt" 2>&1 || true
sleep 2
info "Cache warmup complete"

# ── Step 4: dnsperf 3-run gate (c=64, 20s) ──────────────────────────────────

info "═══════════════════════════════════════════════════════"
info "  Task 6.1: dnsperf QPS/P99 gate (3 runs, c=64, 20s)"
info "═══════════════════════════════════════════════════════"

for run in 1 2 3; do
    info "Run $run/3..."
    "$DNSPERF" -server "$SERVER" -f "$QUERIES" -c 64 -d 20s -proto udp \
        > "$RESULTS_DIR/dnsperf-run${run}.txt" 2>&1
    echo ""
    echo "--- dnsperf run $run output ---"
    cat "$RESULTS_DIR/dnsperf-run${run}.txt"
    echo "--- end run $run ---"
    echo ""
    # Brief pause between runs
    sleep 3
done

# ── Step 5: pprof alloc_space during load ────────────────────────────────────

info "═══════════════════════════════════════════════════════"
info "  Task 6.2: pprof alloc_space capture during load"
info "═══════════════════════════════════════════════════════"

# Start a background dnsperf run for 30s to generate load during pprof capture
info "Starting background load (c=64, 30s) for pprof capture..."
"$DNSPERF" -server "$SERVER" -f "$QUERIES" -c 64 -d 30s -proto udp \
    > "$RESULTS_DIR/dnsperf-pprof-load.txt" 2>&1 &
LOAD_PID=$!

# Wait a moment for load to ramp up
sleep 3

# Capture heap profile (alloc_space)
info "Capturing heap profile (alloc_space)..."
curl -sS "http://$PPROF_ADDR/debug/pprof/heap" -o "$RESULTS_DIR/heap.pb.gz" 2>&1

# Run pprof -top with denoised focus (same filters as docs/benchmarks.md)
info ""
info "┌──────────────────────────────────────────────────────────┐"
info "│  pprof alloc_space (denoised: rec53/server + miekg/dns) │"
info "└──────────────────────────────────────────────────────────┘"
info ""

go tool pprof -top -sample_index=alloc_space \
    -focus='rec53/server|rec53/monitor|github.com/miekg/dns' \
    -ignore='runtime/pprof|compress/flate|net/http/pprof' \
    "$RESULTS_DIR/heap.pb.gz" 2>&1 | tee "$RESULTS_DIR/pprof-alloc-top.txt"

echo ""
info "Full pprof output saved to $RESULTS_DIR/pprof-alloc-top.txt"
info "Interactive pprof: go tool pprof $RESULTS_DIR/heap.pb.gz"

# Wait for background load to finish
info "Waiting for background load to finish..."
wait "$LOAD_PID" 2>/dev/null || true

# ── Step 6: Summary ─────────────────────────────────────────────────────────

echo ""
info "═══════════════════════════════════════════════════════"
info "  RESULTS SUMMARY"
info "═══════════════════════════════════════════════════════"
echo ""

echo "──── dnsperf runs (c=64, 20s) ────"
echo ""
printf "%-5s  %-12s  %-10s  %-10s  %-10s  %-8s  %-8s\n" \
    "Run" "QPS" "P50" "P95" "P99" "Errors" "Timeouts"
printf "%-5s  %-12s  %-10s  %-10s  %-10s  %-8s  %-8s\n" \
    "---" "---" "---" "---" "---" "---" "---"

for run in 1 2 3; do
    f="$RESULTS_DIR/dnsperf-run${run}.txt"
    # Parse dnsperf output (adjust patterns if dnsperf output format differs)
    qps=$(grep -oP 'QPS:\s+\K[\d.]+' "$f" 2>/dev/null | head -1 || echo "N/A")
    p50=$(grep -oP 'P50\s+\K[\S]+' "$f" 2>/dev/null | head -1 || echo "N/A")
    p95=$(grep -oP 'P95\s+\K[\S]+' "$f" 2>/dev/null | head -1 || echo "N/A")
    p99=$(grep -oP 'P99\s+\K[\S]+' "$f" 2>/dev/null | tail -1 || echo "N/A")
    errors=$(grep -oP 'Errors:\s+\K[\d]+' "$f" 2>/dev/null | head -1 || echo "N/A")
    timeouts=$(grep -oP 'Timeouts:\s+\K[\d]+' "$f" 2>/dev/null | head -1 || echo "N/A")
    printf "%-5s  %-12s  %-10s  %-10s  %-10s  %-8s  %-8s\n" \
        "$run" "$qps" "$p50" "$p95" "$p99" "$errors" "$timeouts"
done

echo ""
echo "──── v0.4.1 baseline (for comparison) ────"
echo "  QPS: ~97K (median c=64)   P99: ~2.4ms"
echo "  alloc_space (metrics path): ~24%"
echo ""
echo "──── Gate check ────"
echo ""
echo "  [GATE 1] dnsperf QPS must not regress vs ~97K baseline"
echo "           → Compare median QPS from the 3 runs above"
echo ""
echo "  [GATE 2] pprof alloc_space for metrics path must show"
echo "           measurable reduction vs ~24% baseline"
echo "           → Check InCounterAdd/OutCounterAdd/LatencyHistogramObserve"
echo "             in pprof output above. If these functions no longer appear"
echo "             in the top entries, or show significantly reduced %, PASS."
echo ""
echo "──── Files for further analysis ────"
echo ""
echo "  dnsperf results:  $RESULTS_DIR/dnsperf-run{1,2,3}.txt"
echo "  pprof heap:       $RESULTS_DIR/heap.pb.gz"
echo "  pprof top:        $RESULTS_DIR/pprof-alloc-top.txt"
echo ""
echo "══════════════════════════════════════════════════════════"
echo ""
info "Done! Paste the output above back to the agent so we can:"
info "  - Update docs/benchmarks.md with the dual-metric table (Task 6.3)"
info "  - Evaluate cache COW follow-up (Task 6.4)"
info "  - Mark Group 6 complete and commit v0.5.0"
