package monitor

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"
)

var (
	dnsInFlight         atomic.Int64
	metricInFlight      atomic.Int64
	totalDNSRequests    atomic.Uint64
	totalMetricScrapes  atomic.Uint64
	lastDNSStartUnix    atomic.Int64
	lastDNSDoneUnix     atomic.Int64
	lastMetricStartUnix atomic.Int64
	lastMetricDoneUnix  atomic.Int64
)

const slowHandlerThreshold = 500 * time.Millisecond

// DNSRequestStarted records request lifecycle data and returns a closure that
// logs slow/error completions while maintaining in-flight counters.
func DNSRequestStarted(name, qtype string) func(rcode string, writeErr error) {
	start := time.Now()
	now := start.UnixNano()
	lastDNSStartUnix.Store(now)
	totalDNSRequests.Add(1)
	inFlight := dnsInFlight.Add(1)
	var once sync.Once
	if inFlight >= 100 {
		Rec53Log.Warnf("[DIAG] high DNS inflight=%d name=%s type=%s", inFlight, name, qtype)
	}

	return func(rcode string, writeErr error) {
		once.Do(func() {
			lastDNSDoneUnix.Store(time.Now().UnixNano())
			remaining := dnsInFlight.Add(-1)
			elapsed := time.Since(start)
			if elapsed >= slowHandlerThreshold || writeErr != nil {
				Rec53Log.Warnf("[DIAG] dns request finished name=%s type=%s rcode=%s elapsed=%s inflight=%d err=%v",
					name, qtype, rcode, elapsed, remaining, writeErr)
			}
		})
	}
}

func instrumentMetricsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lastMetricStartUnix.Store(start.UnixNano())
		totalMetricScrapes.Add(1)
		inFlight := metricInFlight.Add(1)
		if inFlight >= 20 {
			Rec53Log.Warnf("[DIAG] high metric inflight=%d path=%s remote=%s", inFlight, r.URL.Path, r.RemoteAddr)
		}

		next.ServeHTTP(w, r)

		lastMetricDoneUnix.Store(time.Now().UnixNano())
		remaining := metricInFlight.Add(-1)
		elapsed := time.Since(start)
		if elapsed >= slowHandlerThreshold {
			Rec53Log.Warnf("[DIAG] metric scrape finished path=%s remote=%s elapsed=%s inflight=%d",
				r.URL.Path, r.RemoteAddr, elapsed, remaining)
		}
	})
}

// DumpRuntimeDiagnostics writes a runtime summary and full goroutine dump to the
// configured log sink. Intended for on-demand diagnosis of hangs or stalls.
func DumpRuntimeDiagnostics(reason string) {
	var (
		buf bytes.Buffer
		ms  runtime.MemStats
		now = time.Now()
	)

	runtime.ReadMemStats(&ms)

	fmt.Fprintf(&buf, "\n===== rec53 runtime diagnostics =====\n")
	fmt.Fprintf(&buf, "time=%s reason=%s\n", now.Format(time.RFC3339Nano), reason)
	fmt.Fprintf(&buf, "goroutines=%d dns_inflight=%d metric_inflight=%d dns_total=%d metric_total=%d\n",
		runtime.NumGoroutine(),
		dnsInFlight.Load(),
		metricInFlight.Load(),
		totalDNSRequests.Load(),
		totalMetricScrapes.Load(),
	)
	fmt.Fprintf(&buf, "last_dns_start=%s last_dns_done=%s last_metric_start=%s last_metric_done=%s\n",
		formatUnixNano(lastDNSStartUnix.Load()),
		formatUnixNano(lastDNSDoneUnix.Load()),
		formatUnixNano(lastMetricStartUnix.Load()),
		formatUnixNano(lastMetricDoneUnix.Load()),
	)
	fmt.Fprintf(&buf, "heap_alloc=%d heap_inuse=%d heap_objects=%d gc_cycles=%d\n",
		ms.HeapAlloc, ms.HeapInuse, ms.HeapObjects, ms.NumGC)
	fmt.Fprintf(&buf, "----- goroutine dump -----\n")
	if err := pprof.Lookup("goroutine").WriteTo(&buf, 2); err != nil {
		fmt.Fprintf(&buf, "failed to dump goroutines: %v\n", err)
	}
	fmt.Fprintf(&buf, "===== end runtime diagnostics =====\n")

	writer := getLogWriter()
	if _, err := writer.Write(buf.Bytes()); err != nil && Rec53Log != nil {
		Rec53Log.Errorf("[DIAG] write diagnostics failed: %v", err)
	}
	_ = writer.Sync()
}

func formatUnixNano(ts int64) string {
	if ts == 0 {
		return "n/a"
	}
	return time.Unix(0, ts).Format(time.RFC3339Nano)
}
