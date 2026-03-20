package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func RunPlain(ctx context.Context, cfg Config) error {
	return runPlain(ctx, cfg, os.Stdout)
}

func runPlain(ctx context.Context, cfg Config, out io.Writer) error {
	cfg = cfg.normalized()
	scraper := NewScraper(cfg.Timeout)
	ticker := time.NewTicker(cfg.RefreshInterval)
	defer ticker.Stop()

	var previous *MetricsSnapshot
	var latest Dashboard

	printDashboard := func(d Dashboard) error {
		latest = d
		_, err := io.WriteString(out, renderPlainDashboard(d, cfg.RefreshInterval))
		return err
	}

	if err := printDashboard(deriveDashboard(cfg.Target, nil, nil, 0)); err != nil {
		return err
	}

	for {
		result, err := scraper.Scrape(cfg.Target)
		if err != nil {
			if err := printDashboard(withScrapeError(latest, err)); err != nil {
				return err
			}
		} else {
			dashboard := deriveDashboard(cfg.Target, previous, result.Snapshot, result.Duration)
			previous = result.Snapshot
			if err := printDashboard(dashboard); err != nil {
				return err
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func renderPlainDashboard(d Dashboard, refresh time.Duration) string {
	sections := []string{
		fmt.Sprintf("rec53top %s target=%s refresh=%s scrape=%s updated=%s",
			d.Mode, d.Target, formatDurationCompact(refresh), formatDurationCompact(d.ScrapeDuration), d.LastUpdate.Format("15:04:05")),
		d.OverallSummary,
		renderPlainPanel("Traffic", []string{
			fmt.Sprintf("status=%s", d.Traffic.Status),
			fmt.Sprintf("qps=%s", number(d.Traffic.QPS)),
			fmt.Sprintf("p99=%s", latency(d.Traffic.P99MS)),
			fmt.Sprintf("servfail=%s", percent(d.Traffic.ServfailRatio)),
			fmt.Sprintf("nxdomain=%s", percent(d.Traffic.NXDomainRatio)),
			fmt.Sprintf("noerror=%s", percent(d.Traffic.NoErrorRatio)),
		}),
		renderPlainPanel("Cache", []string{
			fmt.Sprintf("status=%s", d.Cache.Status),
			fmt.Sprintf("hit_ratio=%s", percent(d.Cache.HitRatio)),
			fmt.Sprintf("positive_hit=%s", rate(d.Cache.PositiveHitRate)),
			fmt.Sprintf("negative_hit=%s", rate(d.Cache.NegativeHitRate)),
			fmt.Sprintf("delegation=%s", rate(d.Cache.DelegationRate)),
			fmt.Sprintf("miss=%s", rate(d.Cache.MissRate)),
			fmt.Sprintf("entries=%s", number(d.Cache.Entries)),
			fmt.Sprintf("lifecycle=%s", d.Cache.Lifecycle),
		}),
		renderPlainPanel("Snapshot", []string{
			fmt.Sprintf("status=%s", d.Snapshot.Status),
			fmt.Sprintf("load_success=%s", number(d.Snapshot.LoadSuccess)),
			fmt.Sprintf("load_failure=%s", number(d.Snapshot.LoadFailure)),
			fmt.Sprintf("imported=%s", number(d.Snapshot.Imported)),
			fmt.Sprintf("skip_expired=%s", number(d.Snapshot.SkippedExpired)),
			fmt.Sprintf("skip_corrupt=%s", number(d.Snapshot.SkippedCorrupt)),
			fmt.Sprintf("saved=%s", number(d.Snapshot.SaveSuccess)),
			fmt.Sprintf("duration_p99=%s", latency(d.Snapshot.DurationP99MS)),
		}),
		renderPlainPanel("Upstream", []string{
			fmt.Sprintf("status=%s", d.Upstream.Status),
			fmt.Sprintf("timeout=%s", rate(d.Upstream.TimeoutRate)),
			fmt.Sprintf("bad_rcode=%s", rate(d.Upstream.BadRcodeRate)),
			fmt.Sprintf("fallback=%s", rate(d.Upstream.FallbackRate)),
			fmt.Sprintf("winner=%s %s", fallbackText(d.Upstream.Winner), rate(d.Upstream.WinnerRate)),
			fmt.Sprintf("dominant=%s", fallbackText(d.Upstream.DominantReason)),
		}),
		renderPlainPanel("XDP", []string{
			fmt.Sprintf("status=%s", d.XDP.Status),
			fmt.Sprintf("mode=%s", d.XDP.Mode),
			fmt.Sprintf("hit_ratio=%s", percent(d.XDP.HitRatio)),
			fmt.Sprintf("sync_errors=%s", rate(d.XDP.SyncErrorRate)),
			fmt.Sprintf("cleanup=%s", rate(d.XDP.CleanupRate)),
			fmt.Sprintf("entries=%s", number(d.XDP.Entries)),
			fmt.Sprintf("pass=%s", rate(d.XDP.PassRate)),
			fmt.Sprintf("errors=%s", rate(d.XDP.ErrorRate)),
		}),
		renderPlainPanel("State Machine", []string{
			fmt.Sprintf("status=%s", d.StateMachine.Status),
			fmt.Sprintf("top_stage=%s %s", fallbackText(d.StateMachine.TopStage), rate(d.StateMachine.TopStageRate)),
			fmt.Sprintf("fail_top_1=%s %s", fallbackText(d.StateMachine.TopFailure), rate(d.StateMachine.TopFailureRate)),
			fmt.Sprintf("fail_top_2=%s %s", fallbackText(d.StateMachine.SecondFailure), rate(d.StateMachine.SecondFailureRate)),
		}),
	}
	if d.LastError != "" {
		sections = append(sections, "error="+d.LastError)
	}
	return strings.Join(sections, "\n\n") + "\n\n"
}

func renderPlainPanel(title string, lines []string) string {
	return title + "\n" + strings.Join(lines, "\n")
}
