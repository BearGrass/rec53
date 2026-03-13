## Requirements

### Requirement: handleState encapsulates handle invocation
The `handleState` helper function SHALL call `stm.handle(stm.getRequest(), stm.getResponse())` and wrap any returned error with state context. It SHALL NOT write log output (logging remains at the call site). It SHALL return `(int, error)` identical to the underlying `handle` call.

#### Scenario: Successful handle call
- **WHEN** `stm.handle()` returns `(ret, nil)`
- **THEN** `handleState` returns `(ret, nil)` unchanged

#### Scenario: Error from handle call
- **WHEN** `stm.handle()` returns `(ret, err)` with non-nil err
- **THEN** `handleState` returns `(ret, wrappedErr)` where wrappedErr includes state number and original error message

### Requirement: followCNAME encapsulates CNAME chain tracking
The `followCNAME` function SHALL perform, in order: CNAME cycle detection, cnameChain append, conditional NS/Extra clearing via `isNSRelevantForCNAME`, and CNAME target update on `stm.getRequest().Question[0].Name`. It SHALL return an error if a CNAME cycle is detected, nil otherwise.

#### Scenario: No cycle, irrelevant NS
- **WHEN** cnameTarget is not in visitedDomains AND `isNSRelevantForCNAME` returns false
- **THEN** visitedDomains[cnameTarget] is set to true, cnameRecord is appended to chain, `stm.getResponse().Ns` and `.Extra` are set to nil, `.Answer` is set to nil, and Question[0].Name is updated to cnameTarget

#### Scenario: No cycle, relevant NS (B-004)
- **WHEN** cnameTarget is not in visitedDomains AND `isNSRelevantForCNAME` returns true
- **THEN** Ns and Extra are preserved (NOT cleared); all other steps proceed identically

#### Scenario: CNAME cycle detected
- **WHEN** cnameTarget is already in visitedDomains
- **THEN** `followCNAME` returns a non-nil error containing the cycle target name

### Requirement: buildFinalResponse encapsulates response assembly
The `buildFinalResponse` function SHALL restore the original DNS question, prepend cnameChain to the response Answer section (if non-empty), and return the assembled `*dns.Msg`. It SHALL NOT modify any other field of the response.

#### Scenario: With CNAME chain
- **WHEN** cnameChain contains one or more records
- **THEN** returned response Answer equals append(cnameChain, original Answer...)

#### Scenario: Without CNAME chain
- **WHEN** cnameChain is empty
- **THEN** returned response Answer is unchanged

### Requirement: Change() loop uses bounded for-counter
The `Change()` function SHALL use `for iterations := 1; iterations <= MaxIterations; iterations++` as its loop structure. It SHALL return an error when the loop exits without reaching RETURN_RESP (i.e., MaxIterations exceeded).

#### Scenario: MaxIterations exceeded
- **WHEN** the state machine has not reached RETURN_RESP after MaxIterations iterations
- **THEN** `Change()` returns `(nil, error)` with a message indicating max iterations exceeded

### Requirement: Behavior identical to pre-refactor Change()
The refactored `Change()` SHALL produce byte-for-byte identical `*dns.Msg` responses for all inputs. All existing tests in `server/` and `e2e/` SHALL pass without modification.

#### Scenario: All existing tests pass
- **WHEN** `go test -race ./...` is run after the refactor
- **THEN** all packages report `ok` with zero failures and zero race conditions
