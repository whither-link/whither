package resolve

import "github.com/whither-link/whither/internal/wiki"

// langToQID maps IETF language tags to their Wikidata item IDs (P407 qualifier).
// Used to prefer the language-appropriate P856 value when multiple exist.
var langToQID = map[string]string{
	"en": "Q1860", // English
}

// selectP856 applies the F3 selection policy to a set of P856 claims:
//  1. Drop deprecated claims.
//  2. If any have rank "preferred", restrict to those; otherwise use all active.
//  3. Prefer a candidate whose P407 language qualifier matches lang.
//  4. Fall back to the first candidate in upstream order.
//
// Returns "" if no valid candidate exists.
func selectP856(sites []wiki.OfficialSite, lang string) string {
	// Drop deprecated.
	active := sites[:0:0]
	for _, s := range sites {
		if s.Rank != "deprecated" {
			active = append(active, s)
		}
	}
	if len(active) == 0 {
		return ""
	}

	// Restrict to preferred rank if any.
	var preferred []wiki.OfficialSite
	for _, s := range active {
		if s.Rank == "preferred" {
			preferred = append(preferred, s)
		}
	}
	candidates := active
	if len(preferred) > 0 {
		candidates = preferred
	}

	// Prefer language qualifier match.
	if qid, ok := langToQID[lang]; ok {
		for _, s := range candidates {
			if s.LangQual == qid {
				return s.URL
			}
		}
	}

	return candidates[0].URL
}
