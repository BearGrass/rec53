# Research

## Background

rec53 already has two layers of expensive-path protection in place: per-client expensive-request limiting and hot-zone admission protection. Those layers answer “who is consuming expensive work” and “which zone is pushing expensive work”, but they do not yet bound the total amount of outbound upstream work the resolver is willing to perform at once.

The current upstream path already includes multiple fanout mechanisms: forwarding upstream queries, Happy Eyeballs races, and concurrent NS resolution. Under pressure, these can amplify latency into local resource consumption: more in-flight work means more goroutines, more waiting, and more tail latency.

## Problem

The resolver needs a first-version control model for outbound upstream pressure. Without one, slow or failing upstreams can cause fanout, retries, and concurrent NS resolution to accumulate until the system spends too much time waiting on external work. That does not only hurt upstream servers; it also increases local CPU pressure, goroutine pressure, and overall request latency.

## Business Goal

Provide a simple, explainable, and safe first version of upstream protection for `v1.3.3`.

The first version should:

- cap total concurrent outbound upstream work with a static global gate
- reduce pressure on upstream servers and, as a consequence, local CPU and waiting resources
- keep behavior easy for operators to understand and troubleshoot
- fit the existing resolver structure without introducing a complex adaptive controller

## Constraints

- The change must stay separate from `v1.3.4` global request fusion logic.
- The implementation should preserve existing request semantics for cheap paths.
- Existing hot-path structure already includes Happy Eyeballs and concurrent NS resolution, so the design should avoid duplicate accounting or confusing nested budgets.
- The first version should favor predictable behavior over optimal throughput tuning.

## Options Considered

### Static global upstream semaphore

Use a fixed global concurrency gate for outbound upstream work. When the gate is full, new upstream work either degrades to a simpler path or fails fast.

- Pros: simple, deterministic, easy to test, easy to explain
- Cons: does not adapt automatically to changing upstream conditions

### Soft degradation ladder

Reduce upstream fanout before refusing work entirely. For example, degrade Happy Eyeballs to single-path mode or reduce NS concurrency first.

- Pros: preserves more useful work under pressure, good user experience
- Cons: still needs a clear global cap and adds policy complexity

### Circuit breaker with half-open probing

Treat consistently failing upstreams as unhealthy and stop sending work to them temporarily, while allowing limited probing for recovery.

- Pros: good for clearly bad upstreams
- Cons: better suited to failure than to pure saturation, adds state and recovery logic

### AIMD adaptive window

Adjust outbound concurrency dynamically using a TCP-style additive-increase / multiplicative-decrease controller.

- Pros: can converge toward available capacity automatically
- Cons: requires stable feedback signals and tuning; harder to reason about in first version

## Recommendation

Use a static global upstream semaphore as the first implementation of `v1.3.3`, with a small soft-degradation ladder before fast-fail behavior.

This is the best first step because it matches the current code shape, is easy to verify, and keeps the control semantics understandable. If later telemetry shows stable feedback signals and a need for finer adaptation, the design can evolve toward a more dynamic controller.

## Open Questions

- Should the first version gate all outbound upstream work with one shared budget, or keep separate budgets for forwarding and recursive NS resolution?
- When the gate is full, should the resolver first degrade fanout, or immediately fail fast?
- What should the first fail-fast response be: `SERVFAIL`, a softer degradation path, or a mix depending on request stage?
- Which signal should be used later if the implementation evolves toward adaptive control: in-flight count, timeout rate, RTT, or a composite signal?
