package server

import (
	"time"

	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
)

var globalDnsCache = newCache()

// imlement dns cache with go-cache library
func newCache() *cache.Cache {
	return cache.New(5*time.Minute, 10*time.Minute)
}

func getCache(key string) (*dns.Msg, bool) {
	value, found := globalDnsCache.Get(key)
	if !found {
		return nil, false
	}
	return value.(*dns.Msg), true
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

func deleteCache(key string) {
	globalDnsCache.Delete(key)
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
