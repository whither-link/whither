package httpapi

import "net/http"

// HealthPath is the canonical liveness-probe path (RFC 8615 well-known namespace).
const HealthPath = "/.well-known/whither/healthz"

func healthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
}
