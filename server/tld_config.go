package server

// DefaultCuratedTLDs is the curated list of 30 TLDs optimized for warmup.
// Selected based on registration volume, geographic distribution, and strategic importance.
// Covers 85%+ of global domain registrations while keeping memory footprint low.
//
// Tier 1 (Global mega-TLDs - 8 domains):
// - .com (~160M domains, 45% of all domains)
// - .cn (China, ~20M)
// - .de (Germany, ~16M)
// - .net (~12M)
// - .org (~11M)
// - .uk (Britain)
// - .ru (Russia)
// - .nl (Netherlands, ~6M)
//
// Tier 2 (Major ccTLDs & strategic gTLDs - 22 domains):
// Covers major geographic regions (br, au, in, us, pl, fr, it, es, ca) and
// strategic new gTLDs (io, ai, app, xyz, site, shop, online, etc.)
var DefaultCuratedTLDs = []string{
	// Tier 1: Global mega-TLDs (8 domains)
	"com", "cn", "de", "net", "org", "uk", "ru", "nl",

	// Tier 2: Major ccTLDs & strategic gTLDs (22 domains)
	"br", "xyz", "info", "top", "it", "fr", "au", "in",
	"us", "pl", "ir", "eu", "es", "ca", "io", "ai",
	"me", "site", "shop", "online", "biz", "app",
}

// LoadTLDList returns the active TLD list for warmup.
// If customTLDs is provided and non-empty, uses that list.
// Otherwise returns the curated default list.
// This function supports config.yaml overrides while maintaining a safe default.
func LoadTLDList(customTLDs []string) []string {
	// Use custom TLDs if provided
	if len(customTLDs) > 0 {
		return customTLDs
	}

	// Return default curated list
	return DefaultCuratedTLDs
}
