package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/whither-link/whither/internal/config"
	"github.com/whither-link/whither/internal/observ"
	"github.com/whither-link/whither/internal/resolve"
)

// NewRouter builds the application request mux and applies middleware.
func NewRouter(cfg *config.Config, logger *slog.Logger, resolver resolve.Resolver) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET "+HealthPath, healthHandler())
	mux.Handle("GET "+AboutPath, aboutHandler(cfg))
	mux.Handle("GET /about", http.RedirectHandler(AboutPath, http.StatusFound))
	// Catch-all: handles articles (F7) and root (F8, article == "").
	mux.Handle("GET /{article...}", redirectHandler(cfg, resolver, logger))

	return observ.RequestID(
		observ.AccessLog(logger)(
			observ.Recovery(logger)(mux),
		),
	)
}
