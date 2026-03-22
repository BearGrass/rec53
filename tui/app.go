package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type dashboardUI struct {
	header      *tview.TextView
	traffic     *tview.TextView
	cache       *tview.TextView
	snapshot    *tview.TextView
	upstream    *tview.TextView
	xdp         *tview.TextView
	state       *tview.TextView
	detail      *tview.TextView
	footer      *tview.TextView
	helpShown   bool
	detailPanel detailPanel
	focusPanel  detailPanel
	detailView  map[detailPanel]detailSubview
	history     []Dashboard
	maxHistory  int
	pages       *tview.Pages
	root        tview.Primitive
	refresh     time.Duration
}

type detailPanel string
type detailSubview string

const (
	detailNone     detailPanel = ""
	detailTraffic  detailPanel = "traffic"
	detailCache    detailPanel = "cache"
	detailSnapshot detailPanel = "snapshot"
	detailUpstream detailPanel = "upstream"
	detailXDP      detailPanel = "xdp"
	detailState    detailPanel = "state"
)

const (
	subviewSummary          detailSubview = "summary"
	subviewCacheLookup      detailSubview = "cache-lookup"
	subviewCacheLifecycle   detailSubview = "cache-lifecycle"
	subviewUpstreamFailures detailSubview = "upstream-failures"
	subviewUpstreamWinners  detailSubview = "upstream-winners"
	subviewXDPPacketPaths   detailSubview = "xdp-packet-paths"
	subviewXDPSyncCleanup   detailSubview = "xdp-sync-cleanup"
	subviewStatePathGraph   detailSubview = "state-path-graph"
	subviewStateFailures    detailSubview = "state-failures"
)

type detailSubviewDef struct {
	id    detailSubview
	label string
}

var overviewPanelOrder = []detailPanel{
	detailTraffic,
	detailCache,
	detailSnapshot,
	detailUpstream,
	detailXDP,
	detailState,
}

func Run(ctx context.Context, cfg Config) error {
	cfg = cfg.normalized()
	if cfg.Plain {
		return RunPlain(ctx, cfg)
	}

	app := tview.NewApplication()
	ui := newDashboardUI()
	ui.refresh = cfg.RefreshInterval
	scraper := NewScraper(cfg.Timeout)
	refreshCh := make(chan struct{}, 1)
	var latest Dashboard
	app.SetRoot(ui.layout(), true)

	updateDashboard := func(d Dashboard) {
		ui.pushHistory(d)
		latest = d
		ui.render(d)
	}

	updateDashboard(deriveDashboard(cfg.Target, nil, nil, 0))

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if ui.handleKey(event, latest, func() {
			select {
			case refreshCh <- struct{}{}:
			default:
			}
		}, app.Stop) {
			return nil
		}
		return event
	})

	go func() {
		<-ctx.Done()
		app.Stop()
	}()

	go func() {
		ticker := time.NewTicker(cfg.RefreshInterval)
		defer ticker.Stop()

		var previous *MetricsSnapshot
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-refreshCh:
			}

			result, err := scraper.Scrape(cfg.Target)
			if err != nil {
				app.QueueUpdateDraw(func() {
					updateDashboard(withScrapeError(latest, err))
				})
				continue
			}

			dashboard := deriveDashboard(cfg.Target, previous, result.Snapshot, result.Duration)
			previous = result.Snapshot
			app.QueueUpdateDraw(func() {
				updateDashboard(dashboard)
			})
		}
	}()

	if err := app.Run(); err != nil {
		return fmt.Errorf("%w; retry with -plain or TERM=xterm-256color", err)
	}
	return nil
}

func newDashboardUI() *dashboardUI {
	makePanel := func(title string) *tview.TextView {
		view := tview.NewTextView().SetDynamicColors(true)
		view.SetBorder(true)
		view.SetTitle(" " + title + " ")
		return view
	}

	return &dashboardUI{
		header:     tview.NewTextView().SetDynamicColors(true),
		traffic:    makePanel("Traffic"),
		cache:      makePanel("Cache"),
		snapshot:   makePanel("Snapshot"),
		upstream:   makePanel("Upstream"),
		xdp:        makePanel("XDP"),
		state:      makePanel("State Machine"),
		detail:     makePanel("Detail"),
		footer:     tview.NewTextView().SetDynamicColors(true),
		focusPanel: detailTraffic,
		maxHistory: 16,
		detailView: map[detailPanel]detailSubview{
			detailCache:    subviewSummary,
			detailUpstream: subviewSummary,
			detailXDP:      subviewSummary,
		},
	}
}

func (ui *dashboardUI) handleKey(event *tcell.EventKey, latest Dashboard, refresh func(), stop func()) bool {
	switch event.Key() {
	case tcell.KeyCtrlC:
		if stop != nil {
			stop()
		}
		return true
	case tcell.KeyEscape:
		ui.returnToOverview()
		ui.render(latest)
		return true
	case tcell.KeyEnter:
		if ui.detailPanel == detailNone {
			ui.openFocusedDetail()
			ui.render(latest)
			return true
		}
	case tcell.KeyUp:
		if ui.detailPanel == detailNone && ui.moveFocus(-1, 0) {
			ui.render(latest)
			return true
		}
	case tcell.KeyDown:
		if ui.detailPanel == detailNone && ui.moveFocus(1, 0) {
			ui.render(latest)
			return true
		}
	case tcell.KeyLeft:
		if ui.detailPanel != detailNone && ui.cycleDetailSubview(latest, -1) {
			ui.render(latest)
			return true
		}
		if ui.detailPanel == detailNone && ui.moveFocus(0, -1) {
			ui.render(latest)
			return true
		}
	case tcell.KeyRight:
		if ui.detailPanel != detailNone && ui.cycleDetailSubview(latest, 1) {
			ui.render(latest)
			return true
		}
		if ui.detailPanel == detailNone && ui.moveFocus(0, 1) {
			ui.render(latest)
			return true
		}
	case tcell.KeyTab:
		if ui.detailPanel != detailNone && ui.cycleDetailSubview(latest, 1) {
			ui.render(latest)
			return true
		}
		if ui.detailPanel == detailNone {
			ui.cycleFocus(1)
			ui.render(latest)
			return true
		}
	case tcell.KeyBacktab:
		if ui.detailPanel != detailNone && ui.cycleDetailSubview(latest, -1) {
			ui.render(latest)
			return true
		}
		if ui.detailPanel == detailNone {
			ui.cycleFocus(-1)
			ui.render(latest)
			return true
		}
	}

	switch event.Rune() {
	case 'q':
		if stop != nil {
			stop()
		}
		return true
	case 'r':
		if refresh != nil {
			refresh()
		}
		return true
	case 'h', '?':
		ui.helpShown = !ui.helpShown
		ui.render(latest)
		return true
	case '0', 'o':
		ui.returnToOverview()
		ui.render(latest)
		return true
	case '1':
		ui.openDetail(detailTraffic)
		ui.render(latest)
		return true
	case '2':
		ui.openDetail(detailCache)
		ui.render(latest)
		return true
	case '3':
		ui.openDetail(detailSnapshot)
		ui.render(latest)
		return true
	case '4':
		ui.openDetail(detailUpstream)
		ui.render(latest)
		return true
	case '5':
		ui.openDetail(detailXDP)
		ui.render(latest)
		return true
	case '6':
		ui.openDetail(detailState)
		ui.render(latest)
		return true
	case '[':
		if ui.detailPanel != detailNone && ui.cycleDetailSubview(latest, -1) {
			ui.render(latest)
			return true
		}
	case ']':
		if ui.detailPanel != detailNone && ui.cycleDetailSubview(latest, 1) {
			ui.render(latest)
			return true
		}
	case 'j':
		if ui.detailPanel == detailNone && ui.moveFocus(1, 0) {
			ui.render(latest)
			return true
		}
	case 'k':
		if ui.detailPanel == detailNone && ui.moveFocus(-1, 0) {
			ui.render(latest)
			return true
		}
	case 'l':
		if ui.detailPanel == detailNone && ui.moveFocus(0, 1) {
			ui.render(latest)
			return true
		}
	}

	return false
}

func (ui *dashboardUI) layout() tview.Primitive {
	row1 := tview.NewFlex().
		AddItem(ui.traffic, 0, 1, false).
		AddItem(ui.cache, 0, 1, false)
	row2 := tview.NewFlex().
		AddItem(ui.snapshot, 0, 1, false).
		AddItem(ui.upstream, 0, 1, false)
	row3 := tview.NewFlex().
		AddItem(ui.xdp, 0, 1, false).
		AddItem(ui.state, 0, 1, false)

	overview := tview.NewFlex().SetDirection(tview.FlexRow)
	overview.AddItem(row1, 8, 0, false)
	overview.AddItem(row2, 8, 0, false)
	overview.AddItem(row3, 8, 0, false)

	detailPage := tview.NewFlex().SetDirection(tview.FlexRow)
	detailPage.AddItem(ui.detail, 0, 1, false)

	ui.pages = tview.NewPages().
		AddPage("overview", overview, true, true).
		AddPage("detail", detailPage, true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow)
	root.AddItem(ui.header, 2, 0, false)
	root.AddItem(ui.pages, 0, 1, false)
	root.AddItem(ui.footer, 2, 0, false)
	ui.root = root
	return root
}

func (ui *dashboardUI) render(d Dashboard) {
	modeColor := statusColor(d.Mode)
	header := fmt.Sprintf(
		"[white]rec53top[-] [%s]%s[-] target %s  refresh %s  scrape %s  %s\n%s",
		modeColor,
		d.Mode,
		d.Target,
		formatDurationCompact(ui.refresh),
		formatDurationCompact(d.ScrapeDuration),
		d.LastUpdate.Format("15:04:05"),
		d.OverallSummary,
	)
	if d.LastError != "" {
		header += "\n[red]" + d.LastError + "[-]"
	}
	ui.header.SetText(header)

	ui.traffic.SetText(strings.Join([]string{
		statusLine(d.Traffic.Status),
		fmt.Sprintf("qps            %s", number(d.Traffic.QPS)),
		fmt.Sprintf("p99            %s", latency(d.Traffic.P99MS)),
		fmt.Sprintf("servfail       %s", percent(d.Traffic.ServfailRatio)),
		fmt.Sprintf("nxdomain       %s", percent(d.Traffic.NXDomainRatio)),
		fmt.Sprintf("noerror        %s", percent(d.Traffic.NoErrorRatio)),
	}, "\n"))

	ui.cache.SetText(strings.Join([]string{
		statusLine(d.Cache.Status),
		fmt.Sprintf("hit ratio      %s", percent(d.Cache.HitRatio)),
		fmt.Sprintf("positive hit   %s", rate(d.Cache.PositiveHitRate)),
		fmt.Sprintf("negative hit   %s", rate(d.Cache.NegativeHitRate)),
		fmt.Sprintf("delegation     %s", rate(d.Cache.DelegationRate)),
		fmt.Sprintf("miss           %s", rate(d.Cache.MissRate)),
		fmt.Sprintf("entries        %s", number(d.Cache.Entries)),
		fmt.Sprintf("lifecycle      %s", d.Cache.Lifecycle),
	}, "\n"))

	ui.snapshot.SetText(strings.Join([]string{
		statusLine(d.Snapshot.Status),
		fmt.Sprintf("load success   %s", number(d.Snapshot.LoadSuccess)),
		fmt.Sprintf("load failure   %s", number(d.Snapshot.LoadFailure)),
		fmt.Sprintf("imported       %s", number(d.Snapshot.Imported)),
		fmt.Sprintf("skip expired   %s", number(d.Snapshot.SkippedExpired)),
		fmt.Sprintf("skip corrupt   %s", number(d.Snapshot.SkippedCorrupt)),
		fmt.Sprintf("saved          %s", number(d.Snapshot.SaveSuccess)),
		fmt.Sprintf("duration p99   %s", latency(d.Snapshot.DurationP99MS)),
	}, "\n"))

	ui.upstream.SetText(strings.Join([]string{
		statusLine(d.Upstream.Status),
		fmt.Sprintf("timeout        %s", rate(d.Upstream.TimeoutRate)),
		fmt.Sprintf("bad rcode      %s", rate(d.Upstream.BadRcodeRate)),
		fmt.Sprintf("fallback       %s", rate(d.Upstream.FallbackRate)),
		fmt.Sprintf("winner         %s %s", d.Upstream.Winner, rate(d.Upstream.WinnerRate)),
		fmt.Sprintf("dominant       %s", fallbackText(d.Upstream.DominantReason)),
	}, "\n"))

	ui.xdp.SetText(strings.Join([]string{
		statusLine(d.XDP.Status),
		fmt.Sprintf("mode           %s", d.XDP.Mode),
		fmt.Sprintf("hit ratio      %s", percent(d.XDP.HitRatio)),
		fmt.Sprintf("sync errors    %s", rate(d.XDP.SyncErrorRate)),
		fmt.Sprintf("cleanup        %s", rate(d.XDP.CleanupRate)),
		fmt.Sprintf("entries        %s", number(d.XDP.Entries)),
		fmt.Sprintf("pass           %s", rate(d.XDP.PassRate)),
		fmt.Sprintf("errors         %s", rate(d.XDP.ErrorRate)),
	}, "\n"))

	ui.state.SetText(strings.Join([]string{
		statusLine(d.StateMachine.Status),
		fmt.Sprintf("top stage      %s %s", fallbackText(d.StateMachine.TopStage), rate(d.StateMachine.TopStageRate)),
		fmt.Sprintf("top terminal   %s %s", fallbackText(d.StateMachine.TopTerminal), rate(d.StateMachine.TopTerminalRate)),
		fmt.Sprintf("top failure    %s %s", fallbackText(d.StateMachine.TopFailure), rate(d.StateMachine.TopFailureRate)),
	}, "\n"))

	ui.updateOverviewTitles()
	ui.detail.SetTitle(" " + ui.detailTitle(d) + " ")
	ui.detail.SetText(ui.renderDetail(d))
	if ui.pages != nil {
		if ui.detailPanel == detailNone {
			ui.pages.SwitchToPage("overview")
		} else {
			ui.pages.SwitchToPage("detail")
		}
	}

	ui.footer.SetText(ui.footerText(d))
}

func (ui *dashboardUI) updateOverviewTitles() {
	for _, panel := range overviewPanelOrder {
		view := ui.panelView(panel)
		if view == nil {
			continue
		}
		title := panelTitle(panel)
		if panel == ui.focusedPanel() {
			title = "> " + title + " <"
		}
		view.SetTitle(" " + title + " ")
	}
}

func (ui *dashboardUI) focusLabel() string {
	return panelTitle(ui.focusedPanel())
}

func (ui *dashboardUI) detailTitle(d Dashboard) string {
	if ui.detailPanel == detailNone {
		return "Detail"
	}
	if def, ok := ui.currentDetailSubviewDef(ui.detailPanel, d); ok {
		return fmt.Sprintf("%s [%s]", panelTitle(ui.detailPanel), def.label)
	}
	return panelTitle(ui.detailPanel)
}

func (ui *dashboardUI) renderDetail(d Dashboard) string {
	if defs := ui.detailSubviewDefs(ui.detailPanel, d); len(defs) > 1 {
		subview, _ := ui.currentDetailSubviewDef(ui.detailPanel, d)
		return ui.renderDetailWithTabs(defs, subview.id, ui.renderDetailWithTrendCues(d, ui.renderDetailSubview(d, subview.id)))
	}
	switch ui.detailPanel {
	case detailTraffic:
		return ui.renderDetailWithTrendCues(d, renderDetailModel(buildTrafficDetailModel(d)))
	case detailCache:
		return ui.renderDetailWithTrendCues(d, renderDetailModel(buildCacheDetailModel(d)))
	case detailSnapshot:
		return ui.renderDetailWithTrendCues(d, renderDetailModel(buildSnapshotDetailModel(d)))
	case detailUpstream:
		return ui.renderDetailWithTrendCues(d, renderDetailModel(buildUpstreamDetailModel(d)))
	case detailXDP:
		return ui.renderDetailWithTrendCues(d, renderDetailModel(buildXDPDetailModel(d)))
	case detailState:
		return ui.renderDetailWithTrendCues(d, renderDetailModel(buildStateMachineDetailModel(d)))
	default:
		return "Use arrows, j/k/l, or Tab to focus an overview panel.\nPress Enter to open the focused detail view, or use 1-6 as direct shortcuts.\n\n1 Traffic\n2 Cache\n3 Snapshot\n4 Upstream\n5 XDP\n6 State Machine"
	}
}

func (ui *dashboardUI) renderDetailWithTrendCues(d Dashboard, body string) string {
	trends := ui.renderTrendCues(d)
	if trends == "" {
		return body
	}
	return body + "\n\nRecent trend cues:\n" + trends
}

func (ui *dashboardUI) renderDetailSubview(d Dashboard, subview detailSubview) string {
	switch ui.detailPanel {
	case detailCache:
		switch subview {
		case subviewCacheLookup:
			return renderDetailModel(buildCacheLookupSubviewModel(d))
		case subviewCacheLifecycle:
			return renderDetailModel(buildCacheLifecycleSubviewModel(d))
		default:
			return renderDetailModel(buildCacheDetailModel(d))
		}
	case detailUpstream:
		switch subview {
		case subviewUpstreamFailures:
			return renderDetailModel(buildUpstreamFailuresSubviewModel(d))
		case subviewUpstreamWinners:
			return renderDetailModel(buildUpstreamWinnersSubviewModel(d))
		default:
			return renderDetailModel(buildUpstreamDetailModel(d))
		}
	case detailXDP:
		switch subview {
		case subviewXDPPacketPaths:
			return renderDetailModel(buildXDPPacketPathsSubviewModel(d))
		case subviewXDPSyncCleanup:
			return renderDetailModel(buildXDPSyncCleanupSubviewModel(d))
		default:
			return renderDetailModel(buildXDPDetailModel(d))
		}
	case detailState:
		return renderDetailModel(buildStateMachineDetailModel(d))
	default:
		return ui.renderDetail(d)
	}
}

func (ui *dashboardUI) renderDetailWithTabs(defs []detailSubviewDef, current detailSubview, body string) string {
	tabs := make([]string, 0, len(defs))
	for _, def := range defs {
		if def.id == current {
			tabs = append(tabs, fmt.Sprintf("[yellow]> %s <[-]", def.label))
			continue
		}
		tabs = append(tabs, def.label)
	}
	return fmt.Sprintf("Subview: %s\n\n%s", strings.Join(tabs, "   "), body)
}

func (ui *dashboardUI) openFocusedDetail() {
	ui.detailPanel = ui.focusedPanel()
	ui.resetDetailSubview(ui.detailPanel)
}

func (ui *dashboardUI) openDetail(panel detailPanel) {
	if !isOverviewPanel(panel) {
		return
	}
	ui.focusPanel = panel
	ui.detailPanel = panel
	ui.resetDetailSubview(panel)
}

func (ui *dashboardUI) returnToOverview() {
	ui.detailPanel = detailNone
}

func (ui *dashboardUI) focusedPanel() detailPanel {
	if !isOverviewPanel(ui.focusPanel) {
		return detailTraffic
	}
	return ui.focusPanel
}

func (ui *dashboardUI) cycleFocus(step int) {
	index := overviewPanelIndex(ui.focusedPanel())
	size := len(overviewPanelOrder)
	index = (index + step + size) % size
	ui.focusPanel = overviewPanelOrder[index]
}

func (ui *dashboardUI) moveFocus(rowDelta, colDelta int) bool {
	index := overviewPanelIndex(ui.focusedPanel())
	row := index / 2
	col := index % 2
	nextRow := row + rowDelta
	nextCol := col + colDelta
	if nextRow < 0 || nextRow >= 3 || nextCol < 0 || nextCol >= 2 {
		return false
	}
	ui.focusPanel = overviewPanelOrder[nextRow*2+nextCol]
	return true
}

func (ui *dashboardUI) cycleDetailSubview(d Dashboard, step int) bool {
	defs := ui.detailSubviewDefs(ui.detailPanel, d)
	if len(defs) <= 1 {
		return false
	}
	current, ok := ui.currentDetailSubviewDef(ui.detailPanel, d)
	if !ok {
		current = defs[0]
	}
	index := 0
	for i, def := range defs {
		if def.id == current.id {
			index = i
			break
		}
	}
	size := len(defs)
	index = (index + step + size) % size
	ui.detailView[ui.detailPanel] = defs[index].id
	return true
}

func (ui *dashboardUI) resetDetailSubview(panel detailPanel) {
	if ui.detailView == nil {
		ui.detailView = make(map[detailPanel]detailSubview)
	}
	ui.detailView[panel] = subviewSummary
}

func (ui *dashboardUI) currentDetailSubviewDef(panel detailPanel, d Dashboard) (detailSubviewDef, bool) {
	defs := ui.detailSubviewDefs(panel, d)
	if len(defs) == 0 {
		return detailSubviewDef{}, false
	}
	current := subviewSummary
	if ui.detailView != nil {
		if value, ok := ui.detailView[panel]; ok {
			current = value
		}
	}
	for _, def := range defs {
		if def.id == current {
			return def, true
		}
	}
	return defs[0], true
}

func (ui *dashboardUI) detailSubviewDefs(panel detailPanel, d Dashboard) []detailSubviewDef {
	if !detailDrilldownEnabled(panel, d) {
		return nil
	}
	switch panel {
	case detailCache:
		return []detailSubviewDef{
			{id: subviewSummary, label: "Summary"},
			{id: subviewCacheLookup, label: "Lookup Mix"},
			{id: subviewCacheLifecycle, label: "Lifecycle"},
		}
	case detailUpstream:
		return []detailSubviewDef{
			{id: subviewSummary, label: "Summary"},
			{id: subviewUpstreamFailures, label: "Failures"},
			{id: subviewUpstreamWinners, label: "Winners"},
		}
	case detailXDP:
		return []detailSubviewDef{
			{id: subviewSummary, label: "Summary"},
			{id: subviewXDPPacketPaths, label: "Packet Paths"},
			{id: subviewXDPSyncCleanup, label: "Sync/Cleanup"},
		}
	default:
		return nil
	}
}

func (ui *dashboardUI) footerText(d Dashboard) string {
	if ui.detailPanel != detailNone {
		return ui.detailFooterText(d)
	}
	if ui.helpShown {
		return strings.Join([]string{
			"[yellow]Keys[-] [yellow]q[-] quit  [yellow]r[-] refresh  [yellow]h/?[-] hide  [yellow]enter[-] detail  [yellow]1-6[-] jump  [yellow]0/esc[-] overview",
			fmt.Sprintf("[yellow]Move[-] arrows/jkl/tab  [yellow]Focus[-] %s  [yellow]Status[-] OK DEGRADED DISABLED UNAVAILABLE STALE DISCONNECTED WARMING", ui.focusLabel()),
		}, "\n")
	}
	return fmt.Sprintf("[yellow]q[-] quit  [yellow]r[-] refresh  [yellow]h/?[-] help  [yellow]enter[-] detail  [yellow]1-6[-] jump  [yellow]0/esc[-] overview  [yellow]move[-] arrows/jkl/tab  [yellow]focus[-] %s", ui.focusLabel())
}

func (ui *dashboardUI) detailFooterText(d Dashboard) string {
	if def, ok := ui.currentDetailSubviewDef(ui.detailPanel, d); ok {
		if ui.helpShown {
			return strings.Join([]string{
				"[yellow]Keys[-] [yellow]q[-] quit  [yellow]r[-] refresh  [yellow]h/?[-] hide  [yellow]0/esc[-] overview",
				fmt.Sprintf("[yellow]Detail[-] %s / %s  [yellow]Subview[-] tab shift-tab [ ] left right", panelTitle(ui.detailPanel), def.label),
			}, "\n")
		}
		return fmt.Sprintf("[yellow]q[-] quit  [yellow]r[-] refresh  [yellow]h/?[-] help  [yellow]0/esc[-] overview  [yellow]detail[-] %s / %s  [yellow]subview[-] tab shift-tab [ ] left right", panelTitle(ui.detailPanel), def.label)
	}
	if ui.helpShown {
		return strings.Join([]string{
			"[yellow]Keys[-] [yellow]q[-] quit  [yellow]r[-] refresh  [yellow]h/?[-] hide  [yellow]0/esc[-] overview",
			fmt.Sprintf("[yellow]Detail[-] %s", panelTitle(ui.detailPanel)),
		}, "\n")
	}
	return fmt.Sprintf("[yellow]q[-] quit  [yellow]r[-] refresh  [yellow]h/?[-] help  [yellow]0/esc[-] overview  [yellow]detail[-] %s", panelTitle(ui.detailPanel))
}

func (ui *dashboardUI) pushHistory(d Dashboard) {
	ui.history = append(ui.history, d)
	if ui.maxHistory <= 0 {
		ui.maxHistory = 16
	}
	if len(ui.history) > ui.maxHistory {
		ui.history = append([]Dashboard(nil), ui.history[len(ui.history)-ui.maxHistory:]...)
	}
}

func (ui *dashboardUI) panelView(panel detailPanel) *tview.TextView {
	switch panel {
	case detailTraffic:
		return ui.traffic
	case detailCache:
		return ui.cache
	case detailSnapshot:
		return ui.snapshot
	case detailUpstream:
		return ui.upstream
	case detailXDP:
		return ui.xdp
	case detailState:
		return ui.state
	default:
		return nil
	}
}

func panelTitle(panel detailPanel) string {
	switch panel {
	case detailTraffic:
		return "Traffic"
	case detailCache:
		return "Cache"
	case detailSnapshot:
		return "Snapshot"
	case detailUpstream:
		return "Upstream"
	case detailXDP:
		return "XDP"
	case detailState:
		return "State Machine"
	default:
		return "Detail"
	}
}

func isOverviewPanel(panel detailPanel) bool {
	return overviewPanelIndex(panel) >= 0
}

func overviewPanelIndex(panel detailPanel) int {
	for i, candidate := range overviewPanelOrder {
		if candidate == panel {
			return i
		}
	}
	return -1
}

func detailDrilldownEnabled(panel detailPanel, d Dashboard) bool {
	status := detailPanelStatus(panel, d)
	if status != statusOK && status != statusDegraded {
		return false
	}
	switch panel {
	case detailCache, detailUpstream, detailXDP:
		return true
	default:
		return false
	}
}

func detailPanelStatus(panel detailPanel, d Dashboard) panelStatus {
	switch panel {
	case detailTraffic:
		return d.Traffic.Status
	case detailCache:
		return d.Cache.Status
	case detailSnapshot:
		return d.Snapshot.Status
	case detailUpstream:
		return d.Upstream.Status
	case detailXDP:
		return d.XDP.Status
	case detailState:
		return d.StateMachine.Status
	default:
		return statusUnavailable
	}
}

func (ui *dashboardUI) renderTrendCues(d Dashboard) string {
	if len(ui.history) < 2 {
		return ""
	}
	type cue struct {
		label  string
		values []float64
	}
	cues := make([]cue, 0, 2)
	switch ui.detailPanel {
	case detailCache:
		cues = append(cues,
			cue{label: "hit ratio", values: ui.historyValues(func(item Dashboard) float64 { return item.Cache.HitRatio })},
			cue{label: "miss rate", values: ui.historyValues(func(item Dashboard) float64 { return item.Cache.MissRate })},
		)
	case detailUpstream:
		cues = append(cues,
			cue{label: "timeout", values: ui.historyValues(func(item Dashboard) float64 { return item.Upstream.TimeoutRate })},
			cue{label: "fallback", values: ui.historyValues(func(item Dashboard) float64 { return item.Upstream.FallbackRate })},
		)
	case detailXDP:
		cues = append(cues,
			cue{label: "hit ratio", values: ui.historyValues(func(item Dashboard) float64 { return item.XDP.HitRatio })},
			cue{label: "sync errors", values: ui.historyValues(func(item Dashboard) float64 { return item.XDP.SyncErrorRate })},
		)
	default:
		return ""
	}
	lines := make([]string, 0, len(cues)+1)
	lines = append(lines, "  recent in-process samples only; use Prometheus/Grafana for long-range history")
	for _, trend := range cues {
		if len(trend.values) < 2 {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %-16s %s  %s", trend.label, asciiSparkline(trend.values), trendDirection(trend.values)))
	}
	if len(lines) == 1 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func (ui *dashboardUI) historyValues(extract func(Dashboard) float64) []float64 {
	values := make([]float64, 0, len(ui.history))
	for _, item := range ui.history {
		values = append(values, extract(item))
	}
	return values
}

func asciiSparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	const chars = "._:-=+*#"
	minValue, maxValue := values[0], values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	if maxValue == minValue {
		return strings.Repeat("=", len(values))
	}
	var b strings.Builder
	b.Grow(len(values))
	scale := float64(len(chars) - 1)
	for _, value := range values {
		index := int(((value - minValue) / (maxValue - minValue) * scale) + 0.5)
		if index < 0 {
			index = 0
		}
		if index >= len(chars) {
			index = len(chars) - 1
		}
		b.WriteByte(chars[index])
	}
	return b.String()
}

func trendDirection(values []float64) string {
	if len(values) < 2 {
		return "warming"
	}
	start := values[0]
	end := values[len(values)-1]
	delta := end - start
	threshold := 0.0
	if start != 0 {
		if start < 0 {
			threshold = -start * 0.1
		} else {
			threshold = start * 0.1
		}
	}
	if threshold < 0.05 {
		threshold = 0.05
	}
	switch {
	case delta > threshold:
		return "rising"
	case delta < -threshold:
		return "cooling"
	default:
		return "flat"
	}
}

func renderDetailModel(model detailModel) string {
	lines := []string{
		statusLine(model.Status),
		"",
		"What stands out now:",
		"  " + fallbackText(model.Standout),
	}

	if len(model.CurrentWindowMetrics) > 0 || len(model.CurrentSections) > 0 {
		lines = append(lines, "", "Current window:")
		lines = append(lines, model.CurrentWindowMetrics...)
		for _, section := range model.CurrentSections {
			lines = append(lines, "")
			lines = append(lines, detailSectionLines(section)...)
		}
	}

	if len(model.SinceStartMetrics) > 0 || len(model.SinceStartSections) > 0 {
		lines = append(lines, "", "Since start counters:")
		lines = append(lines, model.SinceStartMetrics...)
		for _, section := range model.SinceStartSections {
			lines = append(lines, "")
			lines = append(lines, detailSectionLines(section)...)
		}
	}

	if len(model.NextChecks) > 0 {
		lines = append(lines, "", "Next checks:")
		for _, check := range model.NextChecks {
			lines = append(lines, "- "+check)
		}
	}

	return strings.Join(lines, "\n")
}

func detailSectionLines(section detailSection) []string {
	lines := []string{section.Title}
	return append(lines, section.Lines...)
}

func statusColor(status panelStatus) string {
	switch status {
	case statusOK:
		return "green"
	case statusDegraded, statusStale:
		return "yellow"
	case statusDisabled:
		return "gray"
	case statusUnavailable, statusDisconnected:
		return "red"
	default:
		return "blue"
	}
}

func statusLine(status panelStatus) string {
	return fmt.Sprintf("status         [%s]%s[-]", statusColor(status), status)
}

func fallbackText(value string) string {
	if value == "" {
		return "--"
	}
	return value
}

func formatDurationCompact(value time.Duration) string {
	if value <= 0 {
		return "--"
	}
	if value < time.Second {
		return fmt.Sprintf("%dms", value.Milliseconds())
	}
	return value.String()
}
