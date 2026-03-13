## ADDED Requirements

### Requirement: getNSNamesFromResponse extracts NS names correctly

#### Scenario: response with multiple NS records
- **WHEN** `getNSNamesFromResponse` is called with a response containing NS records
- **THEN** returns all NS domain names from the Ns section

#### Scenario: response with no NS records
- **WHEN** `getNSNamesFromResponse` is called with an empty Ns section
- **THEN** returns nil

#### Scenario: response with mixed record types in Ns
- **WHEN** Ns section contains SOA and NS records mixed
- **THEN** returns only the NS names, ignoring non-NS records

---

### Requirement: resolveNSIPs resolves IPs from cache

#### Scenario: NS names present in cache
- **WHEN** `resolveNSIPs` is called with NS names that have A records in cache
- **THEN** returns all cached IP addresses

#### Scenario: NS names not in cache
- **WHEN** `resolveNSIPs` is called with NS names not present in cache
- **THEN** returns nil

#### Scenario: partial cache hit
- **WHEN** some NS names are cached and some are not
- **THEN** returns IPs only for the cached names

---

### Requirement: updateNSIPsCache stores resolved IPs in cache

#### Scenario: single NS result cached
- **WHEN** `updateNSIPsCache` is called with one nsResult containing IPs
- **THEN** a subsequent `getCacheCopyByType` for that NS name returns the A records

#### Scenario: multiple NS results cached
- **WHEN** called with multiple nsResult entries
- **THEN** all NS names are stored in cache with correct A records and 300s TTL
