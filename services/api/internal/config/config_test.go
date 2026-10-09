package config

import (
	"testing"
)

func TestValidateRequiresDatabaseURL(t *testing.T) {
	config := Load()
	config.DatabaseURL = ""
	if err := config.Validate(); err == nil {
		t.Fatal("expected DATABASE_URL to be required")
	}
}

func TestValidateRequiresCORSAllowlistInProduction(t *testing.T) {
	production := Config{AppEnv: "production", DatabaseURL: "postgres://example/devpulse"}
	if err := production.Validate(); err == nil {
		t.Fatal("expected CORS_ALLOWED_ORIGINS to be required in production")
	}

	production.AllowedOrigins = []string{"https://app.example.com"}
	if err := production.Validate(); err == nil {
		t.Fatal("expected RESEND_API_KEY to be required in production")
	}

	production.ResendAPIKey = "re_test_key"
	if err := production.Validate(); err == nil {
		t.Fatal("expected APP_URL to be required in production")
	}

	production.AppURL = "https://app.example.com"
	if err := production.Validate(); err != nil {
		t.Fatalf("expected valid production config, got %v", err)
	}

	development := Config{AppEnv: "development", DatabaseURL: "postgres://example/devpulse"}
	if err := development.Validate(); err != nil {
		t.Fatalf("expected open CORS in development, got %v", err)
	}
}
