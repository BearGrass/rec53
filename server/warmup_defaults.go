package server

import "time"

// WarmupConfig represents the configuration for NS record warmup on startup
type WarmupConfig struct {
	Enabled     bool          `yaml:"enabled"`
	Timeout     time.Duration `yaml:"timeout"`
	Concurrency int           `yaml:"concurrency"`
	TLDs        []string      `yaml:"tlds"`
}

// DefaultTLDs is the default list of TLDs for NS record warmup
// Covers generic TLDs, country codes, regional TLDs, and new generic TLDs
var DefaultTLDs = []string{
	// Generic TLDs
	"com", "net", "org", "edu", "gov", "mil", "int", "info", "biz", "name",
	"pro", "asia", "cat", "coop", "jobs", "mobi", "museum", "tel", "travel", "aero",

	// Country code TLDs
	"uk", "cn", "de", "fr", "jp", "br", "in", "au", "ca", "ru",
	"mx", "es", "it", "nl", "se", "ch", "no", "be", "at", "dk",
	"pl", "gr", "pt", "tr", "kr", "tw", "hk", "sg", "my", "th",
	"id", "ph", "vn", "bd", "pk", "ng", "za", "eg", "ke", "nz",
	"ie", "il", "ae", "sa", "ar", "cl", "co", "ve", "pe", "ec",
	"uy", "cz", "hu", "ro", "bg", "hr", "si", "sk", "lt", "lv",

	// Regional TLDs
	"eu", "africa", "americas", "oceania", "asia",

	// New generic TLDs (new gTLDs)
	"app", "dev", "io", "cc", "tv", "co", "xyz", "online", "cloud", "tech",
	"site", "website", "space", "store", "shop", "blog", "news", "media", "services", "solutions",
	"design", "marketing", "consulting", "management", "ventures", "finance", "insurance", "bank", "guru", "expert",
	"academy", "education", "school", "university", "college", "training", "courses", "career", "jobs", "work",
	"company", "business", "agency", "studio", "cafe", "restaurant", "bar", "hotel", "travel", "tours",
	"flights", "booking", "fitness", "health", "medical", "hospital", "clinic", "dental", "pharmacy", "beauty",
	"spa", "salon", "sports", "games", "gaming", "esports", "video", "movie", "cinema", "music",
	"artist", "band", "concert", "theater", "photography", "photo", "gallery", "art", "museum", "fashion",
	"luxury", "jewelry", "shoes", "watch", "wine", "beer", "coffee", "food", "pizza", "burger",
	"sushi", "dance", "religion", "church", "charity", "ngo", "foundation", "club", "community",
}

// DefaultWarmupConfig is the default warmup configuration
var DefaultWarmupConfig = WarmupConfig{
	Enabled:     true,
	Timeout:     5 * time.Second,
	Concurrency: 32,
	TLDs:        DefaultTLDs,
}
