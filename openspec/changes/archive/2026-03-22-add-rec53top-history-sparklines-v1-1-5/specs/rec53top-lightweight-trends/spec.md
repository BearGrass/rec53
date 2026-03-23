## ADDED Requirements

### Requirement: rec53top SHALL keep only lightweight in-process trend history

`rec53top` SHALL limit any trend-oriented history to a short in-process sequence so the TUI can hint at whether a current signal is still rising or already cooling off without becoming a general-purpose historical monitoring frontend.

#### Scenario: Recent trend points are bounded
- **WHEN** `rec53top` collects repeated scrapes during one session
- **THEN** it SHALL keep only a bounded recent sequence of trend points
- **AND** SHALL not require persistent storage or an external timeseries backend

#### Scenario: Trend history resets with the process
- **WHEN** the operator restarts `rec53top`
- **THEN** previously collected trend points SHALL not be reused

### Requirement: rec53top trends SHALL assist current-state interpretation

Trend-oriented output in `rec53top` SHALL exist to help the operator judge whether a currently suspicious signal is still worsening, stabilizing, or fading, rather than to replace Prometheus or Grafana for historical analysis.

#### Scenario: Trend cue supports current diagnosis
- **WHEN** a user reads a detail page with a supported trend cue
- **THEN** the TUI SHALL present the trend as an aid to interpreting the current state
- **AND** SHALL not imply that the TUI is the source of long-term monitoring truth
