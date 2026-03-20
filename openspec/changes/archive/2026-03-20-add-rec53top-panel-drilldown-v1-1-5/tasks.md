## 1. Detail Drill-down State

- [x] 1.1 Add detail subview state for supported `Cache`, `Upstream`, and `XDP` panels
- [x] 1.2 Add detail-only navigation for previous/next subview while keeping overview navigation behavior intact

## 2. Drill-down Rendering

- [x] 2.1 Split `Cache` detail into summary and themed drill-down subviews
- [x] 2.2 Split `Upstream` detail into summary and themed drill-down subviews
- [x] 2.3 Split `XDP` detail into summary and themed drill-down subviews
- [x] 2.4 Show current drill-down subview in detail title or footer

## 3. Docs And Roadmap

- [x] 3.1 Update `docs/user/local-ops-tui.md` and `docs/user/rec53top.md` for drill-down usage
- [x] 3.2 Update `docs/roadmap.md` to reflect `v1.1.5` drill-down progress

## 4. Verification

- [x] 4.1 Add focused tests for detail subview navigation and supported-panel behavior
- [x] 4.2 Run `go test ./tui`
