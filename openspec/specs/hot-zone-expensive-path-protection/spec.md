## Purpose

Define how rec53 identifies a single expensive-path hotspot and temporarily protects it under sustained system pressure while preserving cheap-path answers.

## ADDED Requirements

### Requirement: System SHALL protect only one hot zone at a time
The system SHALL identify at most one currently protected hot zone at any given time. The selected hot zone SHALL represent the single most significant hotspot within the current observation windows for expensive-path traffic.

#### Scenario: One hotspot is selected from current windows
- **WHEN** multiple zone candidates exist within the current observation windows
- **THEN** the system selects only one hot zone as the active protected zone

### Requirement: System SHALL derive hot-zone evidence only from expensive-path occupancy
The system SHALL derive hot-zone evidence only from requests that enter an expensive path. Requests satisfied by `hosts`, forwarding hit, or cache hit SHALL NOT contribute to hot-zone occupancy accounting.

#### Scenario: Cheap-path traffic does not build hotspot evidence
- **WHEN** a request is answered entirely by `hosts`, forwarding hit, or cache hit
- **THEN** the request does not contribute to hot-zone occupancy accounting

### Requirement: System SHALL record coarse occupancy using a coarse business-root key during normal operation
During normal operation, the system SHALL record coarse hot-zone occupancy using a coarse business-root key. This key SHALL prefer matched forwarding zones and configured base suffix derivation before falling back to level-3 domains.

#### Scenario: Normal operation records coarse business-root occupancy
- **WHEN** an expensive-path request enters normal hot-zone accounting
- **THEN** the request is initially accounted to its coarse business-root key

### Requirement: System SHALL select business roots by priority
The system SHALL determine the business root for hot-zone accounting in this priority order: matched forwarding zone, configured base suffix set, and finally level-3 fallback.

#### Scenario: Forwarding zone overrides suffix-derived root
- **WHEN** a request matches a configured forwarding zone
- **THEN** that forwarding zone becomes the business root for hot-zone accounting

#### Scenario: Configured base suffix derives the business root
- **WHEN** a request does not match a forwarding zone but does match a configured base suffix
- **THEN** the business root is derived from the first business layer above that base suffix

#### Scenario: Level-3 fallback is used when no higher-priority root exists
- **WHEN** a request matches neither a forwarding zone nor a configured base suffix
- **THEN** the system falls back to level-3 accounting

### Requirement: System SHALL support default base suffixes with additive operator extensions
The system SHALL provide a default base suffix set and SHALL allow operators to append additional suffixes for their deployment environment. The effective suffix set SHALL use longest-suffix match.

The content of the default base suffix set MAY reuse the same public-suffix-like entries already curated for warmup defaults, but this capability SHALL keep its own configuration semantics and SHALL NOT depend on warmup configuration behavior.

#### Scenario: Operator extension adds environment-specific suffix
- **WHEN** an operator appends an environment-specific base suffix
- **THEN** the suffix participates in business-root selection together with the default suffix set

#### Scenario: Built-in defaults remain active when no extension is configured
- **WHEN** the operator does not configure any extra base suffixes
- **THEN** the system still uses the built-in default base suffix set for business-root selection

#### Scenario: Longest suffix match wins among configured suffixes
- **WHEN** multiple configured base suffixes match the same request name
- **THEN** the longest matching suffix is used

### Requirement: System SHALL exclude public-suffix-like level-2 protection targets
The system SHALL NOT select public-suffix-like level-2 keys such as `com.` and `net.` as protected hot zones.

#### Scenario: Public suffix is excluded from protection candidates
- **WHEN** the only shared suffix above observed requests is a level-2 public suffix such as `com.`
- **THEN** that suffix is not selected as a hot-zone candidate

### Requirement: System SHALL enter observe mode only near system pressure limits
The system SHALL enter observe mode only when short-window average expensive concurrency approaches machine concurrency limits and overall machine CPU utilization is simultaneously in a high-pressure range. In the first version, the short window SHALL be `5s`, the expensive-concurrency threshold SHALL be `avg_expensive_concurrency >= 0.75 * NumCPU()`, and the CPU guardrail SHALL be overall machine CPU `>= 70%`.

#### Scenario: Light anomaly does not trigger observe mode
- **WHEN** expensive-path activity increases but average expensive concurrency is still well below machine concurrency limits or CPU is not in a high-pressure range
- **THEN** the system does not enter observe mode

#### Scenario: Near-limit pressure triggers observe mode
- **WHEN** short-window average expensive concurrency approaches machine concurrency limits and overall machine CPU utilization is also in a high-pressure range
- **THEN** the system enters observe mode

### Requirement: System SHALL aggregate recent windows by simple summation
During observe mode, the system SHALL aggregate recent short windows by simple summation of occupancy-time. The system SHALL NOT require weighted or smoothed aggregation for first-version hotspot selection. In the first version, hotspot selection SHALL use the most recent `3` short windows.

#### Scenario: Recent windows are summed directly
- **WHEN** the system evaluates a hotspot candidate during observe mode
- **THEN** it uses simple summed occupancy-time across recent short windows

### Requirement: System SHALL perform at most one level of drill-down during observe mode
During observe mode, the system SHALL examine the current hottest coarse business-root key and SHALL perform at most one level of child drill-down beneath that key. The system SHALL NOT require recursive multi-level descent in the first version.

#### Scenario: Observe mode drills down one level beneath hottest coarse root
- **WHEN** the system has identified the hottest coarse business-root key during observe mode
- **THEN** it evaluates only the direct child layer beneath that key for refined protection selection

### Requirement: System SHALL choose the minimal sufficient hotspot suffix conservatively
The system SHALL choose the minimal sufficient hotspot suffix conservatively. If a child suffix contributes the main portion of its parent's expensive occupancy across recent aggregated windows, the child MAY become the protection candidate. Otherwise, the parent SHALL remain the protection candidate. In the first version, “main portion” SHALL mean at least `80%` of the parent's aggregated occupancy.

#### Scenario: Dominant child becomes the candidate
- **WHEN** a child suffix contributes the main portion of its parent's expensive occupancy across recent aggregated windows
- **THEN** the child suffix becomes the preferred hot-zone candidate instead of the parent

#### Scenario: Parent remains the candidate when no child dominates
- **WHEN** no child suffix contributes the main portion of its parent's expensive occupancy across recent aggregated windows
- **THEN** the parent suffix remains the preferred hot-zone candidate

#### Scenario: Full child dominance stops further drill-down
- **WHEN** one child suffix accounts for all observed occupancy beneath the current parent in the aggregated observe-mode windows
- **THEN** that child becomes the selected protection candidate and no further drill-down is required

### Requirement: System SHALL require consecutive windows before entering protection
The system SHALL require a hot-zone candidate to remain valid across multiple consecutive short observation windows before entering protected status. The purpose of this delay SHALL be to give short-lived startup or initialization bursts time to subside before protection activates. In the first version, the same preferred candidate SHALL remain valid for `3` consecutive observation windows before protection activates.

#### Scenario: Short startup burst does not immediately trigger protection
- **WHEN** a candidate becomes hot in a single short observation window but does not persist across consecutive windows
- **THEN** the system does not yet place that zone into protected status

#### Scenario: Persistent hotspot enters protection
- **WHEN** a candidate remains the preferred hot-zone candidate across the required consecutive observation windows
- **THEN** the system places that zone into protected status

### Requirement: Protected hot zones SHALL block only new expensive-path entry
When a zone is in protected status, the system SHALL reject only new requests that would enter an expensive path for that zone. Requests that can still be answered through `hosts`, forwarding hit, or cache hit SHALL continue to be served. Rejected expensive-path requests SHALL use `REFUSED`.

#### Scenario: Expensive-path entry is rejected for protected zone
- **WHEN** a request for the protected hot zone would need a forwarding external query or iterative/upstream path
- **THEN** the system rejects entry into that expensive path with `REFUSED`

#### Scenario: Protection scope stays at expensive-path admission only
- **WHEN** a hot zone becomes protected
- **THEN** the system only rejects new expensive-path entry and SHALL NOT extend this first-version protection to upstream send concurrency or multi-zone global pressure handling

#### Scenario: Cheap path remains available for protected zone
- **WHEN** a request for the protected hot zone can be answered through `hosts`, forwarding hit, or cache hit
- **THEN** the request continues to be served

### Requirement: Hot-zone protection SHALL not cover multi-zone global pressure
The system SHALL limit this capability to single-hot-zone protection. Pressure distributed across many different zones SHALL NOT be treated as a reason to activate this capability by itself.

#### Scenario: Many zones are busy but no single zone dominates
- **WHEN** expensive-path traffic is distributed across many different zones without a single clear hotspot
- **THEN** this capability does not activate single-hot-zone protection solely on that basis

### Requirement: System SHALL exit protection after pressure returns near the pre-trigger baseline
When protection is active, the system SHALL retain a global expensive-occupancy baseline derived from the short interval immediately before observe mode was triggered. The system SHALL exit protection when current global expensive occupancy returns near that pre-trigger baseline, allowing only a small safety margin above it. In the first version, the exit condition SHALL be `current <= pre-trigger-baseline * 1.05`.

#### Scenario: Protection exits after occupancy returns near baseline
- **WHEN** current global expensive occupancy falls back near the recorded pre-trigger baseline with only a small safety margin
- **THEN** the system exits protection for the currently protected hot zone
