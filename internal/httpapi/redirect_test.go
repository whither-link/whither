package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whither-link/whither/internal/httpapi"
	"github.com/whither-link/whither/internal/observ"
	"github.com/whither-link/whither/internal/resolve"
)

// fakeResolver is a configurable Resolver for redirect handler tests.
type fakeResolver struct {
	resolveResult resolve.Result
	resolveErr    error
	staleResult   resolve.Result
	staleOK       bool
	lastFresh     bool
}

func (f *fakeResolver) Resolve(_ context.Context, _ string, fresh bool) (resolve.Result, error) {
	f.lastFresh = fresh
	return f.resolveResult, f.resolveErr
}

func (f *fakeResolver) GetStale(_ context.Context, _ string) (resolve.Result, bool, error) {
	return f.staleResult, f.staleOK, nil
}

func newRedirectRouter(t *testing.T, r resolve.Resolver) http.Handler {
	t.Helper()
	cfg := testCfg()
	return httpapi.NewRouter(cfg, observ.NewLogger(cfg), r)
}

func TestRedirect_Root(t *testing.T) {
	r := &fakeResolver{}
	router := newRedirectRouter(t, r)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct == "" {
		t.Error("Content-Type header missing on root response")
	}
}

func TestRedirect_Article_AllHeaders(t *testing.T) {
	fr := &fakeResolver{
		resolveResult: resolve.Result{
			Location:    "https://example.com",
			ResolvedVia: resolve.ViaP856,
			Positive:    true,
		},
	}
	router := newRedirectRouter(t, fr)

	req := httptest.NewRequest(http.MethodGet, "/Some_Article", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want 302", rr.Code)
	}
	cfg := testCfg()
	checks := map[string]string{
		"Location":       "https://example.com",
		"X-Resolved-Via": string(resolve.ViaP856),
		"X-Attribution":  cfg.AttributionText,
	}
	for header, want := range checks {
		if got := rr.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if cc := rr.Header().Get("Cache-Control"); cc == "" {
		t.Error("Cache-Control header missing")
	}
}

func TestRedirect_ResolvedVia_Values(t *testing.T) {
	cases := []resolve.ResolvedVia{resolve.ViaP856, resolve.ViaInfobox, resolve.ViaFallback}
	for _, via := range cases {
		t.Run(string(via), func(t *testing.T) {
			fr := &fakeResolver{
				resolveResult: resolve.Result{
					Location:    "https://example.com",
					ResolvedVia: via,
				},
			}
			router := newRedirectRouter(t, fr)
			req := httptest.NewRequest(http.MethodGet, "/Foo", http.NoBody)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if got := rr.Header().Get("X-Resolved-Via"); got != string(via) {
				t.Errorf("X-Resolved-Via = %q, want %q", got, string(via))
			}
		})
	}
}

func TestRedirect_Fresh_PassedToResolver(t *testing.T) {
	fr := &fakeResolver{
		resolveResult: resolve.Result{Location: "https://example.com", ResolvedVia: resolve.ViaP856},
	}
	router := newRedirectRouter(t, fr)

	req := httptest.NewRequest(http.MethodGet, "/Foo?fresh=1", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if !fr.lastFresh {
		t.Error("expected fresh=true when ?fresh=1 is set")
	}
}

func TestRedirect_NoFresh_DefaultsFalse(t *testing.T) {
	fr := &fakeResolver{
		resolveResult: resolve.Result{Location: "https://example.com", ResolvedVia: resolve.ViaP856},
	}
	router := newRedirectRouter(t, fr)

	req := httptest.NewRequest(http.MethodGet, "/Foo", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if fr.lastFresh {
		t.Error("expected fresh=false when ?fresh=1 is absent")
	}
}

func TestRedirect_BadInput_Returns400(t *testing.T) {
	// Fake resolver returns ErrBadInput (e.g. empty title after decode); handler must map to 400.
	fr := &fakeResolver{resolveErr: resolve.ErrBadInput}
	router := newRedirectRouter(t, fr)

	req := httptest.NewRequest(http.MethodGet, "/bad-article", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestRedirect_PathTooLong_Returns400(t *testing.T) {
	fr := &fakeResolver{}
	router := newRedirectRouter(t, fr)

	// Build a path longer than MaxPathLen (512) using safe ASCII characters.
	article := strings.Repeat("a", 513)
	req := httptest.NewRequest(http.MethodGet, "/"+article, http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestRedirect_UpstreamUnavailable_StaleServe(t *testing.T) {
	fr := &fakeResolver{
		resolveErr: resolve.ErrUpstreamUnavailable,
		staleResult: resolve.Result{
			Location:    "https://stale.example.com",
			ResolvedVia: resolve.ViaP856,
		},
		staleOK: true,
	}
	router := newRedirectRouter(t, fr)

	req := httptest.NewRequest(http.MethodGet, "/Foo", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want 302 (stale-serve)", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "https://stale.example.com" {
		t.Errorf("Location = %q, want stale URL", loc)
	}
}

func TestRedirect_UpstreamUnavailable_NoStale_Returns503(t *testing.T) {
	fr := &fakeResolver{
		resolveErr: resolve.ErrUpstreamUnavailable,
		staleOK:    false,
	}
	router := newRedirectRouter(t, fr)

	req := httptest.NewRequest(http.MethodGet, "/Foo", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
	if ra := rr.Header().Get("Retry-After"); ra == "" {
		t.Error("Retry-After header missing on 503")
	}
}

func TestRedirect_NonGET_Returns405(t *testing.T) {
	fr := &fakeResolver{}
	router := newRedirectRouter(t, fr)

	req := httptest.NewRequest(http.MethodPost, "/Some_Article", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
}

func TestRedirect_UnexpectedError_Returns500(t *testing.T) {
	fr := &fakeResolver{resolveErr: errUnexpected}
	router := newRedirectRouter(t, fr)

	req := httptest.NewRequest(http.MethodGet, "/Foo", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

// errUnexpected is a non-sentinel error for the 500 test.
type unexpectedErr struct{}

func (unexpectedErr) Error() string { return "unexpected" }

var errUnexpected = unexpectedErr{}
