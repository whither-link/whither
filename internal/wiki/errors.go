package wiki

import "errors"

// Sentinel errors returned by all wiki clients.
var (
	// ErrNotFound means the requested page, QID or claim does not exist.
	ErrNotFound = errors.New("not found")

	// ErrDisambiguation means the title resolved to a disambiguation page.
	ErrDisambiguation = errors.New("disambiguation page")

	// ErrUpstreamUnavailable means all retry attempts failed with transient errors.
	ErrUpstreamUnavailable = errors.New("upstream unavailable")

	// ErrBadResponse means the upstream returned a response that could not be parsed.
	ErrBadResponse = errors.New("bad upstream response")
)
