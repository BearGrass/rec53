package utils

import (
	"strings"
)

// GetZoneList returns the list of zones that a domain is a member of,
// from most-specific to least-specific (including the empty string "").
//
// The input domain must be a fully-qualified domain name (FQDN) ending in ".".
// Non-FQDN input (e.g. "com" with no trailing dot) is handled gracefully:
// only the domain itself is returned, without attempting to walk up the tree,
// because strings.Index would return -1 and the loop would become infinite.
func GetZoneList(domain string) []string {
	zoneList := make([]string, 0)
	zoneList = append(zoneList, domain)
	for {
		if len(domain) == 0 {
			break
		}
		idx := strings.Index(domain, ".")
		if idx == -1 {
			// No dot found: non-FQDN input; stop here to avoid an infinite loop.
			break
		}
		domain = domain[idx+1:]
		zoneList = append(zoneList, domain)
	}
	return zoneList
}
