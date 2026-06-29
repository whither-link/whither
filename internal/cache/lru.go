package cache

import (
	"context"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type l1Entry struct {
	entry     Entry
	expiresAt time.Time
}

// TwoLevel is a Cache that fronts an L2 Cache with a bounded in-process LRU (L1).
type TwoLevel struct {
	l1      *lru.Cache[string, l1Entry]
	l2      Cache
	l1TTL   time.Duration
	clockFn func() time.Time
}

// NewTwoLevel constructs a TwoLevel cache. clockFn defaults to time.Now when nil.
// Returns an error if l1Size <= 0.
func NewTwoLevel(l1Size int, l1TTL time.Duration, l2 Cache, clockFn func() time.Time) (*TwoLevel, error) {
	if clockFn == nil {
		clockFn = time.Now
	}
	l1Cache, err := lru.New[string, l1Entry](l1Size)
	if err != nil {
		return nil, err
	}
	return &TwoLevel{l1: l1Cache, l2: l2, l1TTL: l1TTL, clockFn: clockFn}, nil
}

// Get implements [Cache].
func (t *TwoLevel) Get(ctx context.Context, key string) (Entry, bool, error) {
	if le, ok := t.l1.Get(key); ok {
		if t.clockFn().Before(le.expiresAt) {
			return le.entry, true, nil
		}
		t.l1.Remove(key)
	}
	e, ok, err := t.l2.Get(ctx, key)
	if ok {
		t.l1.Add(key, l1Entry{entry: e, expiresAt: t.clockFn().Add(t.l1TTL)})
	}
	return e, ok, err
}

// Set implements [Cache].
func (t *TwoLevel) Set(ctx context.Context, key string, e *Entry) error {
	t.l1.Add(key, l1Entry{entry: *e, expiresAt: t.clockFn().Add(t.l1TTL)})
	return t.l2.Set(ctx, key, e)
}

// Delete implements [Cache].
func (t *TwoLevel) Delete(ctx context.Context, key string) error {
	t.l1.Remove(key)
	return t.l2.Delete(ctx, key)
}
