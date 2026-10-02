package analytics

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

// Enrichment holds server-derived request metadata. It is never decoded
// from the event body (json:"-") and never contains the raw client IP:
// the IP is used transiently for GeoIP resolution and then discarded,
// per the privacy rules (no raw IP in the analytics dataset).
type Enrichment struct {
	Country        string
	Region         string
	DeviceType     string
	Browser        string
	BrowserVersion string
	OS             string
	OSVersion      string
	IsBot          bool
}

// RequestMeta carries the raw HTTP request signals needed for enrichment.
// ClientIP is transient: resolved to country/region and then dropped.
type RequestMeta struct {
	UserAgent string
	ClientIP  string
	Origin    string
	Referer   string
}

// GeoResolver maps a client IP to approximate geography. The null
// implementation returns unknown until a GeoIP database is wired in.
type GeoResolver interface {
	Lookup(ip string) (country, region string)
}

type NullGeoResolver struct{}

func (NullGeoResolver) Lookup(_ string) (string, string) { return "", "" }

// Enrich derives device, browser, OS, bot and geography signals from
// request metadata. The client IP is used only for the transient GeoIP
// lookup and is never stored on the event.
func (event *Event) Enrich(meta RequestMeta, geo GeoResolver) {
	info := ParseUserAgent(meta.UserAgent)
	country, region := "", ""
	if geo != nil && meta.ClientIP != "" {
		country, region = geo.Lookup(meta.ClientIP)
	}
	event.Enrichment = Enrichment{
		Country:        country,
		Region:         region,
		DeviceType:     info.DeviceType,
		Browser:        info.Browser,
		BrowserVersion: info.BrowserVersion,
		OS:             info.OS,
		OSVersion:      info.OSVersion,
		IsBot:          info.IsBot,
	}
}

// ClientIP extracts the caller IP, preferring the leftmost X-Forwarded-For
// entry (nearest to the original client through trusted proxies) and
// falling back to the connection remote address. Port and zone are stripped.
func ClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if first, _, _ := strings.Cut(forwarded, ","); strings.TrimSpace(first) != "" {
			return stripPort(strings.TrimSpace(first))
		}
	}
	return stripPort(r.RemoteAddr)
}

func stripPort(address string) string {
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return strings.Trim(address, "[]")
}

// botMarkers are matched case-insensitively against the User-Agent.
// The list targets obvious crawlers and automation; it does not aim
// for perfect detection.
var botMarkers = []string{
	"bot", "crawler", "spider", "crawl", "slurp", "scraper", "archiver",
	"mediapartners-google", "adsbot", "headless", "phantom", "selenium",
	"puppeteer", "playwright", "monitor", "pingdom", "uptime",
}

// IsBot reports whether the User-Agent looks like automation.
// An empty User-Agent is not treated as a bot.
func IsBot(userAgent string) bool {
	lowered := strings.ToLower(userAgent)
	if lowered == "" {
		return false
	}
	for _, marker := range botMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

// ClientInfo is the parsed User-Agent result.
type ClientInfo struct {
	DeviceType     string
	Browser        string
	BrowserVersion string
	OS             string
	OSVersion      string
	IsBot          bool
}

// ParseUserAgent classifies device, browser and OS from a User-Agent
// string using ordered substring rules (no fingerprinting involved).
func ParseUserAgent(userAgent string) ClientInfo {
	info := ClientInfo{DeviceType: "unknown", Browser: "Other", OS: "Other"}
	if userAgent == "" {
		return info
	}
	if IsBot(userAgent) {
		info.IsBot = true
		info.DeviceType = "bot"
	}
	lowered := strings.ToLower(userAgent)

	info.OS, info.OSVersion = parseOS(userAgent, lowered)
	info.Browser, info.BrowserVersion = parseBrowser(userAgent, lowered)
	if !info.IsBot {
		info.DeviceType = parseDevice(lowered)
	}
	return info
}

func parseDevice(lowered string) string {
	switch {
	case strings.Contains(lowered, "tablet") || strings.Contains(lowered, "ipad"):
		return "tablet"
	case strings.Contains(lowered, "mobile") || strings.Contains(lowered, "android") ||
		strings.Contains(lowered, "iphone") || strings.Contains(lowered, "ipod") ||
		strings.Contains(lowered, "blackberry") || strings.Contains(lowered, "iemobile") ||
		strings.Contains(lowered, "opera mini") || strings.Contains(lowered, "windows phone"):
		return "mobile"
	case strings.Contains(lowered, "windows") || strings.Contains(lowered, "macintosh") ||
		strings.Contains(lowered, "linux") || strings.Contains(lowered, "x11") ||
		strings.Contains(lowered, "cros"):
		return "desktop"
	default:
		return "unknown"
	}
}

func parseBrowser(original, lowered string) (string, string) {
	// Order matters: Edge/Opera/Samsung UAs also contain Chrome and Safari tokens.
	switch {
	case strings.Contains(lowered, "edg/"):
		return "Edge", tokenVersion(original, "Edg/")
	case strings.Contains(lowered, "opr/") || strings.Contains(lowered, "opera"):
		if version := tokenVersion(original, "OPR/"); version != "" {
			return "Opera", version
		}
		return "Opera", tokenVersion(original, "Opera ")
	case strings.Contains(lowered, "samsungbrowser/"):
		return "Samsung Internet", tokenVersion(original, "SamsungBrowser/")
	case strings.Contains(lowered, "firefox/") || strings.Contains(lowered, "fxios/"):
		if version := tokenVersion(original, "FxiOS/"); version != "" {
			return "Firefox", version
		}
		return "Firefox", tokenVersion(original, "Firefox/")
	case strings.Contains(lowered, "crios/"):
		return "Chrome", tokenVersion(original, "CriOS/")
	case strings.Contains(lowered, "chrome/"):
		return "Chrome", tokenVersion(original, "Chrome/")
	case strings.Contains(lowered, "safari/"):
		return "Safari", tokenVersion(original, "Version/")
	default:
		return "Other", ""
	}
}

func parseOS(original, lowered string) (string, string) {
	switch {
	case strings.Contains(lowered, "windows nt"):
		return "Windows", windowsVersion(tokenVersion(original, "Windows NT "))
	case strings.Contains(lowered, "windows phone"):
		return "Windows", tokenVersion(original, "Windows Phone ")
	case strings.Contains(lowered, "android"):
		return "Android", tokenVersion(original, "Android ")
	case strings.Contains(lowered, "iphone") || strings.Contains(lowered, "ipad") || strings.Contains(lowered, "ipod"):
		return "iOS", darwinVersion(tokenVersion(original, "OS "))
	case strings.Contains(lowered, "mac os x"):
		return "macOS", darwinVersion(tokenVersion(original, "Mac OS X "))
	case strings.Contains(lowered, "macintosh"):
		return "macOS", ""
	case strings.Contains(lowered, "cros"):
		return "Linux", ""
	case strings.Contains(lowered, "linux"):
		return "Linux", ""
	default:
		return "Other", ""
	}
}

// tokenVersion returns the version following a token like "Chrome/",
// stopping at the first space, semicolon or parenthesis.
func tokenVersion(userAgent, token string) string {
	index := strings.Index(userAgent, token)
	if index < 0 {
		lowered := strings.ToLower(userAgent)
		index = strings.Index(lowered, strings.ToLower(token))
		if index < 0 {
			return ""
		}
	}
	rest := userAgent[index+len(token):]
	end := strings.IndexAny(rest, " ;)")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func windowsVersion(nt string) string {
	// Windows 11 reports NT 10.0 like Windows 10; report it as-is.
	if major, _, _ := strings.Cut(nt, "."); major != "" {
		if _, err := strconv.Atoi(major); err == nil {
			return major
		}
	}
	return nt
}

func darwinVersion(underscored string) string {
	return strings.ReplaceAll(underscored, "_", ".")
}
