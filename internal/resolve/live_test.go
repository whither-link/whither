//go:build live

package resolve_test

// Curated correctness suite — hits real Wikimedia/Wikidata APIs.
// Run via: go test -tags=live -v ./internal/resolve/...
// Or: GitHub Actions live.yml (workflow_dispatch / weekly schedule).
//
// These tests are intentionally non-blocking on PRs; they catch upstream drift.

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/whither-link/whither/internal/cache"
	"github.com/whither-link/whither/internal/config"
	"github.com/whither-link/whither/internal/resolve"
	"github.com/whither-link/whither/internal/wiki"
)

// liveResolver constructs a Resolver backed by real Wikimedia clients and an
// in-memory FakeCache so no Redis instance is required.
func liveResolver(t *testing.T) resolve.Resolver {
	t.Helper()

	contact := os.Getenv("WHITHER_USER_AGENT_CONTACT")
	if contact == "" {
		t.Skip("WHITHER_USER_AGENT_CONTACT not set — skipping live tests")
	}

	cfg := &config.Config{
		WikiAPIBase:            "https://en.wikipedia.org/w/api.php",
		WikidataAPIBase:        "https://www.wikidata.org/w/api.php",
		ArticleHTMLBase:        "https://en.wikipedia.org/api/rest_v1",
		WikiArticleBase:        "https://en.wikipedia.org/wiki/",
		WikiSearchBase:         "https://en.wikipedia.org/w/index.php?search=",
		UserAgentContact:       contact,
		UpstreamTimeout:        10 * time.Second,
		UpstreamMaxRetries:     2,
		UpstreamBackoffBase:    200 * time.Millisecond,
		UpstreamMaxConcurrency: 4,
		OpenSearchLimit:        1,
		CacheTTLPositive:       24 * time.Hour,
		CacheTTLNegative:       2 * time.Hour,
		CacheLang:              "en",
		CacheKeyPrefix:         "live-test",
	}

	clients := wiki.NewClients(cfg)
	fc := cache.NewFakeCache(cfg.CacheTTLPositive, cfg.CacheTTLNegative, time.Now)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	return resolve.NewResolver(cfg, clients.MediaWiki, clients.Wikidata, clients.Articles, fc, log)
}

// entity is one entry in the curated correctness set.
type entity struct {
	article       string // Wikipedia article name (URL-encoded where needed)
	wantDomains   []string // accepted substrings in the resolved URL
}

var curatedSet = []entity{
	{
		article:     "BBC",
		wantDomains: []string{"bbc.co.uk", "bbc.com"},
	},
	{
		article:     "NASA",
		wantDomains: []string{"nasa.gov"},
	},
	{
		article:     "Wikipedia",
		wantDomains: []string{"wikipedia.org"},
	},
	{
		article:     "New_York_Times",
		wantDomains: []string{"nytimes.com"},
	},
	{
		article:     "Anna%27s_Archive",
		wantDomains: []string{"annas-archive.org"},
	},
}

func TestLive_CuratedEntities(t *testing.T) {
	r := liveResolver(t)

	for _, e := range curatedSet {
		t.Run(e.article, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			result, err := r.Resolve(ctx, e.article, false)
			if err != nil {
				t.Fatalf("Resolve(%q): %v", e.article, err)
			}
			if !result.Positive {
				t.Fatalf("Resolve(%q): got negative result (no URL found), Location=%q", e.article, result.Location)
			}

			matched := false
			for _, want := range e.wantDomains {
				if strings.Contains(result.Location, want) {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("Resolve(%q): Location=%q does not contain any of %v", e.article, result.Location, e.wantDomains)
			}

			t.Logf("OK  %s → %s (via %s)", e.article, result.Location, result.ResolvedVia)
		})
	}
}
