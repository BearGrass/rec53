## Why

`rec53top` has grown from a helper screen into a real product surface, but its documentation is still split across usage notes and dashboard references. We need a single, polished doc set that explains each page, field, and state clearly in both English and Chinese so users can read it like a manual instead of a changelog.

## What Changes

- Create a dedicated `docs/user/rec53top/` directory to group the TUI docs into one product-oriented set.
- Add bilingual docs that cover the overview page, detail pages, field meanings, navigation, self-test flow, observability layout, and operator checklist.
- Keep the tone more product-manual-like: clear, direct, and stable enough to serve as the primary user reference.
- Update top-level README links so readers can enter the TUI doc set from one stable place.

## Capabilities

### New Capabilities
- `rec53top-product-docs`: a grouped bilingual documentation set for the rec53top TUI, including page-by-page and field-by-field explanations plus navigation and operational guidance.

### Modified Capabilities
- `local-ops-tui-docs`: the operational TUI docs need to align with the new product-oriented structure and page/field coverage.
- `rec53top-release-intro`: the release-facing intro should point into the new directory-based doc set and match the new positioning.

## Impact

- `docs/user/` structure and link targets.
- `README.md` and `README.zh.md` navigation for TUI docs.
- Existing TUI documentation pages, especially overview, operations, observability, and checklist references.
- OpenSpec specs and tasks for the TUI documentation set.
