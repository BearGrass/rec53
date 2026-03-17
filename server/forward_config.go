package server

import (
	"sort"

	"github.com/miekg/dns"
)

// ForwardZone describes a set of upstream DNS servers to use for a specific
// domain suffix. Queries whose name is a sub-domain of (or equal to) Zone are
// forwarded directly to Upstreams instead of being resolved iteratively.
//
// When multiple ForwardZone entries could match a query, the one with the
// longest Zone suffix wins (most-specific-match).
type ForwardZone struct {
	// Zone is the domain suffix to match (e.g. "corp.example.com").
	// Trailing dot is optional; it is normalised to a canonical FQDN on load.
	Zone string `yaml:"zone"`

	// Upstreams is the ordered list of upstream DNS server addresses in
	// "host:port" form (e.g. "192.168.1.1:53"). Each upstream is tried in
	// sequence; if all fail the resolver returns SERVFAIL.
	Upstreams []string `yaml:"upstreams"`
}

// sortForwardZones returns a copy of zones with each Zone normalised to FQDN
// and sorted by Zone length descending so that longest-suffix matching works
// by simple linear scan.
func sortForwardZones(zones []ForwardZone) []ForwardZone {
	if len(zones) == 0 {
		return nil
	}
	sorted := make([]ForwardZone, len(zones))
	copy(sorted, zones)
	for i := range sorted {
		sorted[i].Zone = dns.Fqdn(sorted[i].Zone)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].Zone) > len(sorted[j].Zone)
	})
	return sorted
}
