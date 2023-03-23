package server

//test cache.go
import (
	"testing"

	"github.com/miekg/dns"
)

func TestCache(t *testing.T) {
	cache := NewCache()
	cache.SetCacheSize(100)
	cache.SetCache("www.baidu.com", &dns.Msg{})
	if cache.GetCacheSize() != 100 {
		t.Error("cache size error")
	}
	if _, ok := cache.GetCache()["www.baidu.com"]; !ok {
		t.Error("cache set error")
	}
	cache.DeleteCache("www.baidu.com")
	if _, ok := cache.GetCache()["www.baidu.com"]; ok {
		t.Error("cache delete error")
	}
}
