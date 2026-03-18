package server

import (
	"strconv"
	"time"

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
//     (Question, Answer, Ns, Extra) but shares the underlying RR pointers with
//     the cached entry. Individual RR structs are NOT deep-copied.
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
		return nil, false
	}
	return shallowCopyMsg(msg), true
}

// shallowCopyMsg allocates a new dns.Msg with copied slice headers but shared
// RR pointers. The returned slices have cap == len so that any caller append
// triggers a new backing array, leaving the source slices untouched.
func shallowCopyMsg(m *dns.Msg) *dns.Msg {
	cp := new(dns.Msg)
	cp.MsgHdr = m.MsgHdr
	cp.Compress = m.Compress

	if len(m.Question) > 0 {
		cp.Question = make([]dns.Question, len(m.Question))
		copy(cp.Question, m.Question)
	}
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
func setCacheCopy(key string, value *dns.Msg, expire uint32) {
	cp := value.Copy()
	stripOPT(cp)
	setCache(key, cp, expire)
}

func deleteExpiredCache() {
	globalDnsCache.DeleteExpired()
}

func deleteAllCache() {
	globalDnsCache.Flush()
}

func getCacheSize() int {
	return globalDnsCache.ItemCount()
}

// FlushCacheForTest flushes the global DNS cache.
// Exported for use by E2E tests to ensure a clean state.
func FlushCacheForTest() {
	deleteAllCache()
}
