package wiki

import "github.com/whither-link/whither/internal/config"

// Clients bundles the three Wikimedia client types that share one HTTP transport
// and one concurrency semaphore.
type Clients struct {
	MediaWiki MediaWikiClient
	Wikidata  WikidataClient
	Articles  ArticleFetcher
}

// NewClients constructs all three clients from cfg.
// opts (e.g. WithSleepFn) are applied to the shared base client.
func NewClients(cfg *config.Config, opts ...Option) *Clients {
	base := newBaseClient(cfg, opts...)
	return &Clients{
		MediaWiki: &mediaWikiClient{base: base, apiBase: cfg.WikiAPIBase},
		Wikidata:  &wikidataClient{base: base, apiBase: cfg.WikidataAPIBase},
		Articles:  &articleFetcher{base: base, htmlBase: cfg.ArticleHTMLBase},
	}
}
