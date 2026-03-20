## 1. Scope And Scaffolding

- [x] 1.1 Finalize the MVP command shape, directory layout, and default endpoint contract for the local ops TUI
- [x] 1.2 Add the new TUI entrypoint and package scaffolding without changing the main `rec53` server startup path
- [x] 1.3 Add the chosen terminal UI dependency and any supporting flags or config parsing needed for a single-target read-only dashboard
- [x] 1.4 Separate scrape, view-model, and render boundaries so later detail view, drill-down, or history enhancements do not require a rewrite

## 2. Metrics Ingestion And View Model

- [x] 2.1 Implement metrics scraping from a direct `/metric` endpoint with timeout and basic request error handling
- [x] 2.2 Implement parsing and normalization for the MVP metric families needed by traffic, cache, snapshot, upstream, XDP, and state-machine panels
- [x] 2.3 Implement local short-window derivation for rates, ratios, and summary states using consecutive scrape snapshots
- [x] 2.4 Implement explicit view-model states for disconnected targets, unavailable metric families, and XDP disabled or unsupported cases

## 3. TUI Panels And Interaction

- [x] 3.1 Implement the fixed six-panel dashboard layout for traffic, cache, snapshot, upstream, XDP, and state-machine health
- [x] 3.2 Implement refresh lifecycle, resize handling, and minimal key bindings for quit, help, and manual refresh
- [x] 3.3 Implement concise status summaries that highlight healthy, degraded, or unavailable states without requiring PromQL knowledge
- [x] 3.4 Document deferred interaction ideas for post-MVP work, including detail view, drill-down, history sparklines, and panel-level navigation

## 4. Tests And Documentation

- [x] 4.1 Add focused tests for scrape parsing, metric-to-panel mapping, and short-window derived calculations
- [x] 4.2 Add tests for degraded states, including unreachable target, missing metric families, and XDP-disabled snapshots
- [x] 4.3 Write TUI user documentation covering launch, target override, self-test flow, and MVP boundaries
- [x] 4.4 Update README and roadmap references so the TUI entrypoint and `v1.1.2` scope are discoverable

## 5. Verification And Release Readiness

- [x] 5.1 Run targeted tests for the new TUI code and any touched command packages, then fix regressions
- [x] 5.2 Run `gofmt -w .`, `go vet ./...`, and the most relevant `go test` commands for the TUI change
- [x] 5.3 Manually validate the TUI against a live local rec53 instance and capture the expected self-test observations for documentation
