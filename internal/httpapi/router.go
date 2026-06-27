package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/whither-link/whither/internal/config"
	"github.com/whither-link/whither/internal/observ"
)

// NewRouter builds the application request mux and applies middleware.
func NewRouter(cfg *config.Config, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET "+HealthPath, healthHandler())

	return observ.RequestID(
		observ.AccessLog(logger)(
			observ.Recovery(logger)(mux),
		),
	)
}
