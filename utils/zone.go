package utils

import "strings"

func GetZoneList(domain string) []string {
	zoneList := make([]string, 0)
	zoneList = append(zoneList, domain)
	for {
		if domain == "." {
			break
		}
		domain = domain[strings.Index(domain, ".")+1:]
		zoneList = append(zoneList, domain)
	}
	return zoneList
}
