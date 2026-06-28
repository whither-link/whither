package resolve

import "context"

// ResolvedVia identifies which resolution strategy produced a redirect target.
type ResolvedVia string

const (
	// ViaP856 means the target came from a Wikidata P856 ("official website") claim.
	ViaP856 ResolvedVia = "wikidata-p856"
	// ViaInfobox means the target was scraped from the article's infobox (F4, deferred).
	ViaInfobox ResolvedVia = "infobox"
	// ViaFallback means no official site was found; target is the article or search page.
	ViaFallback ResolvedVia = "article-fallback"
)

// Result is the output of a successful resolution.
type Result struct {
	Location    string // validated http(s) URL; never empty
	ResolvedVia ResolvedVia
	QID         string // Wikidata entity ID; "" for pure fallbacks
	Positive    bool   // true if an official site was found
	FromCache   bool
}

// Resolver turns a raw URL path segment into a validated redirect target.
type Resolver interface {
	Resolve(ctx context.Context, rawPath string, fresh bool) (Result, error)
}
