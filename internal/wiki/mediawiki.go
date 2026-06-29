package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// MediaWikiClient queries the MediaWiki Action API.
type MediaWikiClient interface {
	// Normalize resolves casing, underscores and soft-redirects
	Normalize(ctx context.Context, title string) (PageInfo, error)
	// OpenSearch returns ranked candidate titles for a fuzzy query.
	OpenSearch(ctx context.Context, query string, limit int) ([]string, error)
}

type mediaWikiClient struct {
	base    *baseClient
	apiBase string
}

// mwQueryResponse is the top-level shape of an action=query response.
type mwQueryResponse struct {
	Query struct {
		Pages []mwPage `json:"pages"`
	} `json:"query"`
}

type mwPage struct {
	Title     string      `json:"title"`
	Missing   bool        `json:"missing"`
	PageProps mwPageProps `json:"pageprops"`
}

type mwPageProps struct {
	WikibaseItem   string           `json:"wikibase_item"`
	Disambiguation *json.RawMessage `json:"disambiguation"` // present (any value) = disambig
}

func (c *mediaWikiClient) Normalize(ctx context.Context, title string) (PageInfo, error) {
	params := url.Values{
		"action":        {"query"},
		"format":        {"json"},
		"formatversion": {"2"},
		"redirects":     {"1"},
		"titles":        {title},
		"prop":          {"pageprops"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"?"+params.Encode(), http.NoBody)
	if err != nil {
		return PageInfo{}, fmt.Errorf("mediawiki normalize: build request: %w", err)
	}

	resp, err := c.base.do(ctx, req)
	if err != nil {
		return PageInfo{}, fmt.Errorf("mediawiki normalize: %w", err)
	}
	defer drainClose(resp)

	var result mwQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return PageInfo{}, fmt.Errorf("mediawiki normalize: decode: %w", ErrBadResponse)
	}

	if len(result.Query.Pages) == 0 {
		return PageInfo{}, ErrNotFound
	}
	page := result.Query.Pages[0]

	if page.Missing {
		return PageInfo{Missing: true, CanonicalTitle: page.Title}, ErrNotFound
	}

	info := PageInfo{
		CanonicalTitle: page.Title,
		QID:            page.PageProps.WikibaseItem,
		IsDisambig:     page.PageProps.Disambiguation != nil,
	}
	if info.IsDisambig {
		return info, ErrDisambiguation
	}
	return info, nil
}

// --- OpenSearch --------------------------------------------------------------

func (c *mediaWikiClient) OpenSearch(ctx context.Context, query string, limit int) ([]string, error) {
	params := url.Values{
		"action": {"opensearch"},
		"format": {"json"},
		"search": {query},
		"limit":  {fmt.Sprintf("%d", limit)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"?"+params.Encode(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("mediawiki opensearch: build request: %w", err)
	}

	resp, err := c.base.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("mediawiki opensearch: %w", err)
	}
	defer drainClose(resp)

	// Response: [query, [titles], [descriptions], [urls]]
	var raw [4]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("mediawiki opensearch: decode: %w", ErrBadResponse)
	}

	var titles []string
	if err := json.Unmarshal(raw[1], &titles); err != nil {
		return nil, fmt.Errorf("mediawiki opensearch: decode titles: %w", ErrBadResponse)
	}
	return titles, nil
}
