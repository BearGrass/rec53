package server

import (
	"testing"
)

// TestLoadTLDListDefault verifies that LoadTLDList returns the curated default when no custom list is given.
func TestLoadTLDListDefault(t *testing.T) {
	result := LoadTLDList(nil)
	if len(result) == 0 {
		t.Fatal("expected non-empty TLD list, got empty")
	}
	if len(result) != len(DefaultCuratedTLDs) {
		t.Errorf("expected %d TLDs, got %d", len(DefaultCuratedTLDs), len(result))
	}
}

// TestLoadTLDListEmpty verifies that LoadTLDList returns the curated default when an empty list is given.
func TestLoadTLDListEmpty(t *testing.T) {
	result := LoadTLDList([]string{})
	if len(result) != len(DefaultCuratedTLDs) {
		t.Errorf("expected default list (%d TLDs), got %d TLDs", len(DefaultCuratedTLDs), len(result))
	}
}

// TestLoadTLDListCustomOverride verifies that a custom TLD list overrides the default.
func TestLoadTLDListCustomOverride(t *testing.T) {
	custom := []string{"example", "test", "local"}
	result := LoadTLDList(custom)
	if len(result) != len(custom) {
		t.Errorf("expected %d custom TLDs, got %d", len(custom), len(result))
	}
	for i, tld := range custom {
		if result[i] != tld {
			t.Errorf("expected TLD[%d] = %q, got %q", i, tld, result[i])
		}
	}
}

// TestCuratedTLDListSize verifies the curated list contains exactly 30 TLDs.
func TestCuratedTLDListSize(t *testing.T) {
	if len(DefaultCuratedTLDs) != 30 {
		t.Errorf("expected 30 curated TLDs, got %d", len(DefaultCuratedTLDs))
	}
}

// TestCuratedTLDListTier1 verifies all required tier-1 TLDs are present.
func TestCuratedTLDListTier1(t *testing.T) {
	tier1 := []string{"com", "cn", "de", "net", "org", "uk", "ru", "nl"}

	// Build a set for fast lookup
	tldSet := make(map[string]bool, len(DefaultCuratedTLDs))
	for _, tld := range DefaultCuratedTLDs {
		tldSet[tld] = true
	}

	for _, required := range tier1 {
		if !tldSet[required] {
			t.Errorf("required tier-1 TLD %q is missing from curated list", required)
		}
	}
}

// TestCuratedTLDListTier2Coverage verifies at least 22 tier-2 TLDs are present.
// Tier-2 TLDs are any TLDs beyond the 8 tier-1 entries.
func TestCuratedTLDListTier2Coverage(t *testing.T) {
	tier1Set := map[string]bool{
		"com": true, "cn": true, "de": true, "net": true,
		"org": true, "uk": true, "ru": true, "nl": true,
	}

	tier2Count := 0
	for _, tld := range DefaultCuratedTLDs {
		if !tier1Set[tld] {
			tier2Count++
		}
	}

	if tier2Count < 22 {
		t.Errorf("expected at least 22 tier-2 TLDs, got %d", tier2Count)
	}
}

// TestCuratedTLDListNoDuplicates verifies no duplicate TLDs in the curated list.
func TestCuratedTLDListNoDuplicates(t *testing.T) {
	seen := make(map[string]bool, len(DefaultCuratedTLDs))
	for _, tld := range DefaultCuratedTLDs {
		if seen[tld] {
			t.Errorf("duplicate TLD found in curated list: %q", tld)
		}
		seen[tld] = true
	}
}

// TestDefaultTLDsAlias verifies DefaultTLDs is the same as DefaultCuratedTLDs.
func TestDefaultTLDsAlias(t *testing.T) {
	if len(DefaultTLDs) != len(DefaultCuratedTLDs) {
		t.Errorf("DefaultTLDs length (%d) != DefaultCuratedTLDs length (%d)", len(DefaultTLDs), len(DefaultCuratedTLDs))
	}
}
