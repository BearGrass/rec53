package server

import (
	"strconv"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
)

var globalDnsCache = newCache()

// imlement dns cache with go-cache library
func newCache() *cache.Cache {
	return cache.New(5*time.Minute, 10*time.Minute)
}

// getCacheKey generates a cache key that includes both domain name and query type.
// This ensures different record types for the same domain are cached separately.
func getCacheKey(name string, qtype uint16) string {
	return name + ":" + strconv.FormatUint(uint64(qtype), 10)
}

func getCache(key string) (*dns.Msg, bool) {
	value, found := globalDnsCache.Get(key)
	if !found {
		return nil, false
	}
	return value.(*dns.Msg), true
}

// getCacheCopyByType returns a deep copy of the cached message for a specific query type.
func getCacheCopyByType(name string, qtype uint16) (*dns.Msg, bool) {
	key := getCacheKey(name, qtype)
	return getCacheCopy(key)
}

// setCacheCopyByType stores a copy of the message with the query type in the key.
func setCacheCopyByType(name string, qtype uint16, value *dns.Msg, expire uint32) {
	key := getCacheKey(name, qtype)
	setCacheCopy(key, value, expire)
}

// getCacheCopy returns a shallow copy of the cached message.
//
// Cache safety invariant:
//   - OPT records are stripped on write (setCacheCopy), so cached entries never
//     contain *dns.OPT. This eliminates the only known Pack()-induced mutation.
//   - The shallow copy allocates a new dns.Msg struct and new slice headers
//     (Answer, Ns, Extra) but shares the underlying RR pointers with the cached
//     entry. Individual RR structs are NOT deep-copied.
//   - Question is intentionally NOT copied. All production callers ignore the
//     returned Question; server.go restores the original question from the
//     client request before sending the reply.
//   - Callers may freely modify slice headers (append, truncate, nil) on the
//     returned message — the cached entry is not affected because the new slices
//     have cap == len, so any append triggers a new backing array.
//   - Callers MUST NOT modify individual RR struct fields (e.g., rr.Header().Ttl,
//     a.A) on the returned message. Doing so would corrupt the cached entry and
//     race with concurrent readers. The TestCacheConcurrentReadPack race test
//     enforces this invariant.
func getCacheCopy(key string) (*dns.Msg, bool) {
	msg, found := getCache(key)
	if !found {
		if monitor.Rec53Metric != nil {
			monitor.Rec53Metric.CacheLookupAdd("miss")
		}
		return nil, false
	}
	if monitor.Rec53Metric != nil {
		result := "delegation_hit"
		switch {
		case len(msg.Answer) > 0:
			result = "positive_hit"
		case hasSOAInAuthority(msg):
			result = "negative_hit"
		}
		monitor.Rec53Metric.CacheLookupAdd(result)
	}
	return shallowCopyMsg(msg), true
}

// shallowCopyMsg allocates a new dns.Msg with copied slice headers but shared
// RR pointers. The returned slices have cap == len so that any caller append
// triggers a new backing array, leaving the source slices untouched.
//
// Question is intentionally omitted: all production callers (state_cache_lookup,
// state_lookup_ns_cache, state_query_upstream) do not use the returned Question,
// and server.go restores the original question from the client request before
// sending the reply. Skipping the Question allocation saves 1 alloc/op on every
// cache read.
func shallowCopyMsg(m *dns.Msg) *dns.Msg {
	cp := new(dns.Msg)
	cp.MsgHdr = m.MsgHdr
	cp.Compress = m.Compress

	if len(m.Answer) > 0 {
		cp.Answer = make([]dns.RR, len(m.Answer))
		copy(cp.Answer, m.Answer)
	}
	if len(m.Ns) > 0 {
		cp.Ns = make([]dns.RR, len(m.Ns))
		copy(cp.Ns, m.Ns)
	}
	if len(m.Extra) > 0 {
		cp.Extra = make([]dns.RR, len(m.Extra))
		copy(cp.Extra, m.Extra)
	}
	return cp
}

func setCache(key string, value interface{}, expire uint32) {
	expireTime := time.Duration(expire) * time.Second
	globalDnsCache.Set(key, value, expireTime)
	if monitor.Rec53Metric != nil {
		monitor.Rec53Metric.CacheLifecycleAdd("write", 1)
		monitor.Rec53Metric.CacheEntriesSet(globalDnsCache.ItemCount())
	}
}

// stripOPT removes all *dns.OPT records from msg.Extra in place.
// OPT records are EDNS0 per-query transport metadata and should not be cached.
// Stripping them eliminates the only known Pack()-induced mutation
// (OPT.SetExtendedRcode modifying OPT.Hdr.Ttl), making shared RR pointers
// safe for concurrent Pack() calls after shallow copy.
func stripOPT(msg *dns.Msg) {
	if len(msg.Extra) == 0 {
		return
	}
	n := 0
	for _, rr := range msg.Extra {
		if _, ok := rr.(*dns.OPT); !ok {
			msg.Extra[n] = rr
			n++
		}
	}
	// Clear trailing references to help GC and truncate.
	for i := n; i < len(msg.Extra); i++ {
		msg.Extra[i] = nil
	}
	msg.Extra = msg.Extra[:n]
}

// setCacheCopy stores a copy of the message to prevent
// the cached message from being modified later.
// The deep copy is followed by OPT stripping to ensure cached entries
// contain no *dns.OPT records (see design decision D2).
// When XDP is enabled, the entry is also synced to the BPF cache map.
func setCacheCopy(key string, value *dns.Msg, expire uint32) {
	cp := value.Copy()
	stripOPT(cp)
	setCache(key, cp, expire)

	// Sync to BPF map for XDP fast path (no-op if globalXDPCacheMap is nil).
	// Guard: only sync positive responses (non-empty Answer) with a Question
	// section. Negative responses (NXDOMAIN/NODATA) and NS delegation entries
	// have empty Answer sections and must NOT be synced — XDP cannot serve
	// them correctly (no SOA in Authority after buildBPFCacheValue strips Ns).
	if len(cp.Answer) > 0 && len(cp.Question) > 0 {
		syncToBPFMap(cp.Question[0].Name, cp.Question[0].Qtype, cp, expire)
	}
}

func deleteExpiredCache() {
	before := globalDnsCache.ItemCount()
	globalDnsCache.DeleteExpired()
	if monitor.Rec53Metric != nil {
		after := globalDnsCache.ItemCount()
		if before > after {
			monitor.Rec53Metric.CacheLifecycleAdd("delete_expired", before-after)
		}
		monitor.Rec53Metric.CacheEntriesSet(after)
	}
}

func deleteAllCache() {
	before := globalDnsCache.ItemCount()
	globalDnsCache.Flush()
	if monitor.Rec53Metric != nil {
		if before > 0 {
			monitor.Rec53Metric.CacheLifecycleAdd("flush", before)
		}
		monitor.Rec53Metric.CacheEntriesSet(0)
	}
}

func getCacheSize() int {
	return globalDnsCache.ItemCount()
}

// FlushCacheForTest flushes the global DNS cache.
// Exported for use by E2E tests to ensure a clean state.
func FlushCacheForTest() {
	deleteAllCache()
}
