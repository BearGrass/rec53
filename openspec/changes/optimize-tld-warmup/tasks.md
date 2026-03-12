## 1. Setup and Configuration

- [x] 1.1 Create `server/tld_config.go` module to manage curated TLD list
- [x] 1.2 Define the 30-TLD curated list as a constant in `tld_config.go`
- [x] 1.3 Add LoadTLDList() function to support config.yaml override
- [x] 1.4 Update config.yaml template with optional `warmup.tlds` field
- [x] 1.5 Add configuration parsing for warmup.tlds in config loading

## 2. Warmup Process Integration

- [x] 2.1 Modify `server/warmup.go` to load TLDs from tld_config.LoadTLDList()
- [x] 2.2 Replace dynamic TLD enumeration with curated list loading
- [x] 2.3 Add logging for TLD count at startup ("Loaded N TLDs for warmup")
- [x] 2.4 Test warmup completion with 30 TLDs
- [x] 2.5 Verify warmup memory footprint reduction (target: 80-90%)

## 3. Error Handling and Robustness

- [x] 3.1 Ensure single TLD probe failure doesn't block entire warmup
- [x] 3.2 Log individual TLD probe failures with context
- [x] 3.3 Verify system continues to normal operation with partial warmup results
- [x] 3.4 Test no-warmup mode (`--no-warmup` flag) still works

## 4. Testing

- [x] 4.1 Add unit tests for LoadTLDList() function
- [x] 4.2 Add unit tests for curated TLD list composition (tier-1 and tier-2 coverage)
- [x] 4.3 Add test for custom TLD list override from config
- [x] 4.4 Add integration test for warmup with 30 TLDs
- [x] 4.5 Measure and verify memory reduction (run with -race flag)
- [x] 4.6 Run E2E tests to validate DNS query functionality unchanged
- [x] 4.7 Test startup in 512MB RAM constrained environment

## 5. Documentation and Finalization

- [x] 5.1 Document the curated TLD list in code comments (with rationale)
- [x] 5.2 Add CHANGELOG entry describing memory optimization
- [x] 5.3 Update README.md if needed to mention warmup optimization
- [x] 5.4 Verify gofmt compliance (`gofmt -w .`)
- [x] 5.5 Review code for AGENTS.md conventions compliance
- [x] 5.6 Create final commit with all changes
