package analytics

import "testing"

func TestClassifySource(t *testing.T) {
	cases := map[string]struct {
		referrer  string
		utmSource string
		want      string
		category  string
	}{
		"empty is direct":          {"", "", "Direct", "Direct"},
		"utm wins as campaign":     {"https://www.google.com/search?q=x", "newsletter", "newsletter", "Campaign"},
		"google search":            {"https://www.google.com/search?q=devpulse", "", "Google", "Organic Search"},
		"github":                   {"https://github.com/dag12y", "", "GitHub", "Social"},
		"linkedin":                 {"https://www.linkedin.com/feed/", "", "LinkedIn", "Social"},
		"unknown host is referral": {"https://example.com/blog", "", "example.com", "Referral"},
		"garbage is other":         {"not a url at all", "", "Other", "Other"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, category := ClassifySource(tc.referrer, tc.utmSource)
			if got != tc.want || category != tc.category {
				t.Fatalf("got (%q, %q), want (%q, %q)", got, category, tc.want, tc.category)
			}
		})
	}
}
