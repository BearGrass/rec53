# rec53top Operations

This page explains how to run `rec53top`, how to move through the interface, and how to validate it locally.

## Run

Recommended:

```bash
./rec53ctl top
```

Manual build:

```bash
go build -o rec53top ./cmd/rec53top
```

Default run:

```bash
./rec53top
```

Custom target:

```bash
./rec53top -target http://127.0.0.1:9999/metric
```

## Keys

- `q`: quit
- `r`: refresh now
- `h` or `?`: toggle help
- arrows / `j k l`: move focus
- `Tab` / `Shift-Tab`: cycle focus or drill-down subviews
- `Enter`: open detail from the focused panel
- `[` / `]`: switch subviews in supported detail pages
- `1` to `6`: jump to a panel directly
- `0` or `Esc`: return to overview

## Self-Test

1. Start rec53.
2. Start `rec53top`.
3. Generate traffic.

```bash
for i in {1..20}; do dig @127.0.0.1 -p 5353 example.com >/dev/null; done
for i in {1..10}; do dig @127.0.0.1 -p 5353 github.com >/dev/null; done
for i in {1..10}; do dig @127.0.0.1 -p 5353 nosuchname1234.example. >/dev/null; done
```

4. Check that traffic, cache, upstream, and state-machine panels start to move.

## Reading Habit

1. Look at the overview first.
2. Open the suspicious panel.
3. Read `What stands out now`.
4. Use the breakdown only if the summary points to a bounded cause.
5. Hand off to logs or broader observability docs when needed.

For broader incident work, see [Observability Dashboard](../observability-dashboard.md) and [Operator Checklist](../operator-checklist.md).
