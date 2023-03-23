package server

import "github.com/miekg/dns"

//imlement dns cache
type Cache interface {
	//get dns cache
	GetCache() map[string]*dns.Msg
	//set dns cache
	SetCache(key string, value *dns.Msg)
	//delete dns cache
	DeleteCache(key string)
	//get dns cache size
	GetCacheSize() int
	//set dns cache size
	SetCacheSize(size int)
}

//dns cache
type cache struct {
	cacheSize int
	cache     map[string]*dns.Msg
}

//get dns cache
func (c *cache) GetCache() map[string]*dns.Msg {
	return c.cache
}

//set dns cache
func (c *cache) SetCache(key string, value *dns.Msg) {
	c.cache[key] = value
}

//delete dns cache
func (c *cache) DeleteCache(key string) {
	delete(c.cache, key)
}

//get dns cache size
func (c *cache) GetCacheSize() int {
	return c.cacheSize
}

//set dns cache size
func (c *cache) SetCacheSize(size int) {
	c.cacheSize = size
}

//new dns cache
func NewCache() Cache {
	return &cache{
		cache: make(map[string]*dns.Msg),
	}
}

//get dns cache
func GetCache() map[string]*dns.Msg {
	return cacheInstance.GetCache()
}

//set dns cache
func SetCache(key string, value *dns.Msg) {
	cacheInstance.SetCache(key, value)
}

//delete dns cache
func DeleteCache(key string) {
	cacheInstance.DeleteCache(key)
}

//get dns cache size
func GetCacheSize() int {
	return cacheInstance.GetCacheSize()
}

//set dns cache size
func SetCacheSize(size int) {
	cacheInstance.SetCacheSize(size)
}

//dns cache instance
var cacheInstance = NewCache()
