## 1. Trend Scope

- [x] 1.1 Add bounded in-process trend storage for selected derived metrics
- [x] 1.2 Keep history retention explicitly small and session-local

## 2. TUI Rendering

- [x] 2.1 Choose the first detail metrics that should show lightweight trend cues
- [x] 2.2 Render lightweight trend cues without turning overview into a historical dashboard

## 3. Docs

- [x] 3.1 Update TUI docs to explain the boundary between lightweight trend cues and Prometheus/Grafana history
- [x] 3.2 Update roadmap wording from “history sparklines” to lightweight trend cues where appropriate

## 4. Verification

- [x] 4.1 Add focused tests for bounded trend retention and rendering
- [x] 4.2 Run `go test ./tui`
