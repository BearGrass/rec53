package server

import (
	"context"

	"github.com/miekg/dns"
)

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
// Returns the SOA record and its TTL (or DefaultNegativeCacheTTL if SOA.Minttl is 0).
// Returns nil, 0 if no SOA is found.
func extractSOAFromAuthority(response *dns.Msg) (*dns.SOA, uint32) {
	for _, rr := range response.Ns {
		if soa, ok := rr.(*dns.SOA); ok {
			ttl := soa.Minttl
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
