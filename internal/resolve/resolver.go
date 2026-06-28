// Package resolve implements the title-to-URL decision tree for Whither.
package resolve

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/whither-link/whither/internal/cache"
	"github.com/whither-link/whither/internal/config"
	"github.com/whither-link/whither/internal/wiki"
)

type resolver struct {
	mw       wiki.MediaWikiClient
	wd       wiki.WikidataClient
	articles wiki.ArticleFetcher
	cache    cache.Cache
	cfg      *config.Config
	log      *slog.Logger
}

// NewResolver constructs a Resolver wired to the provided upstream clients and cache.
func NewResolver(cfg *config.Config, mw wiki.MediaWikiClient, wd wiki.WikidataClient, articles wiki.ArticleFetcher, c cache.Cache, log *slog.Logger) Resolver {
	return &resolver{
		mw:       mw,
		wd:       wd,
		articles: articles,
		cache:    c,
		cfg:      cfg,
		log:      log,
	}
}

// Resolve turns a raw URL path segment into a validated redirect target.
// If fresh is true the cache read is skipped (supports ?fresh=1).
func (r *resolver) Resolve(ctx context.Context, rawPath string, fresh bool) (Result, error) {
	// F21: full URL decode (handles double-encoded input).
	title := fullyURLDecode(rawPath)

	if err := validateInput(title); err != nil {
		return Result{}, err
	}

	key := cache.Key(r.cfg.CacheKeyPrefix, r.cfg.CacheLang, normalizeForKey(title))

	if !fresh {
		if entry, ok, _ := r.cache.Get(ctx, key); ok {
			return fromEntry(entry), nil
		}
	}

	// F1: authoritative title normalization via MediaWiki.
	page, err := r.mw.Normalize(ctx, title)
	if err != nil {
		switch {
		case errors.Is(err, wiki.ErrDisambiguation):
			// F19: disambiguation pages have no single official site.
			// page.CanonicalTitle is valid even when ErrDisambiguation is returned.
			return r.finalize(ctx, key, r.articleURL(page.CanonicalTitle), ViaFallback, "", false)
		case errors.Is(err, wiki.ErrNotFound):
			// F5: try opensearch to find the closest article.
			return r.tryOpenSearch(ctx, key, title)
		case errors.Is(err, wiki.ErrUpstreamUnavailable):
			return Result{}, ErrUpstreamUnavailable
		default:
			return Result{}, fmt.Errorf("normalize %q: %w", title, err)
		}
	}

	return r.resolveWithPage(ctx, key, page)
}

// tryOpenSearch handles the missing-page branch (F5).
func (r *resolver) tryOpenSearch(ctx context.Context, key, title string) (Result, error) {
	cands, err := r.mw.OpenSearch(ctx, title, r.cfg.OpenSearchLimit)
	if err != nil || len(cands) == 0 {
		// F6: no candidates → fall back to search results page.
		return r.finalize(ctx, key, r.searchURL(title), ViaFallback, "", false)
	}
	page, err := r.mw.Normalize(ctx, cands[0])
	if err != nil {
		if errors.Is(err, wiki.ErrUpstreamUnavailable) {
			return Result{}, ErrUpstreamUnavailable
		}
		return r.finalize(ctx, key, r.searchURL(title), ViaFallback, "", false)
	}
	return r.resolveWithPage(ctx, key, page)
}

// resolveWithPage runs the P856 → infobox → article-fallback chain for a known page.
func (r *resolver) resolveWithPage(ctx context.Context, key string, page wiki.PageInfo) (Result, error) {
	// F2: Wikidata P856 lookup.
	if page.QID != "" {
		sites, err := r.wd.OfficialWebsites(ctx, page.QID)
		switch {
		case err == nil:
			if u := selectP856(sites, r.cfg.CacheLang); u != "" {
				return r.finalize(ctx, key, u, ViaP856, page.QID, true)
			}
		case errors.Is(err, wiki.ErrUpstreamUnavailable):
			return Result{}, ErrUpstreamUnavailable
		case errors.Is(err, wiki.ErrNotFound):
			// No P856 claims: expected for many articles; fall through silently.
		default:
			r.log.WarnContext(ctx, "wikidata P856 error", "qid", page.QID, "err", err)
		}
	}

	// F4: infobox scrape (stubbed; cfg.InfoboxEnabled will gate this once implemented).
	if r.cfg.InfoboxEnabled {
		html, err := r.articles.FetchHTML(ctx, page.CanonicalTitle)
		if err == nil {
			if u := scrapeInfobox(html); u != "" {
				return r.finalize(ctx, key, u, ViaInfobox, page.QID, true)
			}
		} else if errors.Is(err, wiki.ErrUpstreamUnavailable) {
			return Result{}, ErrUpstreamUnavailable
		}
	}

	// F6: article-page fallback.
	return r.finalize(ctx, key, r.articleURL(page.CanonicalTitle), ViaFallback, page.QID, false)
}

// finalize is the single F22 chokepoint: every candidate target passes through
// validateTarget here before being cached and returned. On validation failure the
// engine degrades to cfg.WikiArticleBase and marks the result as a negative entry.
func (r *resolver) finalize(ctx context.Context, key, u string, via ResolvedVia, qid string, positive bool) (Result, error) {
	if err := validateTarget(u); err != nil {
		r.log.WarnContext(ctx, "redirect target failed validation, degrading", "url", u, "err", err)
		u = r.cfg.WikiArticleBase
		via = ViaFallback
		qid = ""
		positive = false
	}
	entry := cache.Entry{
		URL:         u,
		ResolvedVia: string(via),
		QID:         qid,
		Positive:    positive,
	}
	_ = r.cache.Set(ctx, key, entry)
	return Result{
		Location:    u,
		ResolvedVia: via,
		QID:         qid,
		Positive:    positive,
	}, nil
}

// GetStale implements [Resolver]. It performs a cache-only lookup so the handler
// can serve a stale response when live resolution fails with ErrUpstreamUnavailable.
func (r *resolver) GetStale(ctx context.Context, rawPath string) (Result, bool, error) {
	title := fullyURLDecode(rawPath)
	if validateInput(title) != nil {
		return Result{}, false, nil
	}
	key := cache.Key(r.cfg.CacheKeyPrefix, r.cfg.CacheLang, normalizeForKey(title))
	entry, ok, _ := r.cache.Get(ctx, key)
	if !ok {
		return Result{}, false, nil
	}
	return fromEntry(entry), true, nil
}

func (r *resolver) articleURL(title string) string {
	return r.cfg.WikiArticleBase + url.PathEscape(title)
}

func (r *resolver) searchURL(query string) string {
	return r.cfg.WikiSearchBase + url.QueryEscape(query)
}

// fromEntry converts a cache.Entry into a Result for serving.
func fromEntry(e cache.Entry) Result {
	return Result{
		Location:    e.URL,
		ResolvedVia: ResolvedVia(e.ResolvedVia),
		QID:         e.QID,
		Positive:    e.Positive,
		FromCache:   true,
	}
}
