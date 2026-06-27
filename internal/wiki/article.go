// Package wiki provides clients for the Wikipedia and Wikidata upstream APIs.
package wiki

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ArticleFetcher fetches the raw HTML of a Wikipedia article
type ArticleFetcher interface {
	// FetchHTML returns the rendered HTML body for the given article title
	FetchHTML(ctx context.Context, title string) ([]byte, error)
}

type articleFetcher struct {
	base     *baseClient
	htmlBase string
}

func (c *articleFetcher) FetchHTML(ctx context.Context, title string) ([]byte, error) {
	reqURL := c.htmlBase + "/page/html/" + url.PathEscape(title)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("article fetch: build request: %w", err)
	}

	resp, err := c.base.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("article fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("article fetch: read body: %w", err)
	}
	return body, nil
}
