## Context

`rec53top` already has separate docs for overview, local operations, observability layout, and operator triage. That split works technically, but it reads like scattered notes rather than a product manual. The new documentation set should feel like one coherent TUI handbook, with the English and Chinese versions kept in parallel.

The user asked whether the TUI docs can live in a dedicated directory. Yes: the cleanest shape is a `docs/user/rec53top/` directory with one stable index and two supporting docs for page/field reference and operational use.

## Goals / Non-Goals

**Goals:**
- Provide a product-style TUI document set in English and Chinese.
- Explain each page, state, and visible field clearly enough to serve as the first reference.
- Group the TUI docs into a dedicated directory with a single entry point.
- Keep links to existing observability and checklist material instead of duplicating every downstream topic.

**Non-Goals:**
- Redesign the TUI itself.
- Rework metric names or dashboard semantics.
- Replace the existing user docs with a totally new information architecture for the rest of the project.

## Decisions

1. **Use a dedicated `docs/user/rec53top/` directory.**
   - Why: the TUI is now large enough to justify a product-style doc bundle.
   - Alternative: keep adding standalone files in `docs/user/`. Rejected because it keeps the reader jumping across unrelated pages.

2. **Split the directory into an index doc, a page/field reference, and an operational guide.**
   - Why: product intro, field reference, and hands-on usage change at different rates.
   - Alternative: one giant document. Rejected because it gets hard to maintain and harder to scan.

3. **Keep bilingual parity file-for-file.**
   - Why: the doc set should be usable directly in both languages without translation lag.
   - Alternative: one bilingual file with mixed sections. Rejected because it is harder to link and harder to keep tone consistent.

4. **Link outward to observability dashboard and operator checklist instead of duplicating them.**
   - Why: those docs are already the canonical references for broader incident handling.
   - Alternative: copy those docs into the TUI bundle. Rejected because it increases drift risk.

## Risks / Trade-offs

- [Drift] Directory structure and cross-links can fall out of sync → keep a single TUI index and update both languages together.
- [Duplication] Page/field wording can drift from the actual TUI → reference the live page names and panel states, not abstract marketing terms.
- [Scope creep] A product manual can become too broad → keep the bundle focused on TUI usage, page meanings, and next-step guidance.

## Migration Plan

1. Create `docs/user/rec53top/README.md` and `README.zh.md` as the product entrypoint.
2. Add `pages.md` / `pages.zh.md` for page-by-page and field-by-field reference.
3. Add `operations.md` / `operations.zh.md` for launch, keys, self-test, and navigation.
4. Update `README.md` and `README.zh.md` to point into the new directory.
5. Keep the existing observability dashboard and checklist docs in place, but link them from the new TUI bundle.

## Open Questions

- Should the existing standalone `docs/user/rec53top.md` and `docs/user/local-ops-tui.md` be kept as redirects or folded into the new directory?
- Should the TUI bundle include the observability dashboard and operator checklist as files inside the directory, or just link to them from the index?
