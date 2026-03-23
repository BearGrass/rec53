## MODIFIED Requirements

### Requirement: local ops TUI docs SHALL explain default navigation

The operational TUI guide SHALL describe the default navigation path for overview focus, opening detail, moving through any supported drill-down subviews, and returning to overview so the interface can be used without memorizing only numeric shortcuts. If lightweight trend cues are shown, the guide SHALL explain that they represent only recent in-process samples and do not replace Prometheus/Grafana history.

#### Scenario: User reads the navigation section
- **WHEN** a user reads the TUI operational guide
- **THEN** the guide SHALL explain overview focus movement
- **AND** SHALL explain how to open detail from the current focus
- **AND** SHALL explain how to move between drill-down subviews when the current panel supports them
- **AND** SHALL explain the boundary between lightweight trend cues and external historical monitoring
- **AND** SHALL still document the numeric shortcuts as compatible fast paths
