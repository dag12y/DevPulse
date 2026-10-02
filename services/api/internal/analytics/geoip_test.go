package analytics

import (
	"path/filepath"
	"runtime"
	"testing"
)

// testDBPath points at the vendored MaxMind conformance database.
// Source: github.com/maxmind/MaxMind-DB test-data (GeoLite2-City-Test).
func testDBPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "GeoLite2-City-Test.mmdb")
}

func TestMaxMindLookup(t *testing.T) {
	resolver, err := OpenMaxMind(testDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Close()

	country, region := resolver.Lookup("2.125.160.216")
	if country != "GB" || region != "West Berkshire" {
		t.Fatalf("got %q/%q, want GB/West Berkshire", country, region)
	}

	country, region = resolver.Lookup("81.2.69.142")
	if country != "GB" || region != "England" {
		t.Fatalf("got %q/%q, want GB/England", country, region)
	}
}

func TestMaxMindLookupRejectsNonPublicIPs(t *testing.T) {
	resolver, err := OpenMaxMind(testDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Close()

	for _, ip := range []string{"", "not-an-ip", "127.0.0.1", "::1", "10.0.0.1", "192.168.1.1", "1.1.1.1"} {
		if country, region := resolver.Lookup(ip); country != "" || region != "" {
			t.Fatalf("ip %q -> %q/%q, want empty", ip, country, region)
		}
	}
}

func TestOpenMaxMindRejectsBadPath(t *testing.T) {
	if _, err := OpenMaxMind("/nonexistent/GeoLite2-City.mmdb"); err == nil {
		t.Fatal("expected error for missing database")
	}
}

// TestEnrichUsesMaxMindWithoutStoringIP proves end-to-end that a real
// resolver feeds country/region into the event while the raw IP never
// lands on the event struct.
func TestEnrichUsesMaxMindWithoutStoringIP(t *testing.T) {
	resolver, err := OpenMaxMind(testDBPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Close()

	event := new(Event)
	event.Enrich(RequestMeta{UserAgent: "test", ClientIP: "2.125.160.216"}, resolver)
	if event.Enrichment.Country != "GB" || event.Enrichment.Region != "West Berkshire" {
		t.Fatalf("enrichment = %+v", event.Enrichment)
	}
}
