package analytics

import "testing"

func TestOriginHostPrefersOrigin(t *testing.T) {
	if got := OriginHost("https://example.com:8080/x", "https://other.com/"); got != "example.com" {
		t.Fatalf("got %q", got)
	}
	if got := OriginHost("", "https://sub.example.com/path?q=1"); got != "sub.example.com" {
		t.Fatalf("got %q", got)
	}
	if got := OriginHost("", ""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestAllowedHost(t *testing.T) {
	for name, testCase := range map[string]struct {
		host    string
		allowed []string
		want    bool
	}{
		"empty allowlist allows all": {host: "evil.com", allowed: nil, want: true},
		"empty host allowed":         {host: "", allowed: []string{"example.com"}, want: true},
		"exact match":                {host: "example.com", allowed: []string{"example.com"}, want: true},
		"subdomain allowed":          {host: "www.example.com", allowed: []string{"example.com"}, want: true},
		"mismatch rejected":          {host: "evil.com", allowed: []string{"example.com"}, want: false},
		"suffix trick rejected":      {host: "notexample.com", allowed: []string{"example.com"}, want: false},
		"case insensitive":           {host: "WWW.EXAMPLE.COM", allowed: []string{"example.com"}, want: true},
		"multiple domains":           {host: "app.example.org", allowed: []string{"example.com", "example.org"}, want: true},
	} {
		t.Run(name, func(t *testing.T) {
			if got := AllowedHost(testCase.host, testCase.allowed); got != testCase.want {
				t.Fatalf("AllowedHost(%q) = %v, want %v", testCase.host, got, testCase.want)
			}
		})
	}
}
