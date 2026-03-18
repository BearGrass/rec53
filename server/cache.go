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

// getCacheCopy returns a deep copy of the cached message to prevent
// concurrent modification issues.
func getCacheCopy(key string) (*dns.Msg, bool) {
	msg, found := getCache(key)
	if !found {
		return nil, false
	}
	// Create a copy of the message
	return msg.Copy(), true
}

func setCache(key string, value interface{}, expire uint32) {
	expireTime := time.Duration(expire) * time.Second
	globalDnsCache.Set(key, value, expireTime)
}

// setCacheCopy stores a copy of the message to prevent
// the cached message from being modified later.
func setCacheCopy(key string, value *dns.Msg, expire uint32) {
	setCache(key, value.Copy(), expire)
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
