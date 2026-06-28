package resolve

import (
	"fmt"
	"net/url"
	"strings"
)

// validateTarget is the closed-redirector invariant (F22, RULES §0.2).
// It returns nil only if u is an absolute http/https URL with a non-empty host
// and no control characters. All redirect targets must pass through this function.
func validateTarget(u string) error {
	if strings.ContainsAny(u, "\r\n\t\x00") {
		return fmt.Errorf("URL contains control characters")
	}
	if u != strings.TrimSpace(u) {
		return fmt.Errorf("URL has leading or trailing whitespace")
	}
	parsed, err := url.Parse(u)
	if err != nil || !parsed.IsAbs() {
		return fmt.Errorf("not an absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("scheme %q is not permitted", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("empty host")
	}
	return nil
}
