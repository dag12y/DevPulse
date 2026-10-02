package analytics

import (
	"errors"
	"net/url"
	"strings"
)

// ErrOriginNotAllowed is returned when the request origin does not match
// the project's allowed_domains. Absent Origin/Referer headers are allowed
// (non-browser clients) – only a present-but-mismatched origin is rejected.
var ErrOriginNotAllowed = errors.New("origin not allowed for this project")

// OriginHost prefers the Origin header host, falling back to the Referer
// host. Both are reduced to a lowercase hostname without port.
func OriginHost(origin, referer string) string {
	if host := hostOf(origin); host != "" {
		return host
	}
	return hostOf(referer)
}

func hostOf(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host
}

// AllowedHost reports whether host matches the project's allowed domains.
// An empty allowlist permits any host. A listed domain also permits its
// subdomains (example.com covers www.example.com).
func AllowedHost(host string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return true
	}
	for _, domain := range allowed {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" {
			continue
		}
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
