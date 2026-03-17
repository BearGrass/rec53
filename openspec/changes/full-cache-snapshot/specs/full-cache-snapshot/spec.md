## ADDED Requirements

### Requirement: SaveSnapshot SHALL persist all cache entry types

`SaveSnapshot` SHALL save all `*dns.Msg` entries from `globalDnsCache` to the snapshot file, regardless of whether the message contains NS records. This includes A/AAAA answer records, CNAME chains, NS delegations, and negative cache entries.

#### Scenario: A/AAAA answer records are saved to snapshot

- **WHEN** `globalDnsCache` contains an entry with key `"www.example.com.:1"` holding an A answer record and `SaveSnapshot` is called
- **THEN** the snapshot file SHALL contain an entry with key `"www.example.com.:1"` and valid base64-encoded wire-format data

#### Scenario: CNAME records are saved to snapshot

- **WHEN** `globalDnsCache` contains an entry with key `"cdn.example.com.:5"` holding a CNAME record and `SaveSnapshot` is called
- **THEN** the snapshot file SHALL contain an entry with key `"cdn.example.com.:5"`

#### Scenario: NS delegation entries continue to be saved

- **WHEN** `globalDnsCache` contains an NS delegation entry with key `"com."` and `SaveSnapshot` is called
- **THEN** the snapshot file SHALL contain an entry with key `"com."`

#### Scenario: Non-dns.Msg cache items are skipped

- **WHEN** `globalDnsCache` contains an item whose value is not `*dns.Msg`
- **THEN** `SaveSnapshot` SHALL skip that item without error

### Requirement: remainingTTL SHALL consider Answer section RRs

`remainingTTL` SHALL compute the minimum remaining TTL across all three message sections: `msg.Answer`, `msg.Ns`, and `msg.Extra`. This ensures that pure answer records (where `msg.Ns` and `msg.Extra` may be empty) have their TTL correctly evaluated.

#### Scenario: Pure answer record TTL calculation

- **WHEN** a snapshot entry contains a `dns.Msg` with only `msg.Answer` populated (no Ns, no Extra) and the Answer RR has a remaining TTL of 120 seconds
- **THEN** `remainingTTL` SHALL return 120

#### Scenario: Mixed sections use minimum TTL

- **WHEN** a snapshot entry contains a `dns.Msg` with Answer TTL 300s, Ns TTL 600s, Extra TTL 200s, and 100 seconds have elapsed since save
- **THEN** `remainingTTL` SHALL return 100 (min of 200, 500, 100)

#### Scenario: All RRs expired returns zero

- **WHEN** all RRs across Answer, Ns, and Extra sections have TTLs that have fully elapsed since `savedAt`
- **THEN** `remainingTTL` SHALL return 0

### Requirement: LoadSnapshot SHALL restore all entry types

`LoadSnapshot` SHALL restore all unexpired entries from the snapshot file into `globalDnsCache`, regardless of record type. The existing TTL-based expiration filtering SHALL apply uniformly to all entry types.

#### Scenario: A/AAAA answer restored from snapshot

- **WHEN** the snapshot file contains an A record entry with remaining TTL > 0
- **THEN** `LoadSnapshot` SHALL call `setCacheCopy` for that entry and include it in the imported count

#### Scenario: Expired negative cache entry discarded on restore

- **WHEN** the snapshot file contains a negative cache entry (NXDOMAIN) saved 120 seconds ago with original TTL of 60 seconds
- **THEN** `LoadSnapshot` SHALL skip that entry (remainingTTL returns 0)

### Requirement: Snapshot format SHALL remain backward compatible

The snapshot JSON structure (`snapshotFile` / `snapshotEntry`) SHALL NOT change. New versions MUST be able to read snapshot files written by old versions (which contain only NS entries).

#### Scenario: Old NS-only snapshot loaded by new version

- **WHEN** a snapshot file written by the old NS-only version is loaded by the new version
- **THEN** `LoadSnapshot` SHALL successfully restore all unexpired NS entries with no errors
