//go:build integration

package cache_test

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/whither-link/whither/internal/cache"
)

func testRedisURL() string {
	if u := os.Getenv("WHITHER_REDIS_URL"); u != "" {
		return u
	}
	return "redis://127.0.0.1:16379/0"
}

func newTestRedisCache(t *testing.T) *cache.RedisCache {
	t.Helper()
	c, err := cache.NewRedisCache(
		testRedisURL(),
		24*time.Hour,
		2*time.Hour,
		200*time.Millisecond,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
	)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestRedisCache_RoundTrip(t *testing.T) {
	c := newTestRedisCache(t)
	key := cache.Key("test", "en", "Anna's_Archive")
	e := cache.Entry{
		URL:         "https://annas-archive.org",
		ResolvedVia: "wikidata-p856",
		QID:         "Q115057960",
		Positive:    true,
	}

	_ = c.Delete(bg, key)
	if err := c.Set(bg, key, e); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, ok, err := c.Get(bg, key)
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.URL != e.URL || got.ResolvedVia != e.ResolvedVia || got.QID != e.QID || got.Positive != e.Positive {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, e)
	}
	if got.StoredAt.IsZero() {
		t.Error("StoredAt should be set by Set()")
	}
}

func TestRedisCache_MissOnAbsentKey(t *testing.T) {
	c := newTestRedisCache(t)
	key := cache.Key("test", "en", "NonExistentKey_RFC003")
	_ = c.Delete(bg, key)

	_, ok, err := c.Get(bg, key)
	if err != nil {
		t.Fatalf("Get on absent key: unexpected error: %v", err)
	}
	if ok {
		t.Fatal("Get on absent key: expected ok=false")
	}
}

func TestRedisCache_Delete(t *testing.T) {
	c := newTestRedisCache(t)
	key := cache.Key("test", "en", "DeleteTest_RFC003")

	_ = c.Set(bg, key, cache.Entry{URL: "https://example.com", Positive: true})
	if err := c.Delete(bg, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, ok, _ := c.Get(bg, key)
	if ok {
		t.Fatal("expected miss after Delete")
	}
}

func TestRedisCache_PositiveAndNegativeEntriesRoundTrip(t *testing.T) {
	c := newTestRedisCache(t)
	posKey := cache.Key("test", "en", "PositiveEntry_RFC003")
	negKey := cache.Key("test", "en", "NegativeEntry_RFC003")

	_ = c.Delete(bg, posKey)
	_ = c.Delete(bg, negKey)

	_ = c.Set(bg, posKey, cache.Entry{URL: "https://example.com", Positive: true})
	_ = c.Set(bg, negKey, cache.Entry{URL: "https://en.wikipedia.org/wiki/Foo", Positive: false})

	_, posOK, _ := c.Get(bg, posKey)
	_, negOK, _ := c.Get(bg, negKey)
	if !posOK || !negOK {
		t.Fatalf("expected both entries immediately after Set: pos=%v neg=%v", posOK, negOK)
	}
}

func TestRedisCache_KeyFormat(t *testing.T) {
	got := cache.Key("v1", "en", "Anna's_Archive")
	want := "v1:en:Anna's_Archive"
	if got != want {
		t.Errorf("Key = %q, want %q", got, want)
	}
}

func TestRedisCache_GracefulOnTimeout(t *testing.T) {
	c, err := cache.NewRedisCache(
		testRedisURL(),
		time.Hour,
		time.Hour,
		time.Nanosecond, // always times out
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
	)
	if err != nil {
		t.Fatalf("NewRedisCache: %v", err)
	}
	defer func() { _ = c.Close() }()

	_, ok, err := c.Get(bg, "k")
	if err != nil {
		t.Errorf("expected nil error on timeout (graceful degradation), got: %v", err)
	}
	if ok {
		t.Error("expected ok=false on timeout")
	}
}
