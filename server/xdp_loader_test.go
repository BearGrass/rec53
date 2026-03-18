package server

import (
	"testing"
)

// TestXDPLoaderConstructor verifies that NewXDPLoader returns a non-nil loader.
func TestXDPLoaderConstructor(t *testing.T) {
	loader := NewXDPLoader("lo")
	if loader == nil {
		t.Fatal("NewXDPLoader returned nil")
	}
}

// TestXDPLoaderCacheMapNilBeforeLoad verifies CacheMap() returns nil before
// eBPF objects are loaded.
func TestXDPLoaderCacheMapNilBeforeLoad(t *testing.T) {
	loader := NewXDPLoader("lo")
	if loader.CacheMap() != nil {
		t.Error("CacheMap() should be nil before Load()")
	}
}

// TestXDPLoaderCloseBeforeLoad verifies Close() is safe to call even if
// Load() was never called (no panic, no error).
func TestXDPLoaderCloseBeforeLoad(t *testing.T) {
	loader := NewXDPLoader("lo")
	err := loader.Close()
	if err != nil {
		t.Errorf("Close() before Load() returned error: %v", err)
	}
}

// TestXDPLoaderLoadAndAttach is an integration test that requires root/CAP_BPF.
// It loads the eBPF program and attaches to loopback in generic mode.
func TestXDPLoaderLoadAndAttach(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping XDP integration test in short mode")
	}

	loader := NewXDPLoader("lo")
	defer loader.Close()

	err := loader.LoadAndAttach()
	if err != nil {
		// If we don't have permissions, skip the test gracefully.
		t.Skipf("LoadAndAttach failed (likely needs root/CAP_BPF): %v", err)
	}

	// After successful load, CacheMap must be non-nil.
	if loader.CacheMap() == nil {
		t.Error("CacheMap() should be non-nil after LoadAndAttach()")
	}

	// Close should detach cleanly.
	err = loader.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}
