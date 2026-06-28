package resolve

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/whither-link/whither/internal/cache"
	"github.com/whither-link/whither/internal/config"
	"github.com/whither-link/whither/internal/wiki"
)

// --- fakes -------------------------------------------------------------------

type fakeMW struct {
	normalizeResult  wiki.PageInfo
	normalizeErr     error
	opensearchResult []string
	opensearchErr    error
}

func (f *fakeMW) Normalize(_ context.Context, _ string) (wiki.PageInfo, error) {
	return f.normalizeResult, f.normalizeErr
}

func (f *fakeMW) OpenSearch(_ context.Context, _ string, _ int) ([]string, error) {
	return f.opensearchResult, f.opensearchErr
}

type fakeWD struct {
	sites []wiki.OfficialSite
	err   error
}

func (f *fakeWD) OfficialWebsites(_ context.Context, _ string) ([]wiki.OfficialSite, error) {
	return f.sites, f.err
}

type fakeArticles struct{}

func (f *fakeArticles) FetchHTML(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}

// --- helpers -----------------------------------------------------------------

func testCfg() *config.Config {
	return &config.Config{
		CacheKeyPrefix:  "v1",
		CacheLang:       "en",
		WikiArticleBase: "https://en.wikipedia.org/wiki/",
		WikiSearchBase:  "https://en.wikipedia.org/w/index.php?search=",
		OpenSearchLimit: 1,
		InfoboxEnabled:  false,
	}
}

func testResolver(mw wiki.MediaWikiClient, wd *fakeWD, c cache.Cache) *resolver {
	return &resolver{
		mw:       mw,
		wd:       wd,
		articles: &fakeArticles{},
		cache:    c,
		cfg:      testCfg(),
		log:      slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
}

func newCache() *cache.FakeCache {
	return cache.NewFakeCache(24*time.Hour, 2*time.Hour, nil)
}

// --- tests -------------------------------------------------------------------

func TestResolver_CacheHit(t *testing.T) {
	c := newCache()
	mw := &fakeMW{} // should not be called
	r := testResolver(mw, &fakeWD{}, c)

	_ = c.Set(context.Background(), cache.Key("v1", "en", "anna's_archive"), cache.Entry{
		URL:         "https://annas-archive.org",
		ResolvedVia: "wikidata-p856",
		QID:         "Q115057960",
		Positive:    true,
	})

	got, err := r.Resolve(context.Background(), "Anna's Archive", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !got.FromCache {
		t.Error("expected FromCache=true")
	}
	if got.Location != "https://annas-archive.org" {
		t.Errorf("Location = %q, want https://annas-archive.org", got.Location)
	}
	if got.ResolvedVia != ViaP856 {
		t.Errorf("ResolvedVia = %q, want %q", got.ResolvedVia, ViaP856)
	}
}

func TestResolver_FreshBypassesCache(t *testing.T) {
	c := newCache()
	_ = c.Set(context.Background(), cache.Key("v1", "en", "foo"), cache.Entry{
		URL:      "https://stale.example.com",
		Positive: true,
	})

	mw := &fakeMW{normalizeResult: wiki.PageInfo{CanonicalTitle: "Foo", QID: "Q1"}}
	wd := &fakeWD{sites: []wiki.OfficialSite{{URL: "https://fresh.example.com", Rank: "normal"}}}
	r := testResolver(mw, wd, c)

	got, err := r.Resolve(context.Background(), "foo", true)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.FromCache {
		t.Error("expected FromCache=false when fresh=true")
	}
	if got.Location != "https://fresh.example.com" {
		t.Errorf("Location = %q, want https://fresh.example.com", got.Location)
	}
}

func TestResolver_P856Hit(t *testing.T) {
	mw := &fakeMW{normalizeResult: wiki.PageInfo{CanonicalTitle: "Anna's Archive", QID: "Q115057960"}}
	wd := &fakeWD{sites: []wiki.OfficialSite{{URL: "https://annas-archive.org", Rank: "normal"}}}
	c := newCache()
	r := testResolver(mw, wd, c)

	got, err := r.Resolve(context.Background(), "Anna's_Archive", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Location != "https://annas-archive.org" {
		t.Errorf("Location = %q", got.Location)
	}
	if got.ResolvedVia != ViaP856 {
		t.Errorf("ResolvedVia = %q, want %q", got.ResolvedVia, ViaP856)
	}
	if !got.Positive {
		t.Error("expected Positive=true")
	}
	if got.QID != "Q115057960" {
		t.Errorf("QID = %q", got.QID)
	}

	// Result should now be cached.
	entry, ok, _ := c.Get(context.Background(), cache.Key("v1", "en", "anna's_archive"))
	if !ok {
		t.Error("expected result to be cached")
	}
	if entry.URL != "https://annas-archive.org" {
		t.Errorf("cached URL = %q", entry.URL)
	}
}

func TestResolver_NoQIDFallsToArticle(t *testing.T) {
	mw := &fakeMW{normalizeResult: wiki.PageInfo{CanonicalTitle: "Obscure Topic"}} // QID=""
	r := testResolver(mw, &fakeWD{}, newCache())

	got, err := r.Resolve(context.Background(), "Obscure Topic", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.ResolvedVia != ViaFallback {
		t.Errorf("ResolvedVia = %q, want %q", got.ResolvedVia, ViaFallback)
	}
	if got.Location != "https://en.wikipedia.org/wiki/Obscure%20Topic" {
		t.Errorf("Location = %q", got.Location)
	}
	if got.Positive {
		t.Error("expected Positive=false")
	}
}

func TestResolver_P856AbsentFallsToArticle(t *testing.T) {
	mw := &fakeMW{normalizeResult: wiki.PageInfo{CanonicalTitle: "Some Topic", QID: "Q999"}}
	wd := &fakeWD{err: wiki.ErrNotFound}
	r := testResolver(mw, wd, newCache())

	got, err := r.Resolve(context.Background(), "Some Topic", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.ResolvedVia != ViaFallback {
		t.Errorf("ResolvedVia = %q, want %q", got.ResolvedVia, ViaFallback)
	}
}

func TestResolver_Disambiguation(t *testing.T) {
	mw := &fakeMW{
		normalizeResult: wiki.PageInfo{CanonicalTitle: "Mercury (disambiguation)", IsDisambig: true},
		normalizeErr:    wiki.ErrDisambiguation,
	}
	r := testResolver(mw, &fakeWD{}, newCache())

	got, err := r.Resolve(context.Background(), "Mercury", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.ResolvedVia != ViaFallback {
		t.Errorf("ResolvedVia = %q, want %q", got.ResolvedVia, ViaFallback)
	}
	if got.Positive {
		t.Error("expected Positive=false for disambiguation")
	}
	want := "https://en.wikipedia.org/wiki/Mercury%20%28disambiguation%29"
	if got.Location != want {
		t.Errorf("Location = %q, want %q", got.Location, want)
	}
}

func TestResolver_MissingPageOpensearchHit(t *testing.T) {
	// Two sequential Normalize calls: first returns ErrNotFound, second succeeds.
	mw2 := &sequentialMW{
		calls: []mwCall{
			{result: wiki.PageInfo{Missing: true}, err: wiki.ErrNotFound},
			{result: wiki.PageInfo{CanonicalTitle: "Anna's Archive", QID: "Q115057960"}, err: nil},
		},
		opensearchResult: []string{"Anna's Archive"},
	}
	wd := &fakeWD{sites: []wiki.OfficialSite{{URL: "https://annas-archive.org", Rank: "normal"}}}
	r := testResolver(mw2, wd, newCache())

	got, err := r.Resolve(context.Background(), "annas archive", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Location != "https://annas-archive.org" {
		t.Errorf("Location = %q", got.Location)
	}
}

func TestResolver_MissingPageOpensearchEmpty(t *testing.T) {
	mw := &fakeMW{
		normalizeResult:  wiki.PageInfo{Missing: true},
		normalizeErr:     wiki.ErrNotFound,
		opensearchResult: nil,
	}
	r := testResolver(mw, &fakeWD{}, newCache())

	got, err := r.Resolve(context.Background(), "zzz nonexistent", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.ResolvedVia != ViaFallback {
		t.Errorf("ResolvedVia = %q, want %q", got.ResolvedVia, ViaFallback)
	}
	// Should redirect to search results for the query.
	if got.Location == "" {
		t.Error("Location should not be empty")
	}
}

func TestResolver_UpstreamUnavailablePropagates(t *testing.T) {
	mw := &fakeMW{normalizeErr: wiki.ErrUpstreamUnavailable}
	r := testResolver(mw, &fakeWD{}, newCache())

	_, err := r.Resolve(context.Background(), "Foo", false)
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Errorf("expected ErrUpstreamUnavailable, got %v", err)
	}
}

func TestResolver_WikidataUnavailablePropagates(t *testing.T) {
	mw := &fakeMW{normalizeResult: wiki.PageInfo{CanonicalTitle: "Foo", QID: "Q1"}}
	wd := &fakeWD{err: wiki.ErrUpstreamUnavailable}
	r := testResolver(mw, wd, newCache())

	_, err := r.Resolve(context.Background(), "Foo", false)
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Errorf("expected ErrUpstreamUnavailable, got %v", err)
	}
}

func TestResolver_GetStale_Hit(t *testing.T) {
	c := newCache()
	_ = c.Set(context.Background(), cache.Key("v1", "en", "foo"), cache.Entry{
		URL:         "https://foo.example.com",
		ResolvedVia: "wikidata-p856",
		QID:         "Q1",
		Positive:    true,
	})
	r := testResolver(&fakeMW{}, &fakeWD{}, c)

	got, ok, err := r.GetStale(context.Background(), "foo")
	if err != nil {
		t.Fatalf("GetStale: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Location != "https://foo.example.com" {
		t.Errorf("Location = %q, want https://foo.example.com", got.Location)
	}
	if !got.FromCache {
		t.Error("expected FromCache=true")
	}
}

func TestResolver_GetStale_Miss(t *testing.T) {
	r := testResolver(&fakeMW{}, &fakeWD{}, newCache())
	_, ok, err := r.GetStale(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GetStale: %v", err)
	}
	if ok {
		t.Error("expected ok=false for empty cache")
	}
}

func TestResolver_EmptyInputReturnsErrBadInput(t *testing.T) {
	r := testResolver(&fakeMW{}, &fakeWD{}, newCache())
	_, err := r.Resolve(context.Background(), "", false)
	if !errors.Is(err, ErrBadInput) {
		t.Errorf("expected ErrBadInput, got %v", err)
	}
}

// TestResolver_F22InvalidP856DegradesToBase asserts the closed-redirector invariant:
// an invalid URL from upstream is never emitted; the engine falls back safely.
func TestResolver_F22InvalidP856DegradesToBase(t *testing.T) {
	mw := &fakeMW{normalizeResult: wiki.PageInfo{CanonicalTitle: "Foo", QID: "Q1"}}
	wd := &fakeWD{sites: []wiki.OfficialSite{{URL: "javascript:alert(1)", Rank: "normal"}}}
	r := testResolver(mw, wd, newCache())

	got, err := r.Resolve(context.Background(), "Foo", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Location == "javascript:alert(1)" {
		t.Fatal("closed-redirector invariant violated: emitted javascript: URL")
	}
	if got.Location != r.cfg.WikiArticleBase {
		t.Errorf("Location = %q, want %q (wiki article base)", got.Location, r.cfg.WikiArticleBase)
	}
	if got.Positive {
		t.Error("expected Positive=false after degradation")
	}
}

func TestResolver_URLDecodeBeforeLookup(t *testing.T) {
	mw := &fakeMW{normalizeResult: wiki.PageInfo{CanonicalTitle: "Anna's Archive", QID: "Q115057960"}}
	wd := &fakeWD{sites: []wiki.OfficialSite{{URL: "https://annas-archive.org", Rank: "normal"}}}
	r := testResolver(mw, wd, newCache())

	// %27 is the percent-encoding of '
	got, err := r.Resolve(context.Background(), "Anna%27s_Archive", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Location != "https://annas-archive.org" {
		t.Errorf("Location = %q", got.Location)
	}
}

// --- sequentialMW calls Normalize in order for multi-call tests --------------

type mwCall struct {
	result wiki.PageInfo
	err    error
}

type sequentialMW struct {
	calls            []mwCall
	idx              int
	opensearchResult []string
	opensearchErr    error
}

func (m *sequentialMW) Normalize(_ context.Context, _ string) (wiki.PageInfo, error) {
	if m.idx >= len(m.calls) {
		return wiki.PageInfo{}, errors.New("unexpected Normalize call")
	}
	c := m.calls[m.idx]
	m.idx++
	return c.result, c.err
}

func (m *sequentialMW) OpenSearch(_ context.Context, _ string, _ int) ([]string, error) {
	return m.opensearchResult, m.opensearchErr
}
