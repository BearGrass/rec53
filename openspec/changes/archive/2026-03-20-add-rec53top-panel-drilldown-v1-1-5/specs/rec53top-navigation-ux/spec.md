## MODIFIED Requirements

### Requirement: rec53top SHALL clarify active navigation in footer/help

The TUI SHALL present the current focus and available primary actions clearly enough that users can discover the overview-to-detail workflow from the interface itself. When a detail page supports drill-down subviews, the interface SHALL also make the current subview and subview-navigation actions understandable from the screen itself.

#### Scenario: Footer reflects navigation options
- **WHEN** the TUI renders the footer/help text
- **THEN** it SHALL mention the primary navigation and activation actions
- **AND** SHALL make the current focus understandable without requiring the reader to infer it indirectly

#### Scenario: Detail footer reflects drill-down navigation
- **WHEN** the user is on a detail page that supports drill-down subviews
- **THEN** the footer or title SHALL indicate the current subview
- **AND** SHALL mention how to move to the previous or next subview
