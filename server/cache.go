package server

import (
	"time"

	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
)

var globalDnsCache = newCache()

//imlement dns cache with go-cache library
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

func setCache(key string, value interface{}, expire uint32) {
	expireTime := time.Duration(expire) * time.Second
	globalDnsCache.Set(key, value, expireTime)
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
