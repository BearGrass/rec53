// dnsperf — DNS performance testing tool for rec53.
//
// Usage:
//
//	go build -o dnsperf ./tools/dnsperf
//
//	# Cache-hit stress test (queries cycle from file)
//	./dnsperf -server 127.0.0.1:53 -f queries.txt -c 50 -n 100000
//
//	# Cache-miss iterative stress test (every query unique)
//	./dnsperf -server 127.0.0.1:53 -random-prefix example.com -c 10 -d 30s
//
//	# Rate-limited test
//	./dnsperf -server 127.0.0.1:53 -f queries.txt -c 20 -qps 5000 -d 60s
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// ── flags ──────────────────────────────────────────────────────

var (
	flagServer       = flag.String("server", "127.0.0.1:53", "target DNS server address")
	flagConcurrency  = flag.Int("c", 10, "number of concurrent workers")
	flagCount        = flag.Int("n", 0, "total queries (default 10000; set 0 with -d for duration mode)")
	flagDuration     = flag.Duration("d", 0, "test duration (overrides -n when set)")
	flagQPS          = flag.Int("qps", 0, "rate limit in queries/sec (0 = unlimited)")
	flagFile         = flag.String("f", "", "query file, one 'name [qtype]' per line")
	flagRandomPrefix = flag.String("random-prefix", "", "generate random subdomain queries for this base domain")
	flagProto        = flag.String("proto", "udp", "protocol: udp or tcp")
	flagTimeout      = flag.Duration("timeout", 5*time.Second, "per-query timeout")
)

// ── types ──────────────────────────────────────────────────────

type query struct {
	name  string
	qtype uint16
}

type result struct {
	latency time.Duration
	rcode   int
	timeout bool
	err     bool
}

// ── query loading ──────────────────────────────────────────────

// loadQueriesFromFile reads a query file with one "name [qtype]" per line.
// Lines starting with # and empty lines are skipped. qtype defaults to A.
func loadQueriesFromFile(path string) ([]query, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var qs []query
	lineNo := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		fields := strings.Fields(line)
		name := dns.Fqdn(fields[0])
		qtype := dns.TypeA
		if len(fields) >= 2 {
			t, ok := dns.StringToType[strings.ToUpper(fields[1])]
			if !ok {
				return nil, fmt.Errorf("line %d: unknown query type %q", lineNo, fields[1])
			}
			qtype = t
		}
		qs = append(qs, query{name: name, qtype: qtype})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(qs) == 0 {
		return nil, fmt.Errorf("no valid queries found in %s", path)
	}
	return qs, nil
}

// ── worker ─────────────────────────────────────────────────────

// worker sends DNS queries over a persistent connection, eliminating per-query
// socket creation overhead. If the connection breaks, it reconnects automatically.
func worker(server, proto string, timeout time.Duration,
	in <-chan query, out chan<- result, wg *sync.WaitGroup) {
	defer wg.Done()

	dial := func() *dns.Conn {
		conn, err := dns.DialTimeout(proto, server, timeout)
		if err != nil {
			return nil
		}
		return conn
	}

	conn := dial()
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	for q := range in {
		msg := new(dns.Msg)
		msg.SetQuestion(q.name, q.qtype)
		msg.RecursionDesired = true

		t0 := time.Now()

		var resp *dns.Msg
		var err error

		// Try on persistent connection; reconnect once on failure.
		for attempt := 0; attempt < 2; attempt++ {
			if conn == nil {
				conn = dial()
				if conn == nil {
					err = fmt.Errorf("dial failed")
					break
				}
			}
			conn.SetDeadline(t0.Add(timeout))
			if err = conn.WriteMsg(msg); err != nil {
				conn.Close()
				conn = nil
				continue
			}
			resp, err = conn.ReadMsg()
			if err != nil {
				conn.Close()
				conn = nil
				continue
			}
			break
		}

		lat := time.Since(t0)
		r := result{latency: lat}
		switch {
		case err != nil && isTimeout(err):
			r.timeout = true
		case err != nil:
			r.err = true
		case resp != nil:
			r.rcode = resp.Rcode
		}
		out <- r
	}
}

func isTimeout(err error) bool {
	s := err.Error()
	return strings.Contains(s, "timeout") || strings.Contains(s, "deadline")
}

// ── stats collector ────────────────────────────────────────────

type collector struct {
	mu       sync.Mutex
	lats     []time.Duration
	rcodes   map[int]int64
	timeouts int64
	errors   int64
	total    int64
}

func newCollector(estimateCap int) *collector {
	if estimateCap <= 0 {
		estimateCap = 10000
	}
	return &collector{
		lats:   make([]time.Duration, 0, estimateCap),
		rcodes: make(map[int]int64),
	}
}

func (c *collector) record(r result) {
	c.mu.Lock()
	c.total++
	c.lats = append(c.lats, r.latency)
	switch {
	case r.timeout:
		c.timeouts++
	case r.err:
		c.errors++
	default:
		c.rcodes[r.rcode]++
	}
	c.mu.Unlock()
}

type statsSnap struct {
	total    int64
	timeouts int64
	errors   int64
	rcodes   map[int]int64
	sorted   []time.Duration
}

func (c *collector) snapshot() statsSnap {
	c.mu.Lock()
	s := statsSnap{
		total:    c.total,
		timeouts: c.timeouts,
		errors:   c.errors,
		rcodes:   make(map[int]int64, len(c.rcodes)),
		sorted:   make([]time.Duration, len(c.lats)),
	}
	copy(s.sorted, c.lats)
	for k, v := range c.rcodes {
		s.rcodes[k] = v
	}
	c.mu.Unlock()
	sort.Slice(s.sorted, func(i, j int) bool { return s.sorted[i] < s.sorted[j] })
	return s
}

func (s statsSnap) percentile(p float64) time.Duration {
	n := len(s.sorted)
	if n == 0 {
		return 0
	}
	idx := int(float64(n-1) * p)
	return s.sorted[idx]
}

// ── reporting ──────────────────────────────────────────────────

func fmtLat(d time.Duration) string {
	us := d.Microseconds()
	if us < 1000 {
		return fmt.Sprintf("%dus", us)
	}
	return fmt.Sprintf("%.1fms", float64(us)/1000)
}

func printProgress(elapsed time.Duration, s statsSnap) {
	qps := float64(s.total) / elapsed.Seconds()
	errs := s.timeouts + s.errors
	fmt.Printf("[%4.0fs] sent=%-8d qps=%-8.0f p50=%-8s p95=%-8s p99=%-8s err=%d\n",
		elapsed.Seconds(), s.total, qps,
		fmtLat(s.percentile(0.50)),
		fmtLat(s.percentile(0.95)),
		fmtLat(s.percentile(0.99)),
		errs)
}

func printSummary(elapsed time.Duration, s statsSnap) {
	fmt.Println()
	fmt.Println("==============================================")
	fmt.Println("  Summary")
	fmt.Println("==============================================")
	fmt.Printf("  Queries:    %d\n", s.total)
	fmt.Printf("  Duration:   %.2fs\n", elapsed.Seconds())
	if elapsed.Seconds() > 0 {
		fmt.Printf("  QPS:        %.1f\n", float64(s.total)/elapsed.Seconds())
	}
	fmt.Printf("  Timeouts:   %d\n", s.timeouts)
	fmt.Printf("  Errors:     %d\n", s.errors)
	fmt.Println()

	if len(s.sorted) > 0 {
		fmt.Println("  Latency:")
		type pctEntry struct {
			label string
			value float64
		}
		for _, p := range []pctEntry{
			{"Min", 0}, {"P50", 0.50}, {"P75", 0.75}, {"P90", 0.90},
			{"P95", 0.95}, {"P99", 0.99}, {"P999", 0.999}, {"Max", 1},
		} {
			var d time.Duration
			switch p.label {
			case "Min":
				d = s.sorted[0]
			case "Max":
				d = s.sorted[len(s.sorted)-1]
			default:
				d = s.percentile(p.value)
			}
			fmt.Printf("    %-6s  %s\n", p.label, fmtLat(d))
		}
		fmt.Println()
	}

	if len(s.rcodes) > 0 {
		fmt.Println("  Response codes:")
		for rc, cnt := range s.rcodes {
			pct := float64(cnt) / float64(s.total) * 100
			fmt.Printf("    %-12s  %d (%.1f%%)\n", dns.RcodeToString[rc], cnt, pct)
		}
		fmt.Println()
	}
}

// ── dispatch ───────────────────────────────────────────────────

// dispatchQueries sends queries to workers until count is reached,
// stop is closed, or (in duration mode with count==0) indefinitely.
func dispatchQueries(base []query, ch chan<- query, stop <-chan struct{},
	ticker *time.Ticker, randomPrefix bool, count int) {

	for i := 0; count == 0 || i < count; i++ {
		// Check early termination (Ctrl+C or duration expired).
		select {
		case <-stop:
			return
		default:
		}

		// Rate limit.
		if ticker != nil {
			select {
			case <-ticker.C:
			case <-stop:
				return
			}
		}

		// Build query, optionally adding random subdomain prefix.
		q := base[i%len(base)]
		if randomPrefix {
			q.name = fmt.Sprintf("%08x.%s", rand.Uint32(), q.name)
		}

		// Send to worker pool.
		select {
		case ch <- q:
		case <-stop:
			return
		}
	}
}

// ── main ───────────────────────────────────────────────────────

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dnsperf [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  dnsperf -server 127.0.0.1:53 -f queries.txt -c 50 -n 100000\n")
		fmt.Fprintf(os.Stderr, "  dnsperf -server 127.0.0.1:53 -random-prefix example.com -c 10 -d 30s\n")
		fmt.Fprintf(os.Stderr, "  dnsperf -server 127.0.0.1:53 -f queries.txt -qps 5000 -d 60s\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// ── validate ──

	if *flagFile == "" && *flagRandomPrefix == "" {
		die("specify -f <query-file> or -random-prefix <domain>")
	}
	if *flagFile != "" && *flagRandomPrefix != "" {
		die("-f and -random-prefix are mutually exclusive")
	}
	if *flagConcurrency < 1 {
		die("-c must be >= 1")
	}
	if p := *flagProto; p != "udp" && p != "tcp" {
		die("-proto must be udp or tcp")
	}

	// Resolve count vs duration: -d takes precedence.
	if *flagDuration > 0 {
		*flagCount = 0
	} else if *flagCount == 0 {
		*flagCount = 10000
	}

	// ── load queries ──

	var base []query
	if *flagFile != "" {
		var err error
		if base, err = loadQueriesFromFile(*flagFile); err != nil {
			die(err.Error())
		}
	} else {
		base = []query{{name: dns.Fqdn(*flagRandomPrefix), qtype: dns.TypeA}}
	}

	// ── header ──

	fmt.Println("DNS Performance Testing Tool")
	fmt.Printf("  Server:      %s (%s)\n", *flagServer, *flagProto)
	fmt.Printf("  Concurrency: %d\n", *flagConcurrency)
	if *flagCount > 0 {
		fmt.Printf("  Queries:     %d\n", *flagCount)
	}
	if *flagDuration > 0 {
		fmt.Printf("  Duration:    %s\n", *flagDuration)
	}
	if *flagQPS > 0 {
		fmt.Printf("  Rate limit:  %d qps\n", *flagQPS)
	}
	if *flagRandomPrefix != "" {
		fmt.Printf("  Mode:        random-prefix -> *.%s\n", *flagRandomPrefix)
	} else {
		fmt.Printf("  Query file:  %s (%d entries)\n", *flagFile, len(base))
	}
	fmt.Println()

	// ── wiring ──

	queryCh := make(chan query, *flagConcurrency*2)
	resultCh := make(chan result, *flagConcurrency*2)
	stop := make(chan struct{})
	var stopOnce sync.Once
	doStop := func() { stopOnce.Do(func() { close(stop) }) }

	// Ctrl+C handler.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		fmt.Fprintln(os.Stderr, "\ninterrupted, finishing in-flight queries...")
		doStop()
	}()

	// Duration timer — closes stop channel when time is up.
	if *flagDuration > 0 {
		go func() {
			select {
			case <-time.After(*flagDuration):
				doStop()
			case <-stop:
			}
		}()
	}

	// ── workers ──

	var wg sync.WaitGroup
	for i := 0; i < *flagConcurrency; i++ {
		wg.Add(1)
		go worker(*flagServer, *flagProto, *flagTimeout, queryCh, resultCh, &wg)
	}

	// ── collector ──

	estCap := *flagCount
	if estCap == 0 {
		estCap = 100000
	}
	coll := newCollector(estCap)
	collDone := make(chan struct{})
	go func() {
		for r := range resultCh {
			coll.record(r)
		}
		close(collDone)
	}()

	// ── periodic reporter ──

	startTime := time.Now()
	repStop := make(chan struct{})
	go func() {
		tk := time.NewTicker(5 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				printProgress(time.Since(startTime), coll.snapshot())
			case <-repStop:
				return
			}
		}
	}()

	// ── rate limiter ──

	var rateTicker *time.Ticker
	if *flagQPS > 0 {
		rateTicker = time.NewTicker(time.Second / time.Duration(*flagQPS))
	}

	// ── dispatch ──

	dispatchQueries(base, queryCh, stop, rateTicker, *flagRandomPrefix != "", *flagCount)

	// ── teardown ──

	close(queryCh)  // signal workers: no more queries
	wg.Wait()       // wait for workers to finish in-flight queries
	close(resultCh) // signal collector: no more results
	<-collDone      // wait for collector to finish
	close(repStop)  // stop periodic reporter
	if rateTicker != nil {
		rateTicker.Stop()
	}

	// ── final report ──

	printSummary(time.Since(startTime), coll.snapshot())
}

func die(msg string) {
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	os.Exit(1)
}
