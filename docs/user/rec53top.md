# rec53top Overview

`rec53top` is the local terminal dashboard for rec53. It reads one rec53 Prometheus endpoint directly and turns the current process state into a fixed six-panel view for traffic, cache, snapshot, upstream, XDP, and state-machine behavior.

## What It Is For

`rec53top` is meant for fast local observability when you want signal before a full Prometheus or Grafana workflow exists.

Typical use cases:

- first-pass triage on a development node or lab machine
- local validation after a config change, restart, or upgrade
- quick comparison between current short-window behavior and since-start counters in one terminal
- developer troubleshooting when you need to see which code-path counters are growing without switching straight to raw metrics output

## How To Read It

The overview page answers the first question: which area looks suspicious right now.

The detail page answers the next question: what stands out in the current short window, which cumulative counters have been growing since process start, and where you should check next.

The TUI intentionally keeps those two views separate:

- `Current window` is for recent rates, ratios, and the current standout condition
- `Since start counters` is for bounded cumulative totals and top-N hot paths

This makes it easier to tell whether a problem is still active now or whether you are mostly looking at historical accumulation.

## Current Boundaries

`rec53top` is intentionally narrow in `v1.1.x`:

- single target only
- local terminal only
- read-only dashboard
- no persistent historical storage
- no multi-node or fleet view
- no multi-level drill-down tree beyond one detail page with lightweight subviews for selected panels

It helps you narrow the search space. It does not replace raw `/metric`, logs, Prometheus queries, or future multi-node observability work.

If the TUI shows a short trend cue, treat it as a session-local hint about whether a current signal is rising or cooling. Use Prometheus/Grafana for real historical monitoring.

## Recommended Path

Use `rec53top` in this order:

1. Open the overview and identify the first suspicious panel.
2. Enter detail with `Enter` or `1` to `6`.
3. Start at `Summary`, then switch to a drill-down subview if the current panel offers one.
4. Compare `What stands out now` with the relevant bounded totals or breakdown page.
5. If both point at the same path, go to logs or raw metrics with a much smaller search area.
6. If they diverge, treat the issue as either warming noise, old accumulation, or a regression that may already be fading.

## Where To Go Next

- For running, flags, keys, status states, and local self-test: [Local Ops TUI](local-ops-tui.md)
- For panel layout and related observability material: [Observability Dashboard](observability-dashboard.md)
- For metric-family meanings and raw counter names: [Metrics](../metrics.md)
- For operator triage flow: [Operator Checklist](operator-checklist.md)
