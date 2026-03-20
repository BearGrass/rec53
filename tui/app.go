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
	pages       *tview.Pages
	root        tview.Primitive
	refresh     time.Duration
}

type detailPanel string

const (
	detailNone     detailPanel = ""
	detailTraffic  detailPanel = "traffic"
	detailCache    detailPanel = "cache"
	detailSnapshot detailPanel = "snapshot"
	detailUpstream detailPanel = "upstream"
	detailXDP      detailPanel = "xdp"
	detailState    detailPanel = "state"
)

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
		if ui.detailPanel == detailNone && ui.moveFocus(0, -1) {
			ui.render(latest)
			return true
		}
	case tcell.KeyRight:
		if ui.detailPanel == detailNone && ui.moveFocus(0, 1) {
			ui.render(latest)
			return true
		}
	case tcell.KeyTab:
		if ui.detailPanel == detailNone {
			ui.cycleFocus(1)
			ui.render(latest)
			return true
		}
	case tcell.KeyBacktab:
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
		fmt.Sprintf("fail top 1     %s %s", fallbackText(d.StateMachine.TopFailure), rate(d.StateMachine.TopFailureRate)),
		fmt.Sprintf("fail top 2     %s %s", fallbackText(d.StateMachine.SecondFailure), rate(d.StateMachine.SecondFailureRate)),
	}, "\n"))

	ui.updateOverviewTitles()
	ui.detail.SetTitle(" " + ui.detailTitle() + " ")
	ui.detail.SetText(ui.renderDetail(d))
	if ui.pages != nil {
		if ui.detailPanel == detailNone {
			ui.pages.SwitchToPage("overview")
		} else {
			ui.pages.SwitchToPage("detail")
		}
	}

	if ui.helpShown {
		ui.footer.SetText(fmt.Sprintf("[yellow]q[-] quit   [yellow]r[-] refresh   [yellow]h/?[-] hide help   [yellow]arrows/jkl/tab[-] move   [yellow]enter[-] detail   [yellow]1-6[-] jump   [yellow]0/esc[-] overview   focus %s   statuses: OK / DEGRADED / DISABLED / UNAVAILABLE / STALE / DISCONNECTED / WARMING", ui.focusLabel()))
	} else {
		ui.footer.SetText(fmt.Sprintf("[yellow]q[-] quit   [yellow]r[-] refresh   [yellow]h/?[-] help   [yellow]arrows/jkl/tab[-] move   [yellow]enter[-] detail   [yellow]1-6[-] jump   [yellow]0/esc[-] overview   focus %s", ui.focusLabel()))
	}
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

func (ui *dashboardUI) detailTitle() string {
	if ui.detailPanel == detailNone {
		return "Detail"
	}
	return panelTitle(ui.detailPanel) + " Detail"
}

func (ui *dashboardUI) renderDetail(d Dashboard) string {
	switch ui.detailPanel {
	case detailTraffic:
		return renderDetailModel(buildTrafficDetailModel(d))
	case detailCache:
		return renderDetailModel(buildCacheDetailModel(d))
	case detailSnapshot:
		return renderDetailModel(buildSnapshotDetailModel(d))
	case detailUpstream:
		return renderDetailModel(buildUpstreamDetailModel(d))
	case detailXDP:
		return renderDetailModel(buildXDPDetailModel(d))
	case detailState:
		return renderDetailModel(buildStateMachineDetailModel(d))
	default:
		return "Use arrows, j/k/l, or Tab to focus an overview panel.\nPress Enter to open the focused detail view, or use 1-6 as direct shortcuts.\n\n1 Traffic\n2 Cache\n3 Snapshot\n4 Upstream\n5 XDP\n6 State Machine"
	}
}

func (ui *dashboardUI) openFocusedDetail() {
	ui.detailPanel = ui.focusedPanel()
}

func (ui *dashboardUI) openDetail(panel detailPanel) {
	if !isOverviewPanel(panel) {
		return
	}
	ui.focusPanel = panel
	ui.detailPanel = panel
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
