package server

import (
	"fmt"
	"sync/atomic"

	"github.com/cilium/ebpf"
	"github.com/miekg/dns"
	"golang.org/x/sys/unix"

	"rec53/monitor"
)

// globalXDPCacheMap holds the BPF cache_map handle.
// When nil (zero value), XDP cache sync is disabled (no-op).
// Set by server integration when XDP is enabled.
// Uses atomic.Pointer for safe concurrent access from cache write goroutines
// and server Shutdown().
var globalXDPCacheMap atomic.Pointer[ebpf.Map]

// domainToWireFormat converts a presentation-format DNS domain name (e.g.
// "example.com.") to wire format (length-prefixed label sequence) with inline
// lowercase normalization. The result matches exactly what the eBPF program
// extracts from network packets.
func domainToWireFormat(name string) ([]byte, error) {
	// Canonicalize: lowercase + ensure trailing dot (FQDN).
	name = dns.CanonicalName(name)

	buf := make([]byte, 255)
	off, err := dns.PackDomainName(name, buf, 0, nil, false)
	if err != nil {
		return nil, fmt.Errorf("wire-format qname for %q: %w", name, err)
	}

	return buf[:off], nil
}

// buildBPFCacheKey constructs a BPF cache_map key from a domain name and
// query type. The key matches the format used by the eBPF program for lookups.
func buildBPFCacheKey(name string, qtype uint16) (dnsCacheCacheKey, error) {
	var key dnsCacheCacheKey

	wire, err := domainToWireFormat(name)
	if err != nil {
		return key, err
	}

	copy(key.Qname[:], wire)
	key.Qtype = qtype
	return key, nil
}

// getMonotonicSeconds returns the current monotonic clock time in seconds,
// matching the clock source used by bpf_ktime_get_ns() in the eBPF program.
func getMonotonicSeconds() (uint64, error) {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		return 0, fmt.Errorf("ClockGettime(CLOCK_MONOTONIC): %w", err)
	}
	return uint64(ts.Sec), nil
}

// buildBPFCacheValue constructs a BPF cache_map value from a DNS message and
// TTL. The message is stripped to a minimal answer-only response before
// serialization via Pack(). The expire_ts is calculated using monotonic clock
// + TTL seconds.
func buildBPFCacheValue(msg *dns.Msg, ttlSeconds uint32) (dnsCacheCacheValue, error) {
	var val dnsCacheCacheValue

	// Build minimal response (Question+Answer only) to fit 512-byte limit.
	// The full cached message may include Ns (authority) and Extra (additional)
	// sections from iterative resolution, which inflate the wire size beyond
	// the 512-byte XDP limit. Strip them to match what the Go resolver would
	// send to a UDP client.
	minimal := new(dns.Msg)
	minimal.MsgHdr = msg.MsgHdr
	minimal.Compress = true
	minimal.Question = msg.Question
	minimal.Answer = msg.Answer

	// Serialize the DNS response.
	packed, err := minimal.Pack()
	if err != nil {
		return val, fmt.Errorf("dns.Msg.Pack() failed: %w", err)
	}

	if len(packed) > 512 {
		return val, fmt.Errorf("packed response size %d exceeds MAX_DNS_RESPONSE_LEN (512)", len(packed))
	}

	// Calculate monotonic expiration time.
	now, err := getMonotonicSeconds()
	if err != nil {
		return val, err
	}

	val.ExpireTs = now + uint64(ttlSeconds)
	val.RespLen = uint32(len(packed))
	copy(val.Response[:], packed)

	return val, nil
}

// syncToBPFMap synchronizes a DNS cache entry to the BPF cache_map.
// This is called inline from setCacheCopy() when XDP is enabled.
// If globalXDPCacheMap is nil (XDP disabled), this is a no-op.
// Errors are logged at Debug level and do not affect Go cache operation.
func syncToBPFMap(name string, qtype uint16, msg *dns.Msg, ttlSeconds uint32) {
	cacheMap := globalXDPCacheMap.Load()
	if cacheMap == nil {
		return
	}

	key, err := buildBPFCacheKey(name, qtype)
	if err != nil {
		monitor.Rec53Log.Debugf("[XDP] sync skipped for %s (type %d): key build failed: %v", name, qtype, err)
		return
	}

	val, err := buildBPFCacheValue(msg, ttlSeconds)
	if err != nil {
		monitor.Rec53Log.Debugf("[XDP] sync skipped for %s (type %d): value build failed: %v", name, qtype, err)
		return
	}

	if err := cacheMap.Update(key, val, ebpf.UpdateAny); err != nil {
		monitor.Rec53Log.Debugf("[XDP] BPF map update failed for %s (type %d): %v", name, qtype, err)
	}
}
