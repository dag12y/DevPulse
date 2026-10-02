package analytics

import (
	"fmt"
	"net"

	"github.com/oschwald/geoip2-golang"
)

// MaxMindResolver resolves approximate country/region from a MaxMind
// City database (GeoIP2 or GeoLite2). The raw IP is used transiently for
// the lookup and never stored — see Event.Enrich.
//
// Country is the ISO code (matching the analytics schema); region is the
// most specific subdivision's English name. Anything unresolvable —
// private IPs, unknown addresses, lookup errors — yields empty strings,
// and callers collapse those to "Unknown" at query time.
type MaxMindResolver struct {
	reader *geoip2.Reader
}

// OpenMaxMind opens the City database at path. Callers own Close.
func OpenMaxMind(path string) (*MaxMindResolver, error) {
	reader, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open GeoIP database: %w", err)
	}
	return &MaxMindResolver{reader: reader}, nil
}

// Close releases the database reader.
func (resolver *MaxMindResolver) Close() error {
	return resolver.reader.Close()
}

// Lookup implements GeoResolver.
func (resolver *MaxMindResolver) Lookup(ip string) (country, region string) {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.IsPrivate() || parsed.IsLoopback() ||
		parsed.IsUnspecified() || parsed.IsMulticast() || parsed.IsLinkLocalUnicast() {
		return "", ""
	}
	record, err := resolver.reader.City(parsed)
	if err != nil {
		return "", ""
	}
	country = record.Country.IsoCode
	if subdivisions := record.Subdivisions; len(subdivisions) > 0 {
		last := subdivisions[len(subdivisions)-1]
		region = last.Names["en"]
		if region == "" {
			region = last.IsoCode
		}
	}
	return country, region
}
