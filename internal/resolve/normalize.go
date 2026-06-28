package resolve

import (
	"fmt"
	"net/url"
	"strings"
)

const maxTitleBytes = 512

// fullyURLDecode iteratively percent-decodes s until it is stable, handling
// double-encoded input (e.g. %2527 → %27 → ').
func fullyURLDecode(s string) string {
	for {
		decoded, err := url.PathUnescape(s)
		if err != nil || decoded == s {
			return s
		}
		s = decoded
	}
}

// validateInput rejects input that can never be a valid Wikipedia title.
func validateInput(title string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("%w: empty title", ErrBadInput)
	}
	if len(title) > maxTitleBytes {
		return fmt.Errorf("%w: title exceeds %d bytes", ErrBadInput, maxTitleBytes)
	}
	if strings.ContainsRune(title, '\x00') {
		return fmt.Errorf("%w: title contains null byte", ErrBadInput)
	}
	return nil
}

// normalizeForKey produces a cheap, deterministic cache key from a decoded title.
// It converts spaces to underscores and lowercases the result. This lets common
// variants (e.g. "Anna's Archive" and "anna's_archive") share a cache slot without
// a MediaWiki round-trip.
func normalizeForKey(title string) string {
	return strings.ToLower(strings.ReplaceAll(title, " ", "_"))
}
