## ADDED Requirements

### Requirement: rec53top detail panels SHALL show bounded cumulative counters

`rec53top` detail panels SHALL provide a bounded since-start counter view alongside the existing current-window interpretation so that operators and developers can compare “what is happening now” with “what has accumulated over process lifetime”.

#### Scenario: Detail panel shows both current-window and cumulative signals
- **WHEN** an operator opens a detail panel with available metrics
- **THEN** the detail view SHALL continue to show the current short-window interpretation
- **AND** SHALL also show a bounded cumulative counter section derived from the same metrics families

#### Scenario: Label-heavy counters remain bounded
- **WHEN** a detail panel exposes cumulative counters with labels such as response codes, cache results, failure reasons, or stages
- **THEN** the detail view SHALL show only a bounded top-N subset rather than an unbounded full label dump

### Requirement: rec53top detail panels SHALL separate current and cumulative meaning

The TUI SHALL make it clear which detail values describe the current recent window and which describe since-start totals so users do not confuse long-lived counters with immediate regressions.

#### Scenario: Current and cumulative sections are visually distinct
- **WHEN** a detail panel includes both recent-window and since-start information
- **THEN** the panel SHALL label or structure those sections so their time meaning is explicit

#### Scenario: Cumulative counters do not replace current diagnosis
- **WHEN** a detail panel includes cumulative counters
- **THEN** the panel SHALL still preserve the current-window standout condition and next-check guidance rather than downgrading into a raw counter dump
