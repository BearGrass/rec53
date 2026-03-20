## 1. Overview Focus And Navigation

- [x] 1.1 Add explicit overview focus state for the six fixed panels
- [x] 1.2 Add keyboard navigation for overview focus via arrow keys, `j/k/l`, and tab-style cycling while keeping `h` for help
- [x] 1.3 Add `Enter` to open the currently focused panel detail while preserving existing numeric shortcuts

## 2. Visual And Help UX

- [x] 2.1 Add a clear but lightweight visual indicator for the focused overview panel
- [x] 2.2 Update footer/help text so current navigation and activation behavior is discoverable
- [x] 2.3 Keep `0` / `Esc` / `q` / `r` / `h` compatibility intact

## 3. Docs And Roadmap

- [x] 3.1 Update roadmap so the active line stays on TUI completion work rather than switching to `v1.2.0`
- [x] 3.2 Update the TUI user guide to document overview focus navigation and detail entry

## 4. Verification

- [x] 4.1 Add focused tests for overview navigation state and Enter-to-detail behavior
- [x] 4.2 Run `go test ./tui`
