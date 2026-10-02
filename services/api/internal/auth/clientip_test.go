package auth

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPTrustOrder(t *testing.T) {
	// X-Real-IP (set by our proxy) wins over everything.
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "203.0.113.9")
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	req.RemoteAddr = "192.0.2.1:1234"
	if got := ClientIP(req); got != "203.0.113.9" {
		t.Fatalf("got %q", got)
	}

	// Otherwise the leftmost forwarded entry is used.
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 192.0.2.1")
	req.RemoteAddr = "192.0.2.1:1234"
	if got := ClientIP(req); got != "198.51.100.7" {
		t.Fatalf("got %q", got)
	}

	// Fallback to the connection address with port stripped.
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	if got := ClientIP(req); got != "192.0.2.1" {
		t.Fatalf("got %q", got)
	}
}
