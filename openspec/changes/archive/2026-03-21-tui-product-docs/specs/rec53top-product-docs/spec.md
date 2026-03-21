## ADDED Requirements

### Requirement: rec53top product docs SHALL provide a dedicated directory entrypoint

The project SHALL provide a dedicated documentation directory for the `rec53top` TUI so readers can start from one stable place and move into the overview, operational, and reference docs without guessing where to begin.

#### Scenario: Reader opens the TUI doc entrypoint
- **WHEN** a reader opens the `rec53top` product docs index
- **THEN** the index SHALL explain what the TUI is for
- **AND** SHALL link to the overview, operational guide, and field reference

### Requirement: rec53top product docs SHALL explain each page and field

The TUI product docs SHALL describe each page, panel, state, and visible field in a way that helps users understand what the screen means, not just which keys to press.

#### Scenario: Reader checks a specific panel or field
- **WHEN** a reader looks for the meaning of a page, panel, or field name shown in `rec53top`
- **THEN** the docs SHALL explain that item in product-manual style
- **AND** SHALL indicate which state or metric family it comes from when relevant

### Requirement: rec53top product docs SHALL stay bilingual

The TUI product docs SHALL be maintained in English and Chinese with matching scope so readers can use either language without losing coverage of the same pages and fields.

#### Scenario: Reader switches languages
- **WHEN** a reader opens the English or Chinese TUI docs
- **THEN** both versions SHALL cover the same user-facing content
- **AND** SHALL use equivalent link structure within the TUI doc directory

### Requirement: rec53top product docs SHALL preserve operational handoff paths

The TUI product docs SHALL still point readers to the observability dashboard, operator checklist, metrics docs, and logs when they need broader incident investigation beyond the TUI itself.

#### Scenario: Reader needs deeper incident analysis
- **WHEN** a reader reaches the limit of what the TUI can explain
- **THEN** the docs SHALL direct them to the broader observability and troubleshooting references
- **AND** SHALL make clear that the TUI is a diagnostic entrypoint, not the only source of truth
