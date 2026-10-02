package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppEnv                   string
	APIPort                  string
	DatabaseURL              string
	RetentionIntervalMinutes int
}

func Load() Config {
	return Config{
		AppEnv:                   getEnv("APP_ENV", "development"),
		APIPort:                  getEnv("API_PORT", "8080"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		RetentionIntervalMinutes: getIntEnv("RETENTION_INTERVAL_MINUTES", 60),
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
