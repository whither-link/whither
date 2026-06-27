package wiki

// PageInfo is the result of normalizing and resolving a Wikipedia article title.
type PageInfo struct {
	CanonicalTitle string
	QID            string // Wikidata entity ID; empty if not found
	IsDisambig     bool
	Missing        bool // true if the page does not exist
}

// OfficialSite is a single P856 ("official website") claim from Wikidata.
type OfficialSite struct {
	URL      string
	Rank     string // "preferred" | "normal" | "deprecated"
	LangQual string // P407 language qualifier, empty if absent
}
