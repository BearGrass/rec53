package server

import (
	"sync"
	"sync/atomic"

	"rec53/monitor"
)

const (
	INIT_IP_LATENCY = 1000
	MAX_IP_LATENCY  = 10000
)

type IPQuality struct {
	isInit  bool
	latency int32
}

func NewIPQuality() *IPQuality {
	return &IPQuality{
		isInit:  true,
		latency: INIT_IP_LATENCY,
	}
}

func (ipq *IPQuality) Init() {
	ipq.isInit = true
	ipq.latency = INIT_IP_LATENCY
}

func (ipq *IPQuality) GetLatency() int32 {
	return ipq.latency
}

func (ipq *IPQuality) SetLatency(latency int32) {
	atomic.StoreInt32(&ipq.latency, latency)
}

func (ipq *IPQuality) SetLatencyAndState(latency int32) {
	atomic.StoreInt32(&ipq.latency, latency)
	ipq.isInit = false
}

type IPPool struct {
	pool map[string]*IPQuality
	l    sync.RWMutex
}

var globalIPPool = NewIPPool()

func NewIPPool() *IPPool {
	return &IPPool{
		pool: make(map[string]*IPQuality),
		l:    sync.RWMutex{},
	}
}

func (ipp *IPPool) isTheIPInit(ip string) bool {
	ipq := ipp.GetIPQuality(ip)
	if ipq == nil {
		ipq = &IPQuality{}
		ipq.Init()
		ipp.SetIPQuality(ip, ipq)
	}
	return ipq.isInit
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
		if !ipq.isInit {
			continue
		}
		currentLatency := ipq.GetLatency()
		nextLatency := int32(float64(currentLatency) * 0.9)
		ipq.SetLatency(nextLatency)
	}
}

func (ipp *IPPool) getBestIPs(ips []string) (string, string) {
	var (
		bestIP                 string = ""
		bestLatency            int32  = MAX_IP_LATENCY
		bestIPWithoutInit      string = ""
		bestLatencyWithoutInit int32  = MAX_IP_LATENCY
	)

	for _, ip := range ips {
		ipq := ipp.GetIPQuality(ip)
		if ipq == nil {
			ipq = &IPQuality{}
			ipq.Init()
			ipp.SetIPQuality(ip, ipq)
		}
		currentLatency := ipq.GetLatency()
		if currentLatency < bestLatency {
			bestIP = ip
			bestLatency = currentLatency
		}
		if !ipq.isInit && currentLatency < bestLatencyWithoutInit {
			bestIPWithoutInit = ip
			bestLatencyWithoutInit = currentLatency
		}
		monitor.Rec53Log.Debug(ip, ",", ipq.GetLatency())
	}
	return bestIP, bestIPWithoutInit
}

func (ipp *IPPool) GetPrefetchIPs(bestIP string) []string {
	var prefetchIPs []string
	theBestLatency := ipp.pool[bestIP].latency
	for ip, ipq := range ipp.pool {
		if ipq.latency < theBestLatency && int32(float32(ipq.latency)/0.9) > theBestLatency && ip != bestIP {
			prefetchIPs = append(prefetchIPs, ip)
		}
	}
	return prefetchIPs
}
