# Proposal

## Why

rec53 already limits expensive requests per client and protects hot zones, but the resolver still has no first-version control for total outbound upstream pressure. The current upstream path can fan out through Happy Eyeballs races, forwarding queries, and concurrent NS resolution; under sustained pressure, that fanout can turn external slowness into local goroutine growth, waiting time, CPU pressure, and worse tail latency.

`v1.3.3` should address that gap with a simple, explainable first version that caps outbound upstream work before the system or its upstream dependencies get overloaded.

## What Changes

- Add a static global concurrency gate for outbound upstream work.
- Use that gate to bound total concurrent upstream activity across forwarding, Happy Eyeballs, and recursive NS resolution.
- Prefer a small soft-degradation ladder before fail-fast behavior when the gate is full.
- Keep the control model intentionally simple so operators can understand when and why upstream work is being reduced.

## Out of Scope

- `v1.3.4` global request fusion and broader system-wide admission control.
- A fully adaptive controller such as AIMD or any other feedback-driven window algorithm.
- Per-client or per-zone policy changes already covered by `v1.3.1` and `v1.3.2`.
- Complex per-upstream health scoring or recovery logic beyond the first version’s needs.

## Approach

Start with a single shared upstream budget for outbound work, then attach it to the places that already create upstream pressure:

- forwarding upstream attempts
- Happy Eyeballs racing queries
- concurrent NS resolution

If the gate is full, the resolver should first reduce fanout where possible and only then fail fast. That keeps the first version conservative without making it hard to explain.

The first version should treat this as a protection layer for outbound pressure, not as a new general-purpose routing or health system.

## Impact

- `server/` will gain a new upstream protection path around the existing outbound query flow.
- `monitor/` and docs will need bounded observability for upstream gate pressure and degradation behavior.
- Operators get a simple explanation: when upstream work is too busy, rec53 shrinks its own fanout before it lets pressure propagate.
- The change should reduce local CPU, goroutine, and latency pressure during upstream saturation while preserving cheap-path behavior.
