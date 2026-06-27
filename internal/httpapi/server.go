package httpapi

import (
	"net/http"

	"github.com/whither-link/whither/internal/config"
)

// NewServer constructs an http.Server with all timeouts configured from cfg.
func NewServer(cfg *config.Config, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           h,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}
