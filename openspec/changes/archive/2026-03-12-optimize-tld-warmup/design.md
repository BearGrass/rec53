## Context

The current warmup process attempts to enumerate and test all available TLDs dynamically, which causes:
- Server crashes due to excessive memory allocation (loading thousands of TLD records)
- Startup delays (testing thousands of nameservers)
- Operational unpredictability in resource-constrained environments

The curated TLD list approach eliminates these issues by focusing on 30 strategically selected TLDs that represent:
- 85%+ of global domain registrations
- Coverage across major ccTLDs (.cn, .de, .uk, .ru, .br, .in, .au, etc.)
- Coverage across major gTLDs (.com, .net, .org, .info, .top, .xyz, etc.)
- Emerging strategic spaces (.io, .ai for tech; .shop, .online for commercial)

## Goals / Non-Goals

**Goals:**
- Reduce warmup memory footprint by 80-90%
- Enable reliable startup in resource-constrained environments (≤512MB RAM)
- Maintain coverage for 85%+ of global domain registrations
- Simplify TLD lifecycle management (explicit vs. dynamic enumeration)
- Reduce startup time proportional to reduction in TLD count

**Non-Goals:**
- Comprehensive TLD coverage (explicitly trading off completeness for reliability)
- Dynamic TLD discovery or auto-updating TLD lists
- Support for ultra-rare or experimental TLDs
- Changes to DNS query logic or cache behavior (warmup optimization only)

## Decisions

### 1. Static Curated List vs. Dynamic Discovery
**Decision**: Use static, curated list of 30 TLDs hardcoded in application with optional override via config.

**Rationale**: 
- Dynamic discovery causes memory crashes - root problem to solve
- Static list is predictable, testable, and operationally safe
- Easy to audit and maintain (30 vs. thousands of TLDs)
- Can still be customized per deployment if needed

**Alternatives Considered**:
- Lazy loading of TLDs (still too many, still crashes) ❌
- Sampling strategy (unpredictable coverage) ❌
- Multi-stage warmup with adaptive loading (complex, still risks crashes) ❌

### 2. TLD List Composition
**Decision**: Select 30 TLDs based on registration volume, geographic distribution, and strategic importance:

**Tier 1 (Global mega-TLDs - 8 domains)**
- .com (≈160M, 45% of all domains)
- .cn (China, ≈20M)
- .de (Germany, ≈16M)
- .net (≈12M)
- .org (≈11M)
- .uk (Britain)
- .ru (Russia)
- .nl (Netherlands, ≈6M)

**Tier 2 (Major ccTLDs & strategic gTLDs - 22 domains)**
- .br, .xyz, .info, .top, .it, .fr, .au, .in, .us, .pl, .ir, .eu, .es, .ca, .io, .ai, .me, .site, .shop, .online, .biz, .app

**Rationale**: Covers 85%+ of global registrations, represents all major geographic regions and use cases.

**Alternatives Considered**:
- Regional focus only (loses global coverage) ❌
- Top 50 TLDs (still causes memory issues in resource-constrained environments) ❌
- User-defined list (adds complexity, no default safe fallback) ❌

### 3. Configuration Management
**Decision**: Support TLD list via `config.yaml` with built-in hardcoded defaults.

Structure in config:
```yaml
warmup:
  tlds:
    - com
    - cn
    - de
    # ... (30 total)
```

**Rationale**: 
- Allows per-deployment customization without code changes
- Defaults ensure safe operation even without explicit config
- Centralized in config for easy auditing and updates

### 4. Implementation Location
**Decision**: Create new module `server/tld_config.go` to manage curated TLD list; modify `server/warmup.go` to consume from this list.

**Rationale**:
- Separation of concerns (TLD list management separate from warmup logic)
- Easier to test TLD list composition independently
- Future-proof for potential TLD rotation or A/B testing

### 5. Warmup Process Changes
**Decision**: Minimize changes to warmup.go - only replace TLD enumeration source.

**Current flow**: Enumerate all TLDs → probe each → cache results  
**New flow**: Load 30 curated TLDs → probe each → cache results

**Rationale**: 
- Preserves all existing warmup logic (probe strategies, caching, error handling)
- Lower risk of regressions
- Testable by changing only the TLD source

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| **Coverage gap**: 15% of domains not represented in warmup | Warmup is optimization only; rec53 can still resolve unmapped TLDs via standard fallback. Risk is acceptable for stability gain. |
| **User complaint**: "Why can't I query .xyz/.online/etc?" | Clarify in docs: warmup uses curated list for startup reliability; queries still work via fallback. List is customizable in config. |
| **Future TLD growth**: New major TLDs (.ai boom, others) | List is explicit and maintained in code/config; easy to add new TLDs quarterly. Version with schema migration if needed. |
| **Hardcoded list feels inflexible** | Provide config override; easy to change without code redeploy. |
| **Regional bias**: Non-English TLDs underrepresented | Mitigated by including major ccTLDs (.cn, .ru, .br, .in, .au, .nl, .de). List was selected for geographic diversity. |

## Migration Plan

**Phase 1: Implementation**
1. Create `server/tld_config.go` with curated list of 30 TLDs
2. Modify `server/warmup.go` to load TLDs from config
3. Update `config.yaml` template with optional `warmup.tlds` override
4. Ensure backward compatibility (empty config uses built-in list)

**Phase 2: Testing**
1. Add unit tests for TLD list composition
2. Measure warmup memory and time reduction
3. Run E2E tests with new TLD set
4. Validate server stability under resource constraints

**Phase 3: Deployment**
1. Deploy with new TLD list (transparent to users)
2. Monitor warmup metrics in production
3. No rollback needed (purely internal optimization)

**No breaking changes**: Existing configs continue to work; new TLD behavior is transparent.

## Open Questions

1. **Should the 30-TLD list be versioned?** (e.g., in CHANGELOG) → Decide during implementation phase
2. **Quarterly review cadence for list updates?** → Recommend but not enforce in v1
3. **Metrics to expose**: Should warmup memory/time be exported to Prometheus? → Consider as follow-up optimization
