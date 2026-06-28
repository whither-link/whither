package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whither-link/whither/internal/httpapi"
	"github.com/whither-link/whither/internal/observ"
)

func newAboutRouter(t *testing.T) http.Handler {
	t.Helper()
	cfg := testCfg()
	return httpapi.NewRouter(cfg, observ.NewLogger(cfg), noopResolver{})
}

func TestAbout_Canonical_OK(t *testing.T) {
	router := newAboutRouter(t)

	req := httptest.NewRequest(http.MethodGet, httpapi.AboutPath, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestAbout_Canonical_AttributionHeader(t *testing.T) {
	router := newAboutRouter(t)

	req := httptest.NewRequest(http.MethodGet, httpapi.AboutPath, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	cfg := testCfg()
	if got := rr.Header().Get("X-Attribution"); got != cfg.AttributionText {
		t.Errorf("X-Attribution = %q, want %q", got, cfg.AttributionText)
	}
}

func TestAbout_Alias_Redirects(t *testing.T) {
	router := newAboutRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want 302", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != httpapi.AboutPath {
		t.Errorf("Location = %q, want %q", loc, httpapi.AboutPath)
	}
}

func TestAbout_NonGET_Returns405(t *testing.T) {
	router := newAboutRouter(t)

	req := httptest.NewRequest(http.MethodPost, httpapi.AboutPath, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
}
