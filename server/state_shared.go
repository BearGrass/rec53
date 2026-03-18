package server

import (
	"context"
	"sync/atomic"

	"github.com/miekg/dns"
)

// hostsForwardSnapshot is an immutable snapshot of the hosts and forwarding
// configuration. A new snapshot is created on each configuration update and
// stored atomically, so readers never observe a partially-updated state.
// Same pattern as globalDnsCache / globalIPPool but uses atomic.Pointer
// because hosts/forward config is always replaced as a whole unit.
type hostsForwardSnapshot struct {
	hostsMap     map[string]*dns.Msg
	hostsNames   map[string]bool
	forwardZones []ForwardZone
}

// globalHostsForward holds the current configuration snapshot.
// All reads and writes go through atomic Load/Store to avoid data races.
var globalHostsForward atomic.Pointer[hostsForwardSnapshot]

func init() {
	// Ensure Load() is always non-nil; avoids nil checks in hot path.
	globalHostsForward.Store(&hostsForwardSnapshot{})
}

func setGlobalHostsAndForward(hostsMap map[string]*dns.Msg, hostsNames map[string]bool, fwdZones []ForwardZone) {
	globalHostsForward.Store(&hostsForwardSnapshot{
		hostsMap:     hostsMap,
		hostsNames:   hostsNames,
		forwardZones: fwdZones,
	})
}

// SetHostsAndForwardForTest compiles hosts entries and sorts forward zones,
// then sets the package-level globals. Exported for e2e tests only.
func SetHostsAndForwardForTest(hosts []HostEntry, forwarding []ForwardZone) {
	hostsMap, hostsNames := compileHostsEntries(hosts)
	fwdZones := sortForwardZones(forwarding)
	setGlobalHostsAndForward(hostsMap, hostsNames, fwdZones)
}

// ResetHostsAndForwardForTest clears the package-level hosts and forward globals.
// Exported for e2e test cleanup.
func ResetHostsAndForwardForTest() {
	globalHostsForward.Store(&hostsForwardSnapshot{})
}

// setSnapshotForTest atomically stores a pre-built snapshot.
// For use by package-internal tests only; not exported.
func setSnapshotForTest(snap *hostsForwardSnapshot) {
	globalHostsForward.Store(snap)
}

// baseState holds the three fields common to every state struct and provides
// default implementations of the getRequest / getResponse / getContext methods
// defined by the stateMachine interface.
type baseState struct {
	request  *dns.Msg
	response *dns.Msg
	ctx      context.Context
}

func (b *baseState) getRequest() *dns.Msg  { return b.request }
func (b *baseState) getResponse() *dns.Msg { return b.response }
func (b *baseState) getContext() context.Context {
	if b.ctx == nil {
		return context.Background()
	}
	return b.ctx
}

// contextKeyType is the type for context keys
type contextKeyType string

// contextKeyWarmupDeadline is the context key for storing the warmup deadline
const contextKeyWarmupDeadline contextKeyType = "warmupDeadline"

// contextKeyNSResolutionDepth tracks recursive NS resolution depth to prevent deadlock.
// When resolveNSIPsConcurrently is resolving NS names, this key is set so that
// nested iterState.handle calls know not to recursively resolve NS names again.
const contextKeyNSResolutionDepth contextKeyType = "nsResolutionDepth"

// DefaultNegativeCacheTTL is the default TTL for negative responses (NXDOMAIN/NODATA)
// when SOA minimum is not available or is zero.
// TODO: make this configurable via config file or command-line flag
const DefaultNegativeCacheTTL = 60

// extractSOAFromAuthority extracts the SOA record from the Authority section.
// Returns the SOA record and its negative-cache TTL per RFC 2308 Section 5:
// min(SOA RR TTL, SOA MINIMUM field). Falls back to DefaultNegativeCacheTTL
// if the computed value is 0.
// Returns nil, 0 if no SOA is found.
func extractSOAFromAuthority(response *dns.Msg) (*dns.SOA, uint32) {
	for _, rr := range response.Ns {
		if soa, ok := rr.(*dns.SOA); ok {
			// RFC 2308 §5: negative cache TTL = min(SOA TTL, SOA MINIMUM)
			ttl := soa.Hdr.Ttl
			if soa.Minttl < ttl {
				ttl = soa.Minttl
			}
			if ttl == 0 {
				ttl = DefaultNegativeCacheTTL
			}
			return soa, ttl
		}
	}
	return nil, 0
}

// hasSOAInAuthority checks if the Authority section contains a SOA record.
// This is used to identify negative responses (NXDOMAIN/NODATA).
func hasSOAInAuthority(response *dns.Msg) bool {
	for _, rr := range response.Ns {
		if _, ok := rr.(*dns.SOA); ok {
			return true
		}
	}
	return false
}
