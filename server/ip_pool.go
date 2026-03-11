package server

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
)

const (
	INIT_IP_LATENCY     = 1000
	MAX_IP_LATENCY      = 10000
	MAX_PREFETCH_CONCUR = 10 // 最大并发 prefetch 数
	PREFETCH_TIMEOUT    = 3  // prefetch 超时秒数
)

type IPQuality struct {
	isInit  atomic.Bool
	latency int32
}

func NewIPQuality() *IPQuality {
	ipq := &IPQuality{
		latency: INIT_IP_LATENCY,
	}
	ipq.isInit.Store(true)
	return ipq
}

func (ipq *IPQuality) Init() {
	ipq.isInit.Store(true)
	atomic.StoreInt32(&ipq.latency, INIT_IP_LATENCY)
}

// IsInit returns whether the IP quality has been initialized
func (ipq *IPQuality) IsInit() bool {
	return ipq.isInit.Load()
}

func (ipq *IPQuality) GetLatency() int32 {
	return atomic.LoadInt32(&ipq.latency)
}

func (ipq *IPQuality) SetLatency(latency int32) {
	atomic.StoreInt32(&ipq.latency, latency)
}

func (ipq *IPQuality) SetLatencyAndState(latency int32) {
	atomic.StoreInt32(&ipq.latency, latency)
	ipq.isInit.Store(false)
}

type IPPool struct {
	pool      map[string]*IPQuality
	l         sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	sem       chan struct{} // semaphore for concurrency limit
	dnsClient *dns.Client
}

var globalIPPool = NewIPPool()

func NewIPPool() *IPPool {
	ctx, cancel := context.WithCancel(context.Background())
	ipp := &IPPool{
		pool:   make(map[string]*IPQuality),
		l:      sync.RWMutex{},
		ctx:    ctx,
		cancel: cancel,
		sem:    make(chan struct{}, MAX_PREFETCH_CONCUR),
		dnsClient: &dns.Client{
			Net:     "udp",
			Timeout: PREFETCH_TIMEOUT * time.Second,
		},
	}
	return ipp
}

// Shutdown gracefully stops all prefetch goroutines
func (ipp *IPPool) Shutdown(ctx context.Context) error {
	ipp.cancel()
	done := make(chan struct{})
	go func() {
		ipp.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (ipp *IPPool) isTheIPInit(ip string) bool {
	ipq := ipp.GetIPQuality(ip)
	if ipq == nil {
		ipq = &IPQuality{}
		ipq.Init()
		ipp.SetIPQuality(ip, ipq)
	}
	return ipq.IsInit()
}

func (ipp *IPPool) GetIPQuality(ip string) *IPQuality {
	ipp.l.RLock()
	defer ipp.l.RUnlock()
	if ipq, ok := ipp.pool[ip]; ok {
		return ipq
	}
	return nil
}

func (ipp *IPPool) SetIPQuality(ip string, ipq *IPQuality) {
	ipp.l.Lock()
	defer ipp.l.Unlock()
	ipp.pool[ip] = ipq
}

func (ipp *IPPool) updateIPQuality(ip string, latency int32) {
	ipq := ipp.GetIPQuality(ip)
	if ipq == nil {
		ipq = &IPQuality{}
		ipq.Init()
		ipp.SetIPQuality(ip, ipq)
	}
	ipq.SetLatencyAndState(latency)
}

func (ipp *IPPool) UpIPsQuality(ips []string) {
	for _, ip := range ips {
		ipq := ipp.GetIPQuality(ip)
		if ipq == nil {
			ipq = &IPQuality{}
			ipq.Init()
			ipp.SetIPQuality(ip, ipq)
		}
		if !ipq.IsInit() {
			continue
		}
		currentLatency := ipq.GetLatency()
		nextLatency := int32(float64(currentLatency) * 0.9)
		ipq.SetLatency(nextLatency)
	}
}

func (ipp *IPPool) getBestIPs(ips []string) (string, string) {
	var (
		bestIP      string = ""
		bestLatency int32  = MAX_IP_LATENCY
		secondIP    string = ""
		secondDelay int32  = MAX_IP_LATENCY
	)

	for _, ip := range ips {
		ipq := ipp.GetIPQuality(ip)
		if ipq == nil {
			ipq = &IPQuality{}
			ipq.Init()
			ipp.SetIPQuality(ip, ipq)
		}
		currentLatency := ipq.GetLatency()
		monitor.Rec53Log.Debug(ip, ",", ipq.GetLatency())

		// Update best and second best IPs
		if currentLatency < bestLatency {
			// Current best becomes second best
			secondIP = bestIP
			secondDelay = bestLatency
			// New best
			bestIP = ip
			bestLatency = currentLatency
		} else if currentLatency < secondDelay && ip != bestIP {
			// Update second best if better than current second
			secondIP = ip
			secondDelay = currentLatency
		}
	}
	return bestIP, secondIP
}

func (ipp *IPPool) GetPrefetchIPs(bestIP string) []string {
	var prefetchIPs []string

	ipp.l.RLock()
	defer ipp.l.RUnlock()

	bestIPQuality, ok := ipp.pool[bestIP]
	if !ok {
		return prefetchIPs
	}
	theBestLatency := bestIPQuality.GetLatency()

	for ip, ipq := range ipp.pool {
		latency := ipq.GetLatency()
		if latency < theBestLatency && int32(float32(latency)/0.9) > theBestLatency && ip != bestIP {
			prefetchIPs = append(prefetchIPs, ip)
		}
	}
	return prefetchIPs
}

// PrefetchIPs prefetches IP quality for given IPs with concurrency control
func (ipp *IPPool) PrefetchIPs(ips []string) {
	for _, ip := range ips {
		select {
		case ipp.sem <- struct{}{}:
			ipp.wg.Add(1)
			go func(targetIP string) {
				defer ipp.wg.Done()
				defer func() { <-ipp.sem }()

				ipp.prefetchIPQuality(targetIP)
			}(ip)
		default:
			// Skip if semaphore is full (too many concurrent prefetches)
			monitor.Rec53Log.Debugf("skip prefetch for %s: too many concurrent prefetches", ip)
		}
	}
}

// prefetchIPQuality performs the actual IP quality check
func (ipp *IPPool) prefetchIPQuality(ip string) {
	select {
	case <-ipp.ctx.Done():
		return
	default:
	}

	_, rtt, err := ipp.dnsClient.Exchange(&dns.Msg{}, ip+":"+getIterPort())
	if err != nil {
		monitor.Rec53Log.Errorf("prefetch ip %s error: %s", ip, err.Error())
		return
	}

	ipq := NewIPQuality()
	ipq.SetLatencyAndState(int32(rtt / time.Millisecond))
	ipp.SetIPQuality(ip, ipq)
	monitor.Rec53Metric.IPQualityGaugeSet(ip, float64(rtt/time.Millisecond))
}

// ResetIPPoolForTest replaces the global IP pool with a fresh instance.
// Exported for use by E2E tests to ensure a clean state.
func ResetIPPoolForTest() {
	// Shutdown existing pool's goroutines
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	globalIPPool.Shutdown(ctx)

	globalIPPool = NewIPPool()
}
