## 1. Core Implementation

- [x] 1.1 Add `runtime` package import to `server/warmup_defaults.go`
- [x] 1.2 Create `calcOptimalConcurrency()` helper function in `server/warmup_defaults.go`
- [x] 1.3 Modify `DefaultWarmupConfig` to use calculated concurrency instead of hardcoded 32
- [x] 1.4 Handle config override: if `warmup.concurrency > 0` in config, use it; otherwise use calculated value

## 2. Logging & Observability

- [x] 2.1 Update warmup startup log in `server/server.go` to show actual concurrency and CPU core count
- [x] 2.2 Log message format: "Starting NS warmup with N TLDs, concurrency: M (CPU cores: K)"

## 3. Configuration

- [x] 3.1 Update `config.yaml` comment to document dynamic concurrency calculation
- [x] 3.2 Update `generate-config.sh` to include explanation of dynamic behavior
- [x] 3.3 Add note that users can override by setting `warmup.concurrency` explicitly

## 4. Testing & Verification

- [x] 4.1 Verify code compiles with `go build -o rec53 ./cmd`
- [x] 4.2 Test on local 4-core machine: verify concurrency = 8
- [x] 4.3 Verify config.yaml override works: set concurrency to 16 and confirm it uses 16
- [x] 4.4 Run `go test -race ./...` to check for race conditions
- [x] 4.5 Run `go test -short ./...` to ensure no test regressions

## 5. Documentation & Finalization

- [x] 5.1 Add entry to CHANGELOG.md describing the change
- [x] 5.2 Update README.md if needed to mention warmup optimization
- [x] 5.3 Run `gofmt -w .` to ensure code formatting compliance
- [x] 5.4 Review code for AGENTS.md convention compliance
- [x] 5.5 Create final commit with all changes
