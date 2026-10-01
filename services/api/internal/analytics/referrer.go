package analytics

import (
	"net/url"
	"strings"
)

// Known referrer hosts mapped to display name and category.
// per MVP traffic-source classification: Organic Search, Social, Referral.
var knownSources = []struct {
	match    string
	name     string
	category string
}{
	{"google.", "Google", "Organic Search"},
	{"bing.", "Bing", "Organic Search"},
	{"duckduckgo.", "DuckDuckGo", "Organic Search"},
	{"yahoo.", "Yahoo", "Organic Search"},
	{"yandex.", "Yandex", "Organic Search"},
	{"baidu.", "Baidu", "Organic Search"},
	{"ecosia.", "Ecosia", "Organic Search"},
	{"github.", "GitHub", "Social"},
	{"linkedin.", "LinkedIn", "Social"},
	{"reddit.", "Reddit", "Social"},
	{"facebook.", "Facebook", "Social"},
	{"instagram.", "Instagram", "Social"},
	{"youtube.", "YouTube", "Social"},
	{"youtu.be", "YouTube", "Social"},
	{"twitter.", "X", "Social"},
	{"x.com", "X", "Social"},
	{"t.co", "X", "Social"},
}

// ClassifySource normalizes a stored referrer + UTM source into a
// display source name and category. It mirrors MVP sections 29-30:
// empty referrer is Direct, explicit UTM wins as Campaign, known hosts
// map to friendly names, everything else is the bare host as Referral.
func ClassifySource(referrer, utmSource string) (source, category string) {
	if trimmed := strings.TrimSpace(utmSource); trimmed != "" {
		return trimmed, "Campaign"
	}
	trimmed := strings.TrimSpace(referrer)
	if trimmed == "" {
		return "Direct", "Direct"
	}
	host := strings.ToLower(trimmed)
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
		host = strings.ToLower(parsed.Host)
	} else if parsed, err := url.Parse("https://" + trimmed); err == nil && parsed.Host != "" && strings.Contains(trimmed, ".") {
		host = strings.ToLower(parsed.Host)
	} else {
		return "Other", "Other"
	}
	host = strings.TrimPrefix(host, "www.")
	for _, known := range knownSources {
		if strings.Contains(host, known.match) {
			return known.name, known.category
		}
	}
	if host == "" {
		return "Other", "Other"
	}
	return host, "Referral"
}
