package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestDashboardUIRenderBeforeLayoutDoesNotPanic(t *testing.T) {
	ui := newDashboardUI()
	ui.refresh = (Config{}).normalized().RefreshInterval

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("render panicked before layout: %v", r)
		}
	}()

	ui.render(deriveDashboard(DefaultTarget, nil, nil, 0))
}

func TestDashboardUIOverviewFocusNavigation(t *testing.T) {
	ui := newDashboardUI()
	latest := deriveDashboard(DefaultTarget, nil, nil, 0)

	if ui.focusedPanel() != detailTraffic {
		t.Fatalf("initial focus = %q, want %q", ui.focusedPanel(), detailTraffic)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, 0), latest, nil, nil)
	if ui.focusedPanel() != detailCache {
		t.Fatalf("focus after right = %q, want %q", ui.focusedPanel(), detailCache)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0), latest, nil, nil)
	if ui.focusedPanel() != detailUpstream {
		t.Fatalf("focus after down = %q, want %q", ui.focusedPanel(), detailUpstream)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'k', 0), latest, nil, nil)
	if ui.focusedPanel() != detailCache {
		t.Fatalf("focus after k = %q, want %q", ui.focusedPanel(), detailCache)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, 0), latest, nil, nil)
	if ui.focusedPanel() != detailSnapshot {
		t.Fatalf("focus after tab = %q, want %q", ui.focusedPanel(), detailSnapshot)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyBacktab, 0, 0), latest, nil, nil)
	if ui.focusedPanel() != detailCache {
		t.Fatalf("focus after shift-tab = %q, want %q", ui.focusedPanel(), detailCache)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'l', 0), latest, nil, nil)
	if ui.focusedPanel() != detailCache {
		t.Fatalf("focus after l on right edge = %q, want unchanged %q", ui.focusedPanel(), detailCache)
	}
}

func TestDashboardUIEnterOpensFocusedDetailAndPreservesFocus(t *testing.T) {
	ui := newDashboardUI()
	latest := deriveDashboard(DefaultTarget, nil, nil, 0)

	ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'j', 0), latest, nil, nil)
	ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'l', 0), latest, nil, nil)
	if ui.focusedPanel() != detailUpstream {
		t.Fatalf("focus before enter = %q, want %q", ui.focusedPanel(), detailUpstream)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0), latest, nil, nil)
	if ui.detailPanel != detailUpstream {
		t.Fatalf("detail after enter = %q, want %q", ui.detailPanel, detailUpstream)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0), latest, nil, nil)
	if ui.detailPanel != detailNone {
		t.Fatalf("detail after esc = %q, want overview", ui.detailPanel)
	}
	if ui.focusedPanel() != detailUpstream {
		t.Fatalf("focus after return = %q, want %q", ui.focusedPanel(), detailUpstream)
	}
}

func TestDashboardUINumericShortcutsKeepFocusInSync(t *testing.T) {
	ui := newDashboardUI()
	latest := deriveDashboard(DefaultTarget, nil, nil, 0)

	ui.handleKey(tcell.NewEventKey(tcell.KeyRune, '5', 0), latest, nil, nil)
	if ui.detailPanel != detailXDP {
		t.Fatalf("detail after 5 = %q, want %q", ui.detailPanel, detailXDP)
	}
	if ui.focusedPanel() != detailXDP {
		t.Fatalf("focus after 5 = %q, want %q", ui.focusedPanel(), detailXDP)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyRune, '0', 0), latest, nil, nil)
	if ui.detailPanel != detailNone {
		t.Fatalf("detail after 0 = %q, want overview", ui.detailPanel)
	}
	if ui.focusedPanel() != detailXDP {
		t.Fatalf("focus after 0 = %q, want %q", ui.focusedPanel(), detailXDP)
	}
}

func TestDashboardUIHelpKeyCompatibility(t *testing.T) {
	ui := newDashboardUI()
	latest := deriveDashboard(DefaultTarget, nil, nil, 0)

	ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'h', 0), latest, nil, nil)
	if !ui.helpShown {
		t.Fatal("help should be shown after h")
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'h', 0), latest, nil, nil)
	if ui.helpShown {
		t.Fatal("help should be hidden after second h")
	}
}

func TestDashboardUIDetailSubviewNavigation(t *testing.T) {
	ui := newDashboardUI()
	latest := Dashboard{
		Cache: CachePanel{Status: statusOK},
	}
	ui.openDetail(detailCache)

	if got := ui.detailView[detailCache]; got != subviewSummary {
		t.Fatalf("initial cache subview = %q, want %q", got, subviewSummary)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, 0), latest, nil, nil)
	if got := ui.detailView[detailCache]; got != subviewCacheLookup {
		t.Fatalf("cache subview after right = %q, want %q", got, subviewCacheLookup)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, 0), latest, nil, nil)
	if got := ui.detailView[detailCache]; got != subviewCacheLifecycle {
		t.Fatalf("cache subview after tab = %q, want %q", got, subviewCacheLifecycle)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyBacktab, 0, 0), latest, nil, nil)
	if got := ui.detailView[detailCache]; got != subviewCacheLookup {
		t.Fatalf("cache subview after shift-tab = %q, want %q", got, subviewCacheLookup)
	}

	ui.handleKey(tcell.NewEventKey(tcell.KeyRune, '[', 0), latest, nil, nil)
	if got := ui.detailView[detailCache]; got != subviewSummary {
		t.Fatalf("cache subview after [ = %q, want %q", got, subviewSummary)
	}
}

func TestDashboardUIDetailSubviewNavigationDisabledForUnsupportedPanels(t *testing.T) {
	ui := newDashboardUI()
	latest := Dashboard{
		Traffic: TrafficPanel{Status: statusOK},
	}
	ui.openDetail(detailTraffic)

	if ui.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, 0), latest, nil, nil) {
		t.Fatal("right should not be consumed for unsupported detail drilldown")
	}
}

func TestDashboardUIPushHistoryIsBounded(t *testing.T) {
	ui := newDashboardUI()
	ui.maxHistory = 3

	for i := 0; i < 5; i++ {
		ui.pushHistory(Dashboard{
			Cache: CachePanel{HitRatio: float64(i)},
		})
	}

	if len(ui.history) != 3 {
		t.Fatalf("history len = %d, want 3", len(ui.history))
	}
	if ui.history[0].Cache.HitRatio != 2 || ui.history[2].Cache.HitRatio != 4 {
		t.Fatalf("history retained wrong range: %+v", ui.history)
	}
}

func TestASCIISparklineAndDirection(t *testing.T) {
	spark := asciiSparkline([]float64{1, 2, 3, 4, 5})
	if len(spark) != 5 {
		t.Fatalf("sparkline len = %d, want 5", len(spark))
	}
	if trendDirection([]float64{1, 2, 3, 4, 5}) != "rising" {
		t.Fatalf("expected rising direction")
	}
	if trendDirection([]float64{5, 4, 3, 2, 1}) != "cooling" {
		t.Fatalf("expected cooling direction")
	}
	if trendDirection([]float64{1, 1.01, 0.99, 1.0}) != "flat" {
		t.Fatalf("expected flat direction")
	}
}
