package server

import (
	"context"
	"testing"

	"github.com/miekg/dns"
)

// TestCacheLookupState_CacheHitEmptyAnswer verifies that a cache hit with an
// empty answer section is treated as a miss (falls through to CACHE_LOOKUP_MISS).
func TestCacheLookupState_CacheHitEmptyAnswer(t *testing.T) {
	deleteAllCache()

	domain := "empty-answer.example.com."
	// Store a message with no Answer records under this domain/type.
	emptyMsg := new(dns.Msg)
	emptyMsg.Answer = nil
	setCacheCopyByType(domain, dns.TypeA, emptyMsg, 300)

	req := new(dns.Msg)
	req.SetQuestion(domain, dns.TypeA)
	resp := new(dns.Msg)

	s := newCacheLookupState(req, resp, context.Background())
	ret, err := s.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != CACHE_LOOKUP_MISS {
		t.Errorf("expected CACHE_LOOKUP_MISS for empty-answer cache entry, got %d", ret)
	}
}

// TestResolveNSIPsRecursively_Empty verifies that resolveNSIPsRecursively with
// no NS names returns nil without panicking.
func TestResolveNSIPsRecursively_Empty(t *testing.T) {
	result := resolveNSIPsRecursively(context.Background(), nil)
	if result != nil {
		t.Errorf("expected nil result for empty input, got %v", result)
	}

	result = resolveNSIPsRecursively(context.Background(), []string{})
	if result != nil {
		t.Errorf("expected nil result for empty slice, got %v", result)
	}
}

// context fall back to context.Background() when nil is passed, and that each
// state's getContext() also falls back when the stored ctx field is nil.

func TestCacheLookupStateWithContext_NilCtx(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)

	s := newCacheLookupState(req, resp, nil)
	if s.ctx == nil {
		t.Fatal("expected ctx to be non-nil after newCacheLookupStateWithContext(nil)")
	}
	if s.ctx == context.Background() {
		// valid – exactly context.Background()
	}
	// getContext on a legitimately-nil ctx field should still return non-nil.
	s.ctx = nil
	got := s.getContext()
	if got == nil {
		t.Error("getContext() returned nil for nil ctx field")
	}
}

func TestClassifyRespStateWithContext_NilCtx(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)

	s := newClassifyRespState(req, resp, nil)
	if s.ctx == nil {
		t.Fatal("expected ctx to be non-nil after newClassifyRespStateWithContext(nil)")
	}
	s.ctx = nil
	got := s.getContext()
	if got == nil {
		t.Error("getContext() returned nil for nil ctx field")
	}
}

func TestExtractGlueStateWithContext_NilCtx(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)

	s := newExtractGlueState(req, resp, nil)
	if s.ctx == nil {
		t.Fatal("expected ctx to be non-nil after newExtractGlueStateWithContext(nil)")
	}
	s.ctx = nil
	got := s.getContext()
	if got == nil {
		t.Error("getContext() returned nil for nil ctx field")
	}
}

func TestStateInitStateWithContext_NilCtx(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)

	s := newStateInitState(req, resp, nil)
	if s.ctx == nil {
		t.Fatal("expected ctx to be non-nil after newStateInitStateWithContext(nil)")
	}
	s.ctx = nil
	got := s.getContext()
	if got == nil {
		t.Error("getContext() returned nil for nil ctx field")
	}
}

func TestLookupNSCacheStateWithContext_NilCtx(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)

	s := newLookupNSCacheState(req, resp, nil)
	if s.ctx == nil {
		t.Fatal("expected ctx to be non-nil after newLookupNSCacheStateWithContext(nil)")
	}
	s.ctx = nil
	got := s.getContext()
	if got == nil {
		t.Error("getContext() returned nil for nil ctx field")
	}
}

func TestQueryUpstreamStateWithContext_NilCtx(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)

	s := newQueryUpstreamState(req, resp, nil)
	if s.ctx == nil {
		t.Fatal("expected ctx to be non-nil after newQueryUpstreamStateWithContext(nil)")
	}
	s.ctx = nil
	got := s.getContext()
	if got == nil {
		t.Error("getContext() returned nil for nil ctx field")
	}
}

func TestReturnRespStateWithContext_NilCtx(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)

	s := newReturnRespState(req, resp, nil)
	if s.ctx == nil {
		t.Fatal("expected ctx to be non-nil after newReturnRespStateWithContext(nil)")
	}
	// returnRespState.getContext is tested; set ctx=nil on underlying field.
	s.ctx = nil
	got := s.getContext()
	if got == nil {
		t.Error("getContext() returned nil for nil ctx field")
	}
}

// TestStateWithContext_PropagatesCtx verifies that a real (non-nil) context is
// correctly stored and returned by getContext().
func TestStateWithContext_PropagatesCtx(t *testing.T) {
	type ctxKey string
	key := ctxKey("test-key")
	parent := context.WithValue(context.Background(), key, "marker")

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)

	t.Run("cacheLookup", func(t *testing.T) {
		s := newCacheLookupState(req, resp, parent)
		if s.getContext().Value(key) != "marker" {
			t.Error("context value not propagated")
		}
	})

	t.Run("classifyResp", func(t *testing.T) {
		s := newClassifyRespState(req, resp, parent)
		if s.getContext().Value(key) != "marker" {
			t.Error("context value not propagated")
		}
	})

	t.Run("extractGlue", func(t *testing.T) {
		s := newExtractGlueState(req, resp, parent)
		if s.getContext().Value(key) != "marker" {
			t.Error("context value not propagated")
		}
	})

	t.Run("stateInit", func(t *testing.T) {
		s := newStateInitState(req, resp, parent)
		if s.getContext().Value(key) != "marker" {
			t.Error("context value not propagated")
		}
	})

	t.Run("lookupNSCache", func(t *testing.T) {
		s := newLookupNSCacheState(req, resp, parent)
		if s.getContext().Value(key) != "marker" {
			t.Error("context value not propagated")
		}
	})

	t.Run("queryUpstream", func(t *testing.T) {
		s := newQueryUpstreamState(req, resp, parent)
		if s.getContext().Value(key) != "marker" {
			t.Error("context value not propagated")
		}
	})

	t.Run("returnResp", func(t *testing.T) {
		s := newReturnRespState(req, resp, parent)
		if s.getContext().Value(key) != "marker" {
			t.Error("context value not propagated")
		}
	})
}
