## 1. State Machine TUI Refocus

- [x] 1.1 Remove aggregated live-path/path-graph as the primary `State Machine` detail interpretation and redefine the panel around recent state counters, terminal-exit counters, and bounded failure summaries
- [x] 1.2 Update `tui/` data derivation and rendering so `State Machine` overview/detail surfaces stay readable under mixed concurrent traffic, loops, and partial-window samples
- [x] 1.3 Add or update `tui/` tests to verify the simplified `State Machine` behavior in healthy, degraded, warming, unavailable, stale, and disconnected states

## 2. TUI Product And Docs Alignment

- [x] 2.1 Update `docs/user/local-ops-tui*.md` and `docs/user/rec53top/pages*.md` to explain the new `State Machine` scope as aggregate heat/exit diagnosis rather than request-level path reconstruction
- [x] 2.2 Update roadmap and related TUI-facing docs so future work clearly separates aggregate `rec53top` diagnosis from request-level trace/debugging

## 3. Domain Resolution Trace Capability

- [x] 3.1 Choose and document the minimal operator surface for domain-scoped trace output (for example a debug command, trace-focused log mode, or another dedicated entrypoint) while keeping it separate from the aggregate TUI
- [x] 3.2 Implement bounded trace capture for a specified domain or query so one traced request records its real resolver state sequence and final terminal outcome
- [x] 3.3 Expose the trace result through the chosen operator surface with output that makes the ordered states and terminal result easy to read
- [x] 3.4 Add focused tests covering at least success, upstream-driven failure, and looping or revisited-state cases for the domain-trace flow

## 4. Verification

- [x] 4.1 Run the most relevant `go test` targets for `tui/`, `server/`, and any package used by the new trace entrypoint
- [x] 4.2 Perform one local operator walkthrough that confirms the simplified `State Machine` panel stays readable and that a specified domain can return a real traced path outside the aggregate TUI
