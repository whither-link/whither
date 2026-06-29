package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/whither-link/whither/internal/cache"
)

var bg = context.Background()

func newFake(clockFn func() time.Time) *cache.FakeCache {
	return cache.NewFakeCache(time.Hour, 30*time.Minute, clockFn)
}

func TestKey(t *testing.T) {
	cases := []struct {
		prefix, lang, title, want string
	}{
		{"v1", "en", "bbc", "v1:en:bbc"},
		{"v2", "fr", "le_monde", "v2:fr:le_monde"},
		{"", "", "", "::"},
	}
	for _, tc := range cases {
		got := cache.Key(tc.prefix, tc.lang, tc.title)
		if got != tc.want {
			t.Errorf("Key(%q,%q,%q) = %q, want %q", tc.prefix, tc.lang, tc.title, got, tc.want)
		}
	}
}

func TestFakeCache_MissOnEmpty(t *testing.T) {
	c := newFake(nil)
	_, ok, err := c.Get(bg, "k")
	if err != nil || ok {
		t.Fatalf("Get on empty cache: ok=%v err=%v", ok, err)
	}
}

func TestFakeCache_SetGet(t *testing.T) {
	c := newFake(nil)
	e := cache.Entry{URL: "https://example.com", ResolvedVia: "wikidata-p856", Positive: true}
	if err := c.Set(bg, "k", &e); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok, err := c.Get(bg, "k")
	if err != nil || !ok {
		t.Fatalf("Get after Set: ok=%v err=%v", ok, err)
	}
	if got.URL != e.URL || got.ResolvedVia != e.ResolvedVia || got.Positive != e.Positive {
		t.Errorf("got %+v, want %+v", got, e)
	}
}

func TestFakeCache_StoredAtSet(t *testing.T) {
	fixed := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	c := newFake(func() time.Time { return fixed })
	_ = c.Set(bg, "k", &cache.Entry{URL: "https://example.com", Positive: true})
	got, _, _ := c.Get(bg, "k")
	if !got.StoredAt.Equal(fixed) {
		t.Errorf("StoredAt = %v, want %v", got.StoredAt, fixed)
	}
}

func TestFakeCache_PositiveTTL(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	c := newFake(func() time.Time { return now })
	_ = c.Set(bg, "k", &cache.Entry{URL: "https://example.com", Positive: true})

	now = now.Add(59 * time.Minute)
	if _, ok, _ := c.Get(bg, "k"); !ok {
		t.Fatal("expected hit at 59m (within 1h positive TTL)")
	}

	now = now.Add(2 * time.Minute) // 61m total
	if _, ok, _ := c.Get(bg, "k"); ok {
		t.Fatal("expected miss at 61m (past 1h positive TTL)")
	}
}

func TestFakeCache_NegativeTTL(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	c := newFake(func() time.Time { return now })
	_ = c.Set(bg, "k", &cache.Entry{URL: "https://en.wikipedia.org/wiki/Foo", Positive: false})

	now = now.Add(29 * time.Minute)
	if _, ok, _ := c.Get(bg, "k"); !ok {
		t.Fatal("expected hit at 29m (within 30m negative TTL)")
	}

	now = now.Add(2 * time.Minute) // 31m total
	if _, ok, _ := c.Get(bg, "k"); ok {
		t.Fatal("expected miss at 31m (past 30m negative TTL)")
	}
}

func TestFakeCache_Delete(t *testing.T) {
	c := newFake(nil)
	_ = c.Set(bg, "k", &cache.Entry{URL: "https://example.com", Positive: true})
	if err := c.Delete(bg, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := c.Get(bg, "k"); ok {
		t.Fatal("expected miss after Delete")
	}
}

func TestFakeCache_Len(t *testing.T) {
	c := newFake(nil)
	if c.Len() != 0 {
		t.Fatalf("Len on empty cache = %d, want 0", c.Len())
	}
	_ = c.Set(bg, "a", &cache.Entry{Positive: true})
	_ = c.Set(bg, "b", &cache.Entry{Positive: true})
	if c.Len() != 2 {
		t.Fatalf("Len = %d, want 2", c.Len())
	}
	_ = c.Delete(bg, "a")
	if c.Len() != 1 {
		t.Fatalf("Len after Delete = %d, want 1", c.Len())
	}
}
