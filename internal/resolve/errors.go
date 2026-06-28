package resolve

import "errors"

// ErrBadInput is returned when the raw path is empty, too long, or contains
// characters that can never form a valid Wikipedia title.
var ErrBadInput = errors.New("bad input")

// ErrUpstreamUnavailable is returned when all upstream retry attempts fail.
// Callers should attempt stale-serve (GetStale) before returning 503.
var ErrUpstreamUnavailable = errors.New("upstream unavailable")
