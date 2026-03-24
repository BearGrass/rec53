package server

import (
	"testing"
	"time"
)

func TestHotZoneCoarseKeyPriority(t *testing.T) {
	base := normalizeHotZoneBaseSuffixes([]string{"svc.cluster.local", "cluster.local"})

	if got := hotZoneCoarseKey("api.dev.corp.test.", "corp.test.", base); got != "corp.test." {
		t.Fatalf("forwarding coarse key = %q, want corp.test.", got)
	}
	if got := hotZoneCoarseKey("api.ns1.svc.cluster.local.", "", base); got != "ns1.svc.cluster.local." {
		t.Fatalf("base suffix coarse key = %q, want ns1.svc.cluster.local.", got)
	}
	if got := hotZoneCoarseKey("www.deep.example.invalid.", "", base); got != "example.invalid." {
		t.Fatalf("fallback coarse key = %q, want example.invalid.", got)
	}
}

func TestHotZoneMatchedBaseSuffixLongestWins(t *testing.T) {
	base := normalizeHotZoneBaseSuffixes([]string{"cluster.local", "svc.cluster.local"})
	if got := hotZoneMatchedBaseSuffix("api.ns1.svc.cluster.local.", base); got != "svc.cluster.local." {
		t.Fatalf("matched suffix = %q, want svc.cluster.local.", got)
	}
}

func TestHotZoneSnapshotSelectCandidatePrefersDominantChild(t *testing.T) {
	s := hotZoneSnapshot{
		aggregatedCoarse: map[string]time.Duration{
			"example.com.": 10 * time.Second,
		},
		aggregatedChild: map[string]map[string]time.Duration{
			"example.com.": {
				"api.example.com.": 9 * time.Second,
				"www.example.com.": time.Second,
			},
		},
	}
	if got := s.selectCandidate(); got != "api.example.com." {
		t.Fatalf("candidate = %q, want api.example.com.", got)
	}
}

func TestHotZoneSnapshotSelectCandidateKeepsParentWithoutDominantChild(t *testing.T) {
	s := hotZoneSnapshot{
		aggregatedCoarse: map[string]time.Duration{
			"example.com.": 10 * time.Second,
		},
		aggregatedChild: map[string]map[string]time.Duration{
			"example.com.": {
				"api.example.com.": 7 * time.Second,
				"www.example.com.": 3 * time.Second,
			},
		},
	}
	if got := s.selectCandidate(); got != "example.com." {
		t.Fatalf("candidate = %q, want example.com.", got)
	}
}

func TestHotZoneControllerProtectsAfterConsecutiveWindows(t *testing.T) {
	controller := newHotZoneController(nil)
	now := time.Unix(0, 0)
	controller.nowFn = func() time.Time { return now }
	controller.cpuUsageFn = func() float64 { return 95 }
	controller.numCPUFn = func() int { return 1 }
	controller.evalEvery = 0

	controller.mu.Lock()
	controller.windows = []hotZoneWindow{
		{
			start:           time.Unix(0, 0),
			globalOccupancy: 4 * time.Second,
			coarseOccupancy: map[string]time.Duration{"example.com.": 4 * time.Second},
			childOccupancy: map[string]map[string]time.Duration{
				"example.com.": {"api.example.com.": 4 * time.Second},
			},
		},
	}
	now = time.Unix(4, 0)
	controller.evaluateLocked(now)
	controller.windows = append(controller.windows, hotZoneWindow{
		start:           time.Unix(5, 0),
		globalOccupancy: 4 * time.Second,
		coarseOccupancy: map[string]time.Duration{"example.com.": 4 * time.Second},
		childOccupancy: map[string]map[string]time.Duration{
			"example.com.": {"api.example.com.": 4 * time.Second},
		},
	})
	now = time.Unix(9, 0)
	controller.evaluateLocked(now)
	controller.windows = append(controller.windows, hotZoneWindow{
		start:           time.Unix(10, 0),
		globalOccupancy: 4 * time.Second,
		coarseOccupancy: map[string]time.Duration{"example.com.": 4 * time.Second},
		childOccupancy: map[string]map[string]time.Duration{
			"example.com.": {"api.example.com.": 4 * time.Second},
		},
	})
	now = time.Unix(14, 0)
	controller.evaluateLocked(now)
	controller.mu.Unlock()

	if controller.protectedZone != "api.example.com." {
		t.Fatalf("protected zone = %q, want api.example.com.", controller.protectedZone)
	}
	controller.mu.Lock()
	controller.preTriggerBaseline = 0.1
	controller.mu.Unlock()

	allowed, _ := controller.TryEnter("www.api.example.com.", "")
	if allowed {
		t.Fatal("expected protected child zone to refuse expensive entry")
	}
	allowed, id := controller.TryEnter("www.other.example.com.", "")
	if !allowed {
		t.Fatal("expected sibling zone to stay allowed")
	}
	controller.Release(id)

	now = now.Add(6 * time.Second)
	controller.cpuUsageFn = func() float64 { return 10 }
	controller.mu.Lock()
	controller.preTriggerBaseline = 1
	controller.mu.Unlock()
	controller.mu.Lock()
	controller.evaluateLocked(now)
	controller.mu.Unlock()
	if controller.protectedZone != "" {
		t.Fatalf("protected zone = %q, want cleared after exit", controller.protectedZone)
	}
}
