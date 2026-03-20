## 1. Detail View-Model

- [x] 1.1 Audit the current `Dashboard` and per-panel structs to identify which detail judgments are already derivable and which additional derived fields or helpers are needed
- [x] 1.2 Add bounded detail-oriented derivation for standout condition, dominant breakdown, and next-check hints without changing rec53 metrics contracts
- [x] 1.3 Add explicit detail-state interpretation for `WARMING`, `UNAVAILABLE`, `DISABLED`, `DISCONNECTED`, and `STALE` so those views remain informative

## 2. Detail Rendering

- [x] 2.1 Refactor `renderDetail` so each panel follows a consistent structure: status, standout condition, key metrics, top breakdowns, and next checks
- [x] 2.2 Replace or shrink the current static `Reading guide` sections with state-aware diagnostic text that reflects the current panel state
- [x] 2.3 Ensure each detail panel adds diagnostic value beyond overview rather than only repeating the same metric summary lines

## 3. Verification And Docs

- [x] 3.1 Add focused tests for detail rendering and/or detail derivation covering healthy, degraded, and non-normal states
- [x] 3.2 Add regression coverage for panels whose current detail output is mostly duplicated summary content, especially traffic, cache, upstream, and state machine
- [x] 3.3 Update local TUI documentation to explain what the enhanced detail view now shows and how operators should use it during troubleshooting
- [x] 3.4 Run the relevant TUI tests, `go vet ./...`, and a manual `rec53top` validation pass against a live local metrics endpoint
