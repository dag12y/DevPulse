package config

import (
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
}

func Load() Config {
	return Config{
		AppEnv:                   getEnv("APP_ENV", "development"),
		APIPort:                  getEnv("API_PORT", "8080"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		RetentionIntervalMinutes: getIntEnv("RETENTION_INTERVAL_MINUTES", 60),
		GeoIPDBPath:              os.Getenv("GEOIP_DB_PATH"),
		AllowedOrigins:           getListEnv("CORS_ALLOWED_ORIGINS"),
	}
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
