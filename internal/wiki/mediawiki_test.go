package wiki_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/whither-link/whither/internal/config"
	"github.com/whither-link/whither/internal/wiki"
)

func testConfig(t *testing.T, apiBase string) *config.Config {
	t.Helper()
	return &config.Config{
		WikiAPIBase:            apiBase,
		WikidataAPIBase:        apiBase,
		ArticleHTMLBase:        apiBase,
		UserAgentContact:       "test@example.com",
		UpstreamTimeout:        5 * time.Second,
		UpstreamMaxRetries:     0, // no retries in unit tests
		UpstreamBackoffBase:    time.Millisecond,
		UpstreamMaxConcurrency: 4,
		UpstreamMaxWaiting:     64,
		UpstreamAcquireTimeout: time.Second,
		Env:                    "development",
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../testdata/wiki/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func fixtureHandler(t *testing.T, fixture, contentType string) http.Handler {
	t.Helper()
	body := readFixture(t, fixture)
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	})
}

// --- Normalize ---------------------------------------------------------------

func TestNormalize_Found(t *testing.T) {
	srv := httptest.NewServer(fixtureHandler(t, "mediawiki-normalize-found.json", "application/json"))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	page, err := clients.MediaWiki.Normalize(context.Background(), "anna's archive")
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if page.CanonicalTitle != "Anna's Archive" {
		t.Errorf("CanonicalTitle = %q, want %q", page.CanonicalTitle, "Anna's Archive")
	}
	if page.QID != "Q115288326" {
		t.Errorf("QID = %q, want %q", page.QID, "Q115288326")
	}
	if page.IsDisambig {
		t.Error("IsDisambig = true, want false")
	}
	if page.Missing {
		t.Error("Missing = true, want false")
	}
}

func TestNormalize_Missing(t *testing.T) {
	srv := httptest.NewServer(fixtureHandler(t, "mediawiki-normalize-missing.json", "application/json"))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	_, err := clients.MediaWiki.Normalize(context.Background(), "ThisPageDoesNotExist99999")
	if !errors.Is(err, wiki.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestNormalize_Disambig(t *testing.T) {
	srv := httptest.NewServer(fixtureHandler(t, "mediawiki-normalize-disambig.json", "application/json"))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	page, err := clients.MediaWiki.Normalize(context.Background(), "Mercury")
	if !errors.Is(err, wiki.ErrDisambiguation) {
		t.Errorf("err = %v, want ErrDisambiguation", err)
	}
	if page.CanonicalTitle != "Mercury" {
		t.Errorf("CanonicalTitle = %q, want Mercury", page.CanonicalTitle)
	}
}

func TestNormalize_Redirect(t *testing.T) {
	srv := httptest.NewServer(fixtureHandler(t, "mediawiki-normalize-redirect.json", "application/json"))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	page, err := clients.MediaWiki.Normalize(context.Background(), "The Pirate Bay")
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if page.QID != "Q22663" {
		t.Errorf("QID = %q, want Q22663", page.QID)
	}
}

func TestNormalize_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	_, err := clients.MediaWiki.Normalize(context.Background(), "anything")
	if !errors.Is(err, wiki.ErrBadResponse) {
		t.Errorf("err = %v, want ErrBadResponse", err)
	}
}

func TestNormalize_UserAgentSet(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		body := readFixture(t, "mediawiki-normalize-found.json")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	_, _ = clients.MediaWiki.Normalize(context.Background(), "test")

	if gotUA == "" {
		t.Error("User-Agent header not sent")
	}
	// Must contain contact info.
	if gotUA == "Go-http-client/1.1" {
		t.Errorf("User-Agent is the default Go UA, want a descriptive one: %q", gotUA)
	}
}

// --- OpenSearch --------------------------------------------------------------

func TestOpenSearch_Hit(t *testing.T) {
	srv := httptest.NewServer(fixtureHandler(t, "mediawiki-opensearch-hit.json", "application/json"))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	titles, err := clients.MediaWiki.OpenSearch(context.Background(), "Anna's Archive", 3)
	if err != nil {
		t.Fatalf("OpenSearch: %v", err)
	}
	if len(titles) == 0 {
		t.Fatal("expected titles, got none")
	}
	if titles[0] != "Anna's Archive" {
		t.Errorf("titles[0] = %q, want %q", titles[0], "Anna's Archive")
	}
}

func TestOpenSearch_Empty(t *testing.T) {
	srv := httptest.NewServer(fixtureHandler(t, "mediawiki-opensearch-empty.json", "application/json"))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	titles, err := clients.MediaWiki.OpenSearch(context.Background(), "xyzzynonexistent", 5)
	if err != nil {
		t.Fatalf("OpenSearch: %v", err)
	}
	if len(titles) != 0 {
		t.Errorf("expected empty titles, got %v", titles)
	}
}
