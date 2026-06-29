package httpapi_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whither-link/whither/internal/config"
	"github.com/whither-link/whither/internal/httpapi"
	"github.com/whither-link/whither/internal/observ"
	"github.com/whither-link/whither/internal/resolve"
)

// noopResolver is a Resolver stub for tests that don't exercise the redirect handler.
type noopResolver struct{}

func (noopResolver) Resolve(_ context.Context, _ string, _ bool) (resolve.Result, error) {
	return resolve.Result{}, resolve.ErrUpstreamUnavailable
}

func (noopResolver) GetStale(_ context.Context, _ string) (resolve.Result, bool, error) {
	return resolve.Result{}, false, nil
}

// testCfg returns a minimal Config suitable for use in handler tests.
func testCfg() *config.Config {
	return &config.Config{
		LogFormat:         "text",
		LogLevel:          slog.LevelInfo,
		ClientCacheMaxAge: 3600,
		AttributionText:   "Test attribution",
		MaxPathLen:        512,
	}
}

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	cfg := testCfg()
	return httpapi.NewRouter(cfg, observ.NewLogger(cfg), noopResolver{})
}

func TestHealthEndpoint_OK(t *testing.T) {
	router := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, httpapi.HealthPath, http.NoBody)
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

	req := httptest.NewRequest(http.MethodGet, httpapi.HealthPath, http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if id := rr.Header().Get("X-Request-Id"); id == "" {
		t.Error("X-Request-Id header not set by middleware")
	}
}

func TestHealthEndpoint_MethodNotAllowed(t *testing.T) {
	router := testRouter(t)

	req := httptest.NewRequest(http.MethodPost, httpapi.HealthPath, http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Go 1.22+ ServeMux returns 405 for wrong method on a registered path.
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
}
