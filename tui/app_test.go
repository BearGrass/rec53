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
