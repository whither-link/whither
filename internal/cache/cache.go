// Package cache provides the caching abstraction for resolved redirect targets.
package cache

import (
	"context"
	"fmt"
	"time"
)

// Entry holds a resolved redirect target and its provenance.
type Entry struct {
	URL         string    `json:"url"`
	ResolvedVia string    `json:"resolved_via"`
	QID         string    `json:"qid"`
	Positive    bool      `json:"positive"`
	StoredAt    time.Time `json:"stored_at"`
}

// Cache is the storage abstraction for resolved redirect targets.
type Cache interface {
	Get(ctx context.Context, key string) (Entry, bool, error)
	Set(ctx context.Context, key string, e Entry) error
	Delete(ctx context.Context, key string) error
}

// Key returns the canonical cache key for a normalized Wikipedia title.
func Key(prefix, lang, normalizedTitle string) string {
	return fmt.Sprintf("%s:%s:%s", prefix, lang, normalizedTitle)
}
