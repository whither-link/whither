package cache

import (
	"context"
	"sync"
	"time"
)

type fakeEntry struct {
	entry     Entry
	expiresAt time.Time
}

// FakeCache is a thread-safe in-memory Cache for unit tests.
// It applies the same positive/negative TTL policy as RedisCache and supports
// clock injection so tests can control time without sleeping.
type FakeCache struct {
	mu      sync.Mutex
	entries map[string]fakeEntry
	clockFn func() time.Time
	ttlPos  time.Duration
	ttlNeg  time.Duration
}

// NewFakeCache constructs an in-memory cache
func NewFakeCache(ttlPositive, ttlNegative time.Duration, clockFn func() time.Time) *FakeCache {
	if clockFn == nil {
		clockFn = time.Now
	}
	return &FakeCache{
		entries: make(map[string]fakeEntry),
		clockFn: clockFn,
		ttlPos:  ttlPositive,
		ttlNeg:  ttlNegative,
	}
}

// Get implements [Cache].
func (f *FakeCache) Get(_ context.Context, key string) (Entry, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fe, ok := f.entries[key]
	if !ok || f.clockFn().After(fe.expiresAt) {
		delete(f.entries, key)
		return Entry{}, false, nil
	}
	return fe.entry, true, nil
}

// Set implements [Cache].
func (f *FakeCache) Set(_ context.Context, key string, e Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.clockFn()
	e.StoredAt = now
	ttl := f.ttlNeg
	if e.Positive {
		ttl = f.ttlPos
	}
	f.entries[key] = fakeEntry{entry: e, expiresAt: now.Add(ttl)}
	return nil
}

// Delete implements [Cache].
func (f *FakeCache) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.entries, key)
	return nil
}

// Len returns the number of entries currently held (for test assertions).
func (f *FakeCache) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.entries)
}
