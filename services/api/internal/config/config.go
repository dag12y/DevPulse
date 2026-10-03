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
}

func Load() Config {
	return Config{
		AppEnv:                   getEnv("APP_ENV", "development"),
		APIPort:                  getEnv("API_PORT", "8080"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		RetentionIntervalMinutes: getIntEnv("RETENTION_INTERVAL_MINUTES", 60),
		GeoIPDBPath:              os.Getenv("GEOIP_DB_PATH"),
		AllowedOrigins:           getListEnv("CORS_ALLOWED_ORIGINS"),
		TrackerDir:               os.Getenv("TRACKER_DIR"),
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
