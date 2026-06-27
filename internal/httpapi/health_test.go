package httpapi_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whither-link/whither/internal/config"
	"github.com/whither-link/whither/internal/httpapi"
	"github.com/whither-link/whither/internal/observ"
)

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	cfg := &config.Config{
		LogFormat: "text",
		LogLevel:  slog.LevelInfo,
	}
	return httpapi.NewRouter(cfg, observ.NewLogger(cfg))
}

func TestHealthEndpoint_OK(t *testing.T) {
	router := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, httpapi.HealthPath, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if body := rr.Body.String(); body != "ok\n" {
		t.Errorf("body = %q, want %q", body, "ok\n")
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
}

func TestHealthEndpoint_SetsRequestID(t *testing.T) {
	router := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, httpapi.HealthPath, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if id := rr.Header().Get("X-Request-Id"); id == "" {
		t.Error("X-Request-Id header not set by middleware")
	}
}

func TestHealthEndpoint_MethodNotAllowed(t *testing.T) {
	router := testRouter(t)

	req := httptest.NewRequest(http.MethodPost, httpapi.HealthPath, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Go 1.22+ ServeMux returns 405 for wrong method on a registered path.
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
}
