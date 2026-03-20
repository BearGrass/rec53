## ADDED Requirements

### Requirement: rec53top SHALL have a release-facing introduction document

The project SHALL provide a user-facing introduction document for `rec53top` that can be referenced by README, release notes, or other user guidance without requiring the reader to start from implementation-oriented or self-test-heavy documentation.

#### Scenario: Introduction document explains positioning
- **WHEN** a user opens the TUI introduction document
- **THEN** the document SHALL explain what `rec53top` is, what problem it solves, and which deployment or troubleshooting scenarios it targets

#### Scenario: Introduction document explains boundaries
- **WHEN** a user evaluates whether `rec53top` fits their needs
- **THEN** the document SHALL describe the current boundaries such as single-target scope, local terminal focus, and the absence of historical multi-node observability

### Requirement: rec53top introduction SHALL guide users to deeper docs

The introduction document SHALL route users toward the deeper operational and reference documents once they need more detail than the release-facing overview provides.

#### Scenario: User needs operational details
- **WHEN** a reader needs launch flags, self-test steps, or detailed panel-reading guidance
- **THEN** the introduction document SHALL point them to the operational TUI guide and related observability documentation

#### Scenario: README can reference the introduction document
- **WHEN** project-level docs want a concise stable entrypoint for the TUI
- **THEN** the introduction document SHALL be suitable as a primary link target instead of forcing README to link directly into deeper usage-only material
