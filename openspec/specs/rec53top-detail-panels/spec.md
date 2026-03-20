# rec53top-detail-panels Specification

## Purpose
TBD - created by archiving change enhance-rec53top-detail-panels. Update Purpose after archive.
## Requirements
### Requirement: rec53top detail panels SHALL add diagnostic value beyond overview

When an operator opens a detail panel in `rec53top`, the detail view SHALL surface information that is materially more diagnostic than the overview card for that domain. It MUST NOT be limited to repeating the same summary metrics with only longer static help text.

#### Scenario: Detail view highlights the current standout condition
- **WHEN** an operator opens a detail panel for a domain that has recent samples
- **THEN** the detail view SHALL identify the current standout condition, dominant signal, or most relevant breakdown for that domain
- **AND** SHALL present that information before or alongside the detailed metric list

#### Scenario: Detail view remains useful when overview is already familiar
- **WHEN** an operator already understands the overview card and opens the corresponding detail panel
- **THEN** the detail view SHALL provide additional diagnostic interpretation or prioritization rather than only a reformatted copy of overview values

### Requirement: rec53top detail panels SHALL guide next investigation steps

Each detail panel SHALL help the operator decide what to inspect next based on the current state of that domain, such as a related panel, logs, or the meaning of the dominant failure category.

#### Scenario: Degraded panel points to the next likely investigation area
- **WHEN** a detail panel is in a degraded or stale state
- **THEN** the detail view SHALL include at least one concrete next-check hint tied to the current dominant signal or failure category

#### Scenario: Healthy panel still explains what to watch
- **WHEN** a detail panel is healthy
- **THEN** the detail view SHALL still summarize what is currently leading the domain and what change would be worth watching next

### Requirement: rec53top detail panels SHALL explain non-normal states explicitly

When a detail panel is warming, unavailable, disabled, disconnected, or stale, the detail view SHALL explain that state explicitly so the operator understands whether the issue is no samples, missing metric families, an intentionally absent feature, or a scrape failure.

#### Scenario: Warming state explains missing short-window meaning
- **WHEN** a detail panel is in `WARMING`
- **THEN** the detail view SHALL explain that recent rate and ratio judgments are not yet stable because the dashboard lacks a prior successful scrape window

#### Scenario: Disabled or unavailable state does not masquerade as empty detail
- **WHEN** a detail panel is in `DISABLED` or `UNAVAILABLE`
- **THEN** the detail view SHALL explain why recent breakdowns are absent
- **AND** SHALL distinguish intentionally disabled capability from missing required metrics

#### Scenario: Stale or disconnected state surfaces scrape context
- **WHEN** a detail panel is in `STALE` or `DISCONNECTED`
- **THEN** the detail view SHALL explain that the panel is not showing fresh live-state interpretation
- **AND** SHALL direct the operator toward connectivity or scrape troubleshooting rather than presenting normal-domain reading guidance

