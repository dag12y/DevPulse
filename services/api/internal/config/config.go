package config

import "os"

type Config struct {
	AppEnv      string
	APIPort     string
	DatabaseURL string
}

func Load() Config {
	return Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		APIPort:     getEnv("API_PORT", "4000"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}
