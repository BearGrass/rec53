## 1. Detail Counters

- [x] 1.1 Audit which existing metrics snapshots and per-panel fields can already provide since-start totals and which panels need new bounded cumulative helpers
- [x] 1.2 Add bounded cumulative counter sections to the relevant detail panels without removing the existing current-window standout and next-check structure
- [x] 1.3 Make the detail render structure explicitly separate current-window interpretation from since-start counters

## 2. Tests And Semantics

- [x] 2.1 Add focused tests for cumulative counter rendering and bounded top-N behavior in detail panels
- [x] 2.2 Add regression coverage that current-window standout text remains present after cumulative counters are introduced
- [x] 2.3 Review and tighten per-panel detail semantics so traffic, cache, upstream, and state-machine detail pages stay consistent

## 3. Release-Facing Docs

- [x] 3.1 Add a release-facing TUI introduction document that explains positioning, use cases, boundaries, and doc links
- [x] 3.2 Update README and relevant user docs to link to the new introduction page as the stable entrypoint for TUI overview
- [x] 3.3 Keep `docs/user/local-ops-tui.md` focused on operation, self-test, and detailed usage rather than release-facing overview copy

## 4. Verification

- [x] 4.1 Run the relevant TUI tests and `go vet ./...`
- [x] 4.2 Manually validate the updated detail view against a live local metrics endpoint and confirm the new doc entrypoints read cleanly
