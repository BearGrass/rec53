package tui

import "testing"

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
