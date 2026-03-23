## 1. Runtime Readiness Model

- [x] 1.1 Add an in-process runtime state holder that tracks `readiness` and bounded `phase`
- [x] 1.2 Define default startup, serving, and shutdown values for the runtime state model
- [x] 1.3 Wire startup transitions so rec53 remains not-ready until DNS listeners have successfully bound
- [x] 1.4 Wire warmup transitions so `phase=warming` is visible without blocking readiness
- [x] 1.5 Wire graceful shutdown transitions so readiness flips to false before listener teardown completes

## 2. Readiness Probe Surface

- [x] 2.1 Extend the operational HTTP server on `dns.metric` to expose `GET /healthz/ready`
- [x] 2.2 Include bounded runtime context in the readiness response body, at minimum `phase`
- [x] 2.3 Ensure the readiness surface remains additive to the existing `/metric` endpoint and does not require a new port

## 3. Lifecycle Mapping Rules

- [x] 3.1 Map cold-start, listener-bind success, warmup active, warmup complete, and shutting-down to the runtime health model
- [x] 3.2 Define how snapshot-not-found and snapshot-restore-failure affect `phase` and `readiness`
- [x] 3.3 Align `cmd/rec53.go` and `server/server.go` so startup and shutdown logs match the runtime health transitions

## 4. Verification

- [x] 4.1 Add tests covering cold-start and successful transition to ready after listener bind
- [x] 4.2 Add tests covering `phase=warming` with `ready=true`
- [x] 4.3 Add tests covering readiness becoming false during graceful shutdown before listener teardown completes
- [x] 4.4 Add tests covering health endpoint HTTP status and bounded response body content

## 5. Operator Documentation

- [x] 5.1 Update operator docs to explain `readiness` and `phase`
- [x] 5.2 Add systemd and container-oriented examples using the new readiness endpoint
- [x] 5.3 Update troubleshooting guidance so warmup, cold-start, snapshot restore issues, and true service failure are distinguished clearly
- [x] 5.4 Update roadmap references or release-facing docs so `v1.2.0` is described as runtime readiness/phase work rather than generic HA work
