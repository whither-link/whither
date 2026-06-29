package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/whither-link/whither/internal/config"
	"github.com/whither-link/whither/internal/observ"
	"github.com/whither-link/whither/internal/resolve"
)

func redirectHandler(cfg *config.Config, r resolve.Resolver, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		article := req.PathValue("article")

		// F8: root — serve the about page in-place so the URL stays as /.
		if article == "" {
			aboutHandler(cfg).ServeHTTP(w, req)
			return
		}

		// F20: reject paths that exceed the configured length bound.
		if len(article) > cfg.MaxPathLen {
			http.Error(w, "path too long", http.StatusBadRequest)
			return
		}

		ctx := req.Context()
		fresh := req.URL.Query().Get("fresh") == "1"

		result, err := r.Resolve(ctx, article, fresh)
		switch {
		case err == nil:
			observ.AddLogAttr(ctx,
				slog.String("resolved_via", string(result.ResolvedVia)),
				slog.Bool("from_cache", result.FromCache),
			)
			writeRedirect(w, result, cfg)
		case errors.Is(err, resolve.ErrBadInput):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, resolve.ErrUpstreamUnavailable):
			// F18: stale-serve — attempt a cache-only lookup before returning 503.
			if stale, ok, _ := r.GetStale(ctx, article); ok {
				log.WarnContext(ctx, "serving stale entry on upstream failure", "article", article)
				writeRedirect(w, stale, cfg)
				return
			}
			w.Header().Set("Retry-After", "30")
			http.Error(w, "upstream unavailable, please retry later", http.StatusServiceUnavailable)
		default:
			log.ErrorContext(ctx, "unexpected resolver error", "err", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	})
}

// writeRedirect sets all required redirect headers and writes a 302.
// Centralising here ensures no path can omit a required header (F10, F16, F31).
func writeRedirect(w http.ResponseWriter, result resolve.Result, cfg *config.Config) {
	h := w.Header()
	h.Set("Location", result.Location)
	h.Set("X-Resolved-Via", string(result.ResolvedVia))
	h.Set("X-Attribution", cfg.AttributionText)
	h.Set("Cache-Control", "public, max-age="+strconv.Itoa(cfg.ClientCacheMaxAge))
	w.WriteHeader(http.StatusFound)
}
