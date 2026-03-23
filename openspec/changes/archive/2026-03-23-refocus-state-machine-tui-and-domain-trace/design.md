## Context

`rec53top` has already grown from a six-panel overview into a local diagnostic TUI with detail pages, drill-down subviews, and short-window trend cues. The current `State Machine` direction attempted to add more value by reconstructing an aggregated "live path" from transition counters, but the product fit is weak: the TUI panel is visually square, the resolver can revisit states, different requests are interleaved in the same window, and scrape windows can cut across in-flight work. Those constraints make a global path visualization both hard to trust and hard to read.

At the same time, the debugging question that users actually care about is usually request-scoped: "what happened to this domain?" That question is better served by a targeted trace/debug interface than by a mixed aggregate view inside the normal overview/detail TUI.

This change therefore spans three areas:

- TUI product semantics and UX
- State-machine aggregate signal selection
- A new domain-scoped trace/debug capability

## Goals / Non-Goals

**Goals:**

- Reframe the `State Machine` panel around aggregate signals that remain readable under concurrent traffic, loops, and partial-window samples.
- Keep `rec53top` focused on local situational awareness rather than request-level forensic reconstruction.
- Define a separate domain-scoped trace capability that can answer the higher-value debugging question of one domain's real resolver path.
- Make the handoff between aggregate TUI diagnosis and targeted trace/debugging explicit in product docs and panel guidance.

**Non-Goals:**

- Do not make `rec53top` a full tracing UI.
- Do not require path conservation or perfectly balanced aggregate graphs inside the TUI.
- Do not commit in this change to one exact operator interface for domain trace if more than one implementation surface remains viable.
- Do not expand the TUI into multi-target, long-history, or heavy graph visualization work.

## Decisions

### 1. The `State Machine` panel will be reduced to counter-oriented aggregate signals

The TUI panel will prioritize recent per-state counters, terminal-exit counters, and a small bounded failure summary.

Why:

- These signals stay legible in a square panel.
- They tolerate loops and mixed concurrent traffic better than a reconstructed global path.
- They match the question an overview panel can answer well: where is heat accumulating, and where are requests ending.

Alternatives considered:

- Keep refining the aggregated path graph. Rejected because the explanation burden remains high even if the rendering improves.
- Show a tree or matrix of aggregate transitions. Rejected for the near term because it still shifts the panel away from fast-glance diagnosis and toward analytical interpretation.

### 2. Request-level path explanation will move to a dedicated domain-scoped trace capability

The system will define a separate capability for requesting the real path of a specified domain or query, including state sequence and terminal result.

Why:

- It answers the operator's real debugging question directly.
- It avoids forcing one UI to serve both aggregate monitoring and per-request tracing.
- It creates a natural bridge from TUI "something is wrong here" to "show me exactly what this domain did."

Alternatives considered:

- Embed one-domain trace directly into the `State Machine` panel. Rejected because it overloads the TUI with request-specific semantics and input workflow.
- Rely on raw logs only. Rejected because the capability should be explicit, bounded, and easier to invoke than manual log parsing.

### 3. The design will keep the trace interface intentionally open at the proposal stage

The change will define the capability and minimum output model first, then evaluate whether the best first implementation is a debug command, a trace-focused log mode, or another targeted operator entrypoint.

Why:

- The product need is clear, but the least-complex interface is not yet settled.
- It allows the proposal/spec layer to align on value before prematurely locking into a UI/API shape.

Alternatives considered:

- Fix the interface now as a TUI subview or one exact CLI command. Rejected because the repo does not yet show one obviously correct surface.

### 4. Docs and panel guidance must explicitly explain the split

The TUI docs and panel text will clearly state:

- aggregate `State Machine` counters explain where mixed traffic is heating up or ending
- domain trace is the tool for "what path did this domain take"

Why:

- Without an explicit split, future iterations will drift back toward overloading the aggregate panel.

## Risks / Trade-offs

- [State Machine becomes less visually ambitious] -> Mitigation: keep the panel intentionally simple and optimize for fast trust rather than novelty.
- [Users may still expect path graphics from recent work] -> Mitigation: update roadmap, TUI docs, and panel wording together so the new scope is explicit.
- [Domain trace interface may still need one more design pass] -> Mitigation: define the minimum capability contract now and leave the exact operator surface as an evaluated implementation choice.
- [Aggregate counters may feel less "diagnostic" than a path graph in some cases] -> Mitigation: preserve failure summaries and clear next-check guidance that points operators toward targeted trace/debugging when aggregate data is insufficient.
