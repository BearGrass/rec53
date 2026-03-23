# Troubleshooting

English | [中文](troubleshooting.zh.md)

Start with the default path: config generation, foreground run, a simple `dig` query, then service deployment.

## Server Does Not Start

Check:

- `config.yaml` exists
- `dns.listen` is valid and free
- `dns.metric` is valid
- the process has permission to bind the configured ports

Useful commands:

```bash
./rec53ctl run
ss -lntup | grep 5353
ss -lntp | grep 9999
curl -s -i http://127.0.0.1:9999/healthz/ready
```

## `rec53ctl install` Fails

Check:

- `systemd` is available
- you are running with sufficient privileges
- `config.yaml` exists when the install path needs to copy it

Useful commands:

```bash
systemctl status rec53
journalctl -u rec53 -n 100 --no-pager
tail -n 100 /var/log/rec53/rec53.log
```

`rec53ctl install` refuses to overwrite an existing unmanaged unit or binary. If you hit that guard, inspect the target paths before retrying.

## Uninstall Does Not Remove Everything

This is intentional. By default:

- `sudo ./rec53ctl uninstall` removes the managed service unit and binary
- config and logs are preserved so uninstall does not destroy operator data

If you really want to remove the managed config and log files too, use:

```bash
sudo ./rec53ctl uninstall --purge
```

## Logs Are Not Where Expected

Check which run mode you used:

- `./rec53ctl run` writes to stderr
- installed services write to `/var/log/rec53/rec53.log` by default
- direct `./dist/rec53 --config ...` runs still use the `-rec53.log` flag or the binary default `./log/rec53.log`

Useful commands:

```bash
journalctl -u rec53 -n 100 --no-pager
tail -n 100 /var/log/rec53/rec53.log
journalctl -u rec53 -f
tail -f /var/log/rec53/rec53.log
```

## `rec53top` Opens With No Visible Content

Check:

- the terminal supports cursor-addressable full-screen applications
- `TERM` is set to a capable value such as `xterm-256color`
- the metrics endpoint is reachable at the configured target

Try:

```bash
TERM=xterm-256color ./dist/rec53top
./dist/rec53top -plain
curl -s http://127.0.0.1:9999/metric | head
```

If the full-screen UI still fails, keep using `-plain`, raw metrics, or Prometheus temporarily and capture the terminal type before debugging further.

## DNS Queries Time Out

Check:

- the server is actually listening on the configured address
- local firewall or network policy is not blocking the port
- the node can reach root servers or forwarding upstreams
- `rec53_upstream_failures_total{reason="timeout"}` is not rising sharply
- `rec53_upstream_fallback_total` is not dominated by failure

Try:

```bash
dig @127.0.0.1 -p 5353 example.com
dig @127.0.0.1 -p 5353 example.com NS
```

## Startup Is Slow

Possible causes:

- warmup still running
- network path to upstream authoritative servers is slow
- cache is cold after restart
- snapshot load restored too few entries

Check the readiness probe before assuming startup is broken:

```bash
curl -s http://127.0.0.1:9999/healthz/ready
```

Interpret it like this:

- `ready=false` with `phase=cold-start`: listeners are not ready yet
- `ready=true` with `phase=warming`: traffic can already be served; startup is still finishing background warmup
- `ready=true` with `phase=steady`: startup contract is complete; look elsewhere if behavior is still poor
- `ready=false` with `phase=shutting-down`: this is an intentional stop path, not a fresh startup failure

Snapshot notes:

- missing snapshot file does not create a separate health phase
- snapshot restore failure means cold-cache startup, not node death
- if you need to explain why startup quality changed, check snapshot metrics and logs instead of expecting extra probe states

Mitigations:

- keep warmup enabled
- enable snapshot only after baseline validation
- avoid turning on XDP while basic startup is still under investigation
- inspect `rec53_cache_lookup_total`, `rec53_snapshot_operations_total`, and `rec53_snapshot_entries_total` before changing runtime knobs

## Forwarding Rules Do Not Match

Check:

- the zone suffix is correct
- upstreams use `host:port`
- no assumption is made about iterative fallback after all forwarding upstreams fail

Remember:

- longest suffix wins
- forwarded responses are not cached

## XDP Does Not Work

Treat this as optional. First confirm the Go path works.

Then check:

- Linux kernel support
- interface name
- required privileges or capabilities
- logs for degrade-to-Go-path messages
- `rec53_xdp_status`, `rec53_xdp_cache_sync_errors_total`, and `rec53_xdp_entries`

If XDP fails to attach, rec53 should still run through the normal Go cache path.
