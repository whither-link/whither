package httpapi

import (
	_ "embed"
	"net/http"

	"github.com/whither-link/whither/internal/config"
)

//go:embed static/about.html
var aboutHTML []byte

// AboutPath is the canonical path for the attribution and usage page.
const AboutPath = "/.well-known/whither/about"

func aboutHandler(cfg *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Attribution", cfg.AttributionText)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(aboutHTML)
	})
}
