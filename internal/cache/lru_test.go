package cache_test

import (
	"testing"
	"time"

	"github.com/whither-link/whither/internal/cache"
)

func newTwoLevel(t *testing.T, l2 *cache.FakeCache, l1TTL time.Duration, clockFn func() time.Time) *cache.TwoLevel {
	t.Helper()
	tl, err := cache.NewTwoLevel(64, l1TTL, l2, clockFn)
	if err != nil {
		t.Fatalf("NewTwoLevel: %v", err)
	}
	return tl
}

func TestTwoLevel_SetGet(t *testing.T) {
	l2 := newFake(nil)
	tl := newTwoLevel(t, l2, time.Minute, nil)
	e := cache.Entry{URL: "https://example.com", Positive: true}
	_ = tl.Set(bg, "k", e)
	got, ok, _ := tl.Get(bg, "k")
	if !ok || got.URL != e.URL {
		t.Fatalf("Get: ok=%v url=%q", ok, got.URL)
	}
}

func TestTwoLevel_L1HitAvoidsL2(t *testing.T) {
	l2 := newFake(nil)
	tl := newTwoLevel(t, l2, time.Minute, nil)
	e := cache.Entry{URL: "https://example.com", Positive: true}

	_ = tl.Set(bg, "k", e) // writes to both L1 and L2

	_ = l2.Delete(bg, "k") // remove from L2 behind TwoLevel's back

	// L1 should serve the entry without going to L2
	got, ok, _ := tl.Get(bg, "k")
	if !ok || got.URL != e.URL {
		t.Fatalf("expected L1 hit after L2 deletion: ok=%v url=%q", ok, got.URL)
	}
}

func TestTwoLevel_L2HitPopulatesL1(t *testing.T) {
	l2 := newFake(nil)
	e := cache.Entry{URL: "https://example.com", Positive: true}
	_ = l2.Set(bg, "k", e) // seed L2 directly

	tl := newTwoLevel(t, l2, time.Minute, nil)

	// First Get: L1 miss → L2 hit → L1 populated
	_, ok, _ := tl.Get(bg, "k")
	if !ok {
		t.Fatal("expected L2 hit on first Get")
	}

	// Remove from L2; second Get should still hit L1
	_ = l2.Delete(bg, "k")
	_, ok, _ = tl.Get(bg, "k")
	if !ok {
		t.Fatal("expected L1 hit after L2 seeded on first Get")
	}
}

func TestTwoLevel_DeleteClearsBothLevels(t *testing.T) {
	l2 := newFake(nil)
	tl := newTwoLevel(t, l2, time.Minute, nil)
	_ = tl.Set(bg, "k", cache.Entry{URL: "https://example.com", Positive: true})
	_ = tl.Delete(bg, "k")

	if _, ok, _ := tl.Get(bg, "k"); ok {
		t.Fatal("expected TwoLevel miss after Delete")
	}
	if _, ok, _ := l2.Get(bg, "k"); ok {
		t.Fatal("expected L2 miss after Delete via TwoLevel")
	}
}

func TestTwoLevel_L1TTLCeiling(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	clockFn := func() time.Time { return now }

	l2 := newFake(clockFn)
	l1TTL := 10 * time.Second
	tl := newTwoLevel(t, l2, l1TTL, clockFn)

	_ = tl.Set(bg, "k", cache.Entry{URL: "https://example.com", Positive: true})

	// Within L1 TTL
	now = now.Add(5 * time.Second)
	if _, ok, _ := tl.Get(bg, "k"); !ok {
		t.Fatal("expected L1 hit within L1 TTL")
	}

	// Past L1 TTL — L1 miss, but L2 (1h positive TTL) still holds it
	now = now.Add(6 * time.Second) // 11s total
	if _, ok, _ := tl.Get(bg, "k"); !ok {
		t.Fatal("expected L2 hit after L1 TTL expiry")
	}
}
