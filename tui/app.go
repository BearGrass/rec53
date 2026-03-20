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
		switch event.Key() {
		case tcell.KeyCtrlC:
			app.Stop()
			return nil
		case tcell.KeyEscape:
			ui.detailPanel = detailNone
			ui.render(latest)
			return nil
		}
		switch event.Rune() {
		case 'q':
			app.Stop()
			return nil
		case 'r':
			select {
			case refreshCh <- struct{}{}:
			default:
			}
			return nil
		case 'h', '?':
			ui.helpShown = !ui.helpShown
			ui.render(latest)
			return nil
		case '0', 'o':
			ui.detailPanel = detailNone
			ui.render(latest)
			return nil
		case '1':
			ui.detailPanel = detailTraffic
			ui.render(latest)
			return nil
		case '2':
			ui.detailPanel = detailCache
			ui.render(latest)
			return nil
		case '3':
			ui.detailPanel = detailSnapshot
			ui.render(latest)
			return nil
		case '4':
			ui.detailPanel = detailUpstream
			ui.render(latest)
			return nil
		case '5':
			ui.detailPanel = detailXDP
			ui.render(latest)
			return nil
		case '6':
			ui.detailPanel = detailState
			ui.render(latest)
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
		header:   tview.NewTextView().SetDynamicColors(true),
		traffic:  makePanel("Traffic"),
		cache:    makePanel("Cache"),
		snapshot: makePanel("Snapshot"),
		upstream: makePanel("Upstream"),
		xdp:      makePanel("XDP"),
		state:    makePanel("State Machine"),
		detail:   makePanel("Detail"),
		footer:   tview.NewTextView().SetDynamicColors(true),
	}
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
		ui.footer.SetText("[yellow]q[-] quit   [yellow]r[-] refresh   [yellow]h[-] hide help   [yellow]1-6[-] detail   [yellow]0/esc[-] overview   statuses: OK / DEGRADED / DISABLED / UNAVAILABLE / STALE")
	} else {
		ui.footer.SetText("[yellow]q[-] quit   [yellow]r[-] refresh   [yellow]h[-] help   [yellow]1-6[-] detail   [yellow]0[-] overview")
	}
}

func (ui *dashboardUI) detailTitle() string {
	switch ui.detailPanel {
	case detailTraffic:
		return "Traffic Detail"
	case detailCache:
		return "Cache Detail"
	case detailSnapshot:
		return "Snapshot Detail"
	case detailUpstream:
		return "Upstream Detail"
	case detailXDP:
		return "XDP Detail"
	case detailState:
		return "State Machine Detail"
	default:
		return "Detail"
	}
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
		return "Press 1-6 to open a panel detail view.\n\n1 Traffic\n2 Cache\n3 Snapshot\n4 Upstream\n5 XDP\n6 State Machine"
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
