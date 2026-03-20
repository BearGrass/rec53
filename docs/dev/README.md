# Developer Docs

This section is for contributors and maintainers. It focuses on how rec53 is organized, how to change it safely, and how to prepare a release.

## Core Docs

- [Architecture](../architecture.md)
- [Contributing](contributing.md)
- [Testing](testing.md)
- [Release Checklist](release.md)

## Reference Material

- [Coding Conventions](../../.rec53/CONVENTIONS.md)
- [Roadmap](../../.rec53/ROADMAP.md)
- [Metrics](../metrics.md)
- [Testing Docs Index](../testing/README.md)

## Working Style

Use the default path as the baseline:

- keep the Go path correct before enabling XDP-specific optimizations
- prefer targeted lifecycle and readability fixes over large refactors
- keep user-facing docs and developer-facing docs separate
- treat Prometheus metrics and labels as an operator-facing contract, not just internal debug output
