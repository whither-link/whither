package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// WikidataClient queries the Wikidata API for P856 claims.
type WikidataClient interface {
	// OfficialWebsites returns all P856 claims for a QID with rank and qualifiers.
	// Returns ErrNotFound if no claims exist.
	OfficialWebsites(ctx context.Context, qid string) ([]OfficialSite, error)
}

type wikidataClient struct {
	base    *baseClient
	apiBase string
}

// wdClaimsResponse is the top-level shape of a wbgetclaims response.
type wdClaimsResponse struct {
	Claims map[string][]wdStatement `json:"claims"`
}

type wdStatement struct {
	Rank       string              `json:"rank"`
	MainSnak   wdSnak              `json:"mainsnak"`
	Qualifiers map[string][]wdSnak `json:"qualifiers"`
}

type wdSnak struct {
	SnakType  string       `json:"snaktype"` // "value" | "novalue" | "somevalue"
	DataValue *wdDataValue `json:"datavalue"`
}

type wdDataValue struct {
	// Value is RawMessage because Wikidata uses different JSON shapes per type:
	// url → plain string, wikibase-entityid → {"entity-type":"item","id":"Q…"}.
	Value json.RawMessage `json:"value"`
	Type  string          `json:"type"`
}

// stringValue returns the plain-string value for url-typed datavalues (e.g. P856 URLs).
// Returns "" for non-string shapes (e.g. wikibase-entityid).
func (d *wdDataValue) stringValue() string {
	if d == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(d.Value, &s); err != nil {
		return ""
	}
	return s
}

// entityID returns the Wikidata item ID from a wikibase-entityid datavalue (e.g. P407 language).
// Returns "" for non-entity shapes.
func (d *wdDataValue) entityID() string {
	if d == nil {
		return ""
	}
	var e struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(d.Value, &e); err != nil {
		return ""
	}
	return e.ID
}

func (c *wikidataClient) OfficialWebsites(ctx context.Context, qid string) ([]OfficialSite, error) {
	params := url.Values{
		"action":   {"wbgetclaims"},
		"format":   {"json"},
		"entity":   {qid},
		"property": {"P856"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"?"+params.Encode(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("wikidata official websites: build request: %w", err)
	}

	resp, err := c.base.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("wikidata official websites: %w", err)
	}
	defer drainClose(resp)

	var result wdClaimsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("wikidata official websites: decode: %w", ErrBadResponse)
	}

	statements, ok := result.Claims["P856"]
	if !ok || len(statements) == 0 {
		return nil, ErrNotFound
	}

	var sites []OfficialSite
	for _, stmt := range statements {
		// Skip novalue/somevalue snaks — they carry no URL.
		if stmt.MainSnak.SnakType != "value" || stmt.MainSnak.DataValue == nil {
			continue
		}
		u := stmt.MainSnak.DataValue.stringValue()
		if u == "" {
			continue
		}
		site := OfficialSite{
			URL:  u,
			Rank: stmt.Rank,
		}
		// Extract P407 (language of work) qualifier as a Wikidata item ID (e.g. "Q1860" = English).
		if quals, ok := stmt.Qualifiers["P407"]; ok && len(quals) > 0 {
			site.LangQual = quals[0].DataValue.entityID()
		}
		sites = append(sites, site)
	}

	if len(sites) == 0 {
		return nil, ErrNotFound
	}
	return sites, nil
}
