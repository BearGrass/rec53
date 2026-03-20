# rec53top-navigation-ux Specification

## Purpose
TBD - created by archiving change enhance-rec53top-v1-1-4. Update Purpose after archive.

## Requirements
### Requirement: rec53top SHALL support overview focus navigation

`rec53top` SHALL provide a visible current focus in the overview dashboard so users can navigate panels without relying only on memorized numeric shortcuts.

#### Scenario: User moves focus in overview
- **WHEN** the user is on the overview page
- **THEN** the TUI SHALL show one currently focused panel
- **AND** SHALL allow moving that focus with navigation keys such as arrow keys, `j/k/l`, or tab-style cycling

#### Scenario: Focus remains stable after returning from detail
- **WHEN** the user opens a detail page from the focused overview panel and then returns to overview
- **THEN** the same panel SHALL remain focused

### Requirement: rec53top SHALL allow opening detail from current focus

The TUI SHALL let users enter the detail page of the currently focused panel via a generic action instead of requiring only panel-number hotkeys.

#### Scenario: Enter opens focused panel detail
- **WHEN** the user is on the overview page and activates the focused panel
- **THEN** the TUI SHALL open that panel's detail page

#### Scenario: Numeric shortcuts remain valid
- **WHEN** the user presses the existing numeric detail shortcuts
- **THEN** the TUI SHALL continue to open the corresponding detail pages

### Requirement: rec53top SHALL clarify active navigation in footer/help

The TUI SHALL present the current focus and available primary actions clearly enough that users can discover the overview-to-detail workflow from the interface itself.

#### Scenario: Footer reflects navigation options
- **WHEN** the TUI renders the footer/help text
- **THEN** it SHALL mention the primary navigation and activation actions
- **AND** SHALL make the current focus understandable without requiring the reader to infer it indirectly
