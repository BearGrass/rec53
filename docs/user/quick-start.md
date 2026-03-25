# Quick Start

English | [中文](quick-start.zh.md)

This guide covers the default operator path for rec53: generate a config, run in foreground, validate DNS answers, then deploy as a service.

## 1. Prerequisites

- Go 1.21 or later
- Linux is recommended for production deployment
- `dig` for validation
- `systemd` if you plan to use `rec53ctl install`

XDP is not required for the default path. Keep it disabled until the Go path is working in your environment.

## 2. Generate Configuration

```bash
./rec53ctl config
```

This creates `config.yaml`. Review it before first run. It does not install Go dependencies by itself; dependencies are fetched automatically when you run `./rec53ctl build`, `./rec53ctl top`, or `go test ./...`.

## 3. Build And Run

Recommended:

```bash
./rec53ctl build
./rec53ctl run
```

Manual:

```bash
mkdir -p dist && go build -o dist/rec53 ./cmd
./dist/rec53 --config ./config.yaml
```

## 4. Validate Basic Resolution

```bash
dig @127.0.0.1 -p 5533 example.com
dig @127.0.0.1 -p 5533 example.com AAAA
dig @127.0.0.1 -p 5533 example.com NS
```

What to check:

- the server responds without timeout
- answers match the query type
- logs do not show repeated startup or bind errors

Optional local observability check:

```bash
./rec53ctl top
```

This gives you a local six-panel summary of traffic, cache, snapshot, upstream, XDP, and state-machine health without deploying Prometheus first.

## 5. Deploy As A Service

```bash
sudo ./rec53ctl install
systemctl status rec53
tail -f /var/log/rec53/rec53.log
```

Common workflow after installation:

```bash
sudo ./rec53ctl upgrade
sudo ./rec53ctl uninstall
sudo ./rec53ctl uninstall --purge
```

## 6. Recommended First Production Rollout

- start with the default Go path
- keep `xdp.enabled: false`
- keep a local listen address first, then widen exposure deliberately
- validate metrics and logs before switching node resolver traffic

## Related Docs

- [Configuration](configuration.md)
- [Operations](operations.md)
- [rec53top Overview](rec53top.md)
- [Local Ops TUI](local-ops-tui.md)
- [Troubleshooting](troubleshooting.md)
