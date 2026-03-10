package utils

import (
	"testing"
)

func TestGetZoneList(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected []string
	}{
		{
			name:   "simple domain",
			domain: "example.com.",
			expected: []string{
				"example.com.",
				"com.",
				"",
			},
		},
		{
			name:   "subdomain",
			domain: "www.example.com.",
			expected: []string{
				"www.example.com.",
				"example.com.",
				"com.",
				"",
			},
		},
		{
			name:   "deep subdomain",
			domain: "a.b.c.example.com.",
			expected: []string{
				"a.b.c.example.com.",
				"b.c.example.com.",
				"c.example.com.",
				"example.com.",
				"com.",
				"",
			},
		},
		{
			name:   "root domain",
			domain: ".",
			expected: []string{
				".",
				"",
			},
		},
		{
			name:   "single label",
			domain: "localhost.",
			expected: []string{
				"localhost.",
				"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetZoneList(tt.domain)

			if len(result) != len(tt.expected) {
				t.Errorf("GetZoneList(%q) returned %d zones, expected %d", tt.domain, len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Expected: %v", tt.expected)
				return
			}

			for i, zone := range result {
				if zone != tt.expected[i] {
					t.Errorf("GetZoneList(%q)[%d] = %q, expected %q", tt.domain, i, zone, tt.expected[i])
				}
			}
		})
	}
}

func TestGetZoneListConsistency(t *testing.T) {
	// Test that calling the function multiple times gives consistent results
	domain := "test.example.com."

	result1 := GetZoneList(domain)
	result2 := GetZoneList(domain)

	if len(result1) != len(result2) {
		t.Error("GetZoneList returned inconsistent lengths")
	}

	for i := range result1 {
		if result1[i] != result2[i] {
			t.Errorf("GetZoneList inconsistent at index %d: %q vs %q", i, result1[i], result2[i])
		}
	}
}