package server

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
)

// SnapshotConfig controls cache snapshot behaviour.
// When Enabled is false (default) the feature is a complete no-op.
// File is the path to the JSON snapshot file; an empty string is
// treated as disabled even when Enabled is true.
type SnapshotConfig struct {
	Enabled bool   `yaml:"enabled"`
	File    string `yaml:"file"`
}

// snapshotEntry is the on-disk representation of a single cache entry.
type snapshotEntry struct {
	Key     string `json:"key"`
	MsgB64  string `json:"msg_b64"`  // dns.Msg wire-format, base64-encoded
	SavedAt int64  `json:"saved_at"` // unix seconds when snapshot was written
}

// snapshotFile is the top-level JSON structure written to disk.
type snapshotFile struct {
	Entries []snapshotEntry `json:"entries"`
}

// SaveSnapshot persists all cache entries from globalDnsCache to disk.
// This includes A/AAAA answers, CNAME chains, NS delegations, negative cache
// entries, and any other dns.Msg stored in the cache.
// It is a no-op when cfg.Enabled is false or cfg.File is empty.
// Errors are returned to the caller for logging; they do not affect Shutdown.
func SaveSnapshot(cfg SnapshotConfig) error {
	if !cfg.Enabled || cfg.File == "" {
		return nil
	}

	now := time.Now().Unix()
	items := globalDnsCache.Items()

	var entries []snapshotEntry
	for key, item := range items {
		msg, ok := item.Object.(*dns.Msg)
		if !ok {
			continue
		}

		wire, err := msg.Pack()
		if err != nil {
			monitor.Rec53Log.Debugf("[SNAPSHOT] pack failed for key %s: %v", key, err)
			continue
		}
		entries = append(entries, snapshotEntry{
			Key:     key,
			MsgB64:  base64.StdEncoding.EncodeToString(wire),
			SavedAt: now,
		})
	}

	sf := snapshotFile{Entries: entries}
	data, err := json.Marshal(sf)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cfg.File), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(cfg.File, data, 0o644); err != nil {
		return err
	}

	monitor.Rec53Log.Infof("[SNAPSHOT] saved %d cache entries to %s", len(entries), cfg.File)
	return nil
}

// LoadSnapshot reads the snapshot file and restores unexpired cache entries
// into globalDnsCache. It must be called before server.Run() to guarantee the
// cache is warm before the first DNS query arrives.
//
// Returns the number of entries imported and any error encountered.
// A missing file is not an error (returns 0, nil). A corrupt file returns 0, err.
// The caller should warn-log on error and continue startup normally.
func LoadSnapshot(cfg SnapshotConfig) (int, error) {
	if !cfg.Enabled || cfg.File == "" {
		return 0, nil
	}

	data, err := os.ReadFile(cfg.File)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var sf snapshotFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return 0, err
	}

	now := time.Now().Unix()
	imported := 0

	for _, entry := range sf.Entries {
		wire, err := base64.StdEncoding.DecodeString(entry.MsgB64)
		if err != nil {
			monitor.Rec53Log.Debugf("[SNAPSHOT] base64 decode failed for key %s: %v", entry.Key, err)
			continue
		}

		var msg dns.Msg
		if err := msg.Unpack(wire); err != nil {
			monitor.Rec53Log.Debugf("[SNAPSHOT] unpack failed for key %s: %v", entry.Key, err)
			continue
		}

		// Compute remaining TTL from the minimum TTL across all sections.
		// saved_at + minTTL gives the absolute expiry; skip if already past.
		minTTL := remainingTTL(&msg, entry.SavedAt, now)
		if minTTL == 0 {
			continue // all RRs expired
		}

		setCacheCopy(entry.Key, &msg, minTTL)
		imported++
	}

	return imported, nil
}

// remainingTTL returns the remaining TTL (seconds) for the message's RRs across
// Answer, Ns, and Extra sections, given savedAt (unix sec) and now (unix sec).
// Returns 0 if all RRs are expired or all sections are empty.
func remainingTTL(msg *dns.Msg, savedAt, now int64) uint32 {
	elapsed := now - savedAt
	if elapsed < 0 {
		elapsed = 0
	}

	var minRemaining uint32
	first := true

	// Scan all three message sections for the minimum remaining TTL.
	for _, section := range [3][]dns.RR{msg.Answer, msg.Ns, msg.Extra} {
		for _, rr := range section {
			originalTTL := rr.Header().Ttl
			if int64(originalTTL) <= elapsed {
				continue // this RR is expired
			}
			remaining := uint32(int64(originalTTL) - elapsed)
			if first || remaining < minRemaining {
				minRemaining = remaining
				first = false
			}
		}
	}
	return minRemaining
}
