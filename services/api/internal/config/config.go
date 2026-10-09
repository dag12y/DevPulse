package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv                   string
	APIPort                  string
	DatabaseURL              string
	RetentionIntervalMinutes int
	GeoIPDBPath              string
	AllowedOrigins           []string
	// TrackerDir points at the built tracker bundle directory
	// (packages/tracker/dist). Empty disables /analytics.js serving.
	TrackerDir string
	// SessionCookieSecure marks the dashboard session cookie Secure and
	// names it with the __Host- prefix. Defaults to true in production;
	// override with SESSION_COOKIE_SECURE (e.g. TLS-terminating proxy in
	// front of a development API).
	SessionCookieSecure bool
	// ResendAPIKey selects Resend as the email provider. Unset means the
	// dev log sender, which prints verification/reset links to the
	// process log — fine in development, never acceptable in production
	// (Validate enforces that).
	ResendAPIKey string
	// EmailFrom is the default From header for auth mail.
	EmailFrom string
	// AppURL is the public dashboard origin; verification and reset
	// links are built as AppURL + path. OAuth callbacks also live on
	// this origin (via the dashboard's /api proxy).
	AppURL string
	// GitHubClientID/Secret enable GitHub login when both are set.
	GitHubClientID     string
	GitHubClientSecret string
	// GoogleClientID/Secret enable Google login when both are set.
	GoogleClientID     string
	GoogleClientSecret string
}

func Load() Config {
	appEnv := getEnv("APP_ENV", "development")
	return Config{
		AppEnv:                   appEnv,
		APIPort:                  getEnv("API_PORT", "8080"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		RetentionIntervalMinutes: getIntEnv("RETENTION_INTERVAL_MINUTES", 60),
		GeoIPDBPath:              os.Getenv("GEOIP_DB_PATH"),
		AllowedOrigins:           getListEnv("CORS_ALLOWED_ORIGINS"),
		TrackerDir:               os.Getenv("TRACKER_DIR"),
		SessionCookieSecure:      getBoolEnv("SESSION_COOKIE_SECURE", appEnv == "production"),
		ResendAPIKey:             os.Getenv("RESEND_API_KEY"),
		EmailFrom:                getEnv("EMAIL_FROM", "DevPulse <onboarding@resend.dev>"),
		AppURL:                   strings.TrimRight(getEnv("APP_URL", "http://localhost:3000"), "/"),
		GitHubClientID:           os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:       os.Getenv("GITHUB_CLIENT_SECRET"),
		GoogleClientID:           os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:       os.Getenv("GOOGLE_CLIENT_SECRET"),
	}
}

// Validate fails fast on configuration that would otherwise surface as
// confusing runtime errors. Production additionally requires an
// explicit CORS allowlist: an open API would accept dashboard calls
// from any origin.
func (config Config) Validate() error {
	if config.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if config.AppEnv == "production" && len(config.AllowedOrigins) == 0 {
		return errors.New("CORS_ALLOWED_ORIGINS must be set in production (comma-separated origins)")
	}
	if config.AppEnv == "production" {
		// Without a provider the dev log sender would print verification
		// and reset links into production logs — anyone with log access
		// could take over accounts. Refuse to boot instead.
		if config.ResendAPIKey == "" {
			return errors.New("RESEND_API_KEY must be set in production (Resend API key for auth email)")
		}
		if config.AppURL == "" {
			return errors.New("APP_URL must be set in production (public dashboard origin, e.g. https://app.example.com)")
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

func getIntEnv(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func getBoolEnv(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getListEnv(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	var values []string
	for _, value := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
