# Release Checklist

This checklist is for preparing a deployable release such as `v1.0.0`.

## 1. Scope Control

- freeze new feature work unless it is required for release readiness
- prioritize documentation clarity, startup/shutdown stability, and regression prevention
- confirm default, optional, and platform-specific features are documented clearly

## 2. Documentation

- sync `README.md` and `README.zh.md`
- confirm `docs/user/*` matches the current recommended operator path
- confirm `docs/dev/*` matches the current development workflow
- update `docs/architecture.md` if code structure or lifecycle behavior changed

## 3. Verification

- run `go test -short ./...`
- run targeted package tests for changed areas
- run `go test -race ./...` when feasible for release candidates
- validate the default deployment path:
  - `./generate-config.sh`
  - build
  - foreground run
  - basic `dig` validation

## 4. Operational Validation

- confirm logs are readable during `rec53ctl run`
- confirm installed services use an explicit `LOG_FILE` path and that `tail -f` works on the documented file
- confirm default log rotation still bounds application-managed disk usage
- confirm metrics endpoint is reachable
- confirm systemd install, upgrade, uninstall, and `uninstall --purge` flow still matches documentation
- confirm install refuses to overwrite unmanaged unit/binary targets
- treat XDP as optional and verify the Go path first

## 5. Release Notes

- update `CHANGELOG.md`
- call out any operator-visible changes
- call out any migration notes for metrics, config, or behavior
