## MODIFIED Requirements

### Requirement: rec53top detail panels SHALL add diagnostic value beyond overview

When an operator opens a detail panel in `rec53top`, the detail view SHALL surface information that is materially more diagnostic than the overview card for that domain. It MUST NOT be limited to repeating the same summary metrics with only longer static help text. For the `State Machine` panel, this additional diagnostic value SHALL include a real path-oriented view derived from transition metrics when those metrics are available.

#### Scenario: Detail view highlights the current standout condition
- **WHEN** an operator opens a detail panel for a domain that has recent samples
- **THEN** the detail view SHALL identify the current standout condition, dominant signal, or most relevant breakdown for that domain
- **AND** SHALL present that information before or alongside the detailed metric list

#### Scenario: Detail view remains useful when overview is already familiar
- **WHEN** an operator already understands the overview card and opens the corresponding detail panel
- **THEN** the detail view SHALL provide additional diagnostic interpretation or prioritization rather than only a reformatted copy of overview values

#### Scenario: State Machine detail shows real path context
- **WHEN** an operator opens the `State Machine` detail panel and transition metrics are available
- **THEN** the detail experience SHALL show the dominant request path, branch points, or terminal exits from real transitions rather than only stage hot spots
