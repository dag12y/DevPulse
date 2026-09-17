package projects

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "time/tzdata"
)

const (
	maxNameLength       = 255
	maxTimezoneLength   = 64
	maxDomainLength     = 253
	maxAllowedDomains   = 50
	trackingIDByteCount = 18
)

var (
	ErrNotFound = errors.New("project not found")
	ErrConflict = errors.New("project conflict")
)

type Project struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	TrackingID     string    `json:"tracking_id"`
	AllowedDomains []string  `json:"allowed_domains"`
	Timezone       string    `json:"timezone"`
	RetentionDays  int       `json:"retention_days"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateInput struct {
	Name           string   `json:"name"`
	AllowedDomains []string `json:"allowed_domains"`
	Timezone       *string  `json:"timezone"`
	RetentionDays  *int     `json:"retention_days"`
}

type UpdateInput struct {
	Name           *string   `json:"name"`
	AllowedDomains *[]string `json:"allowed_domains"`
	Timezone       *string   `json:"timezone"`
	RetentionDays  *int      `json:"retention_days"`
	Enabled        *bool     `json:"enabled"`
}

func (input *CreateInput) Validate() error {
	name, err := validateName(input.Name)
	if err != nil {
		return err
	}
	input.Name = name

	domains, err := validateDomains(input.AllowedDomains)
	if err != nil {
		return err
	}
	input.AllowedDomains = domains

	if input.Timezone != nil {
		timezone, err := validateTimezone(*input.Timezone)
		if err != nil {
			return err
		}
		input.Timezone = &timezone
	}

	if input.RetentionDays != nil && !validRetentionDays(*input.RetentionDays) {
		return errors.New("retention_days must be one of 30, 90, 180, or 365")
	}

	return nil
}

func (input *UpdateInput) Validate() error {
	if input.Name == nil && input.AllowedDomains == nil && input.Timezone == nil && input.RetentionDays == nil && input.Enabled == nil {
		return errors.New("at least one mutable field is required")
	}

	if input.Name != nil {
		name, err := validateName(*input.Name)
		if err != nil {
			return err
		}
		input.Name = &name
	}

	if input.AllowedDomains != nil {
		domains, err := validateDomains(*input.AllowedDomains)
		if err != nil {
			return err
		}
		input.AllowedDomains = &domains
	}

	if input.Timezone != nil {
		timezone, err := validateTimezone(*input.Timezone)
		if err != nil {
			return err
		}
		input.Timezone = &timezone
	}

	if input.RetentionDays != nil && !validRetentionDays(*input.RetentionDays) {
		return errors.New("retention_days must be one of 30, 90, 180, or 365")
	}

	return nil
}

func NewTrackingID() (string, error) {
	value := make([]byte, trackingIDByteCount)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate tracking ID: %w", err)
	}

	return "dp_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func validateName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", errors.New("name is required")
	}
	if len(name) > maxNameLength {
		return "", fmt.Errorf("name must not exceed %d characters", maxNameLength)
	}
	return name, nil
}

func validateTimezone(value string) (string, error) {
	timezone := strings.TrimSpace(value)
	if timezone == "" {
		return "", errors.New("timezone must not be empty")
	}
	if len(timezone) > maxTimezoneLength {
		return "", fmt.Errorf("timezone must not exceed %d characters", maxTimezoneLength)
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return "", errors.New("timezone must be a valid IANA timezone")
	}
	return timezone, nil
}

func validateDomains(values []string) ([]string, error) {
	if len(values) > maxAllowedDomains {
		return nil, fmt.Errorf("allowed_domains must not contain more than %d values", maxAllowedDomains)
	}

	domains := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		domain := strings.ToLower(strings.TrimSpace(value))
		if !validDomain(domain) {
			return nil, fmt.Errorf("invalid allowed domain %q", value)
		}
		if _, exists := seen[domain]; exists {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}

	return domains, nil
}

func validDomain(domain string) bool {
	if domain == "" || len(domain) > maxDomainLength || strings.ContainsAny(domain, "/:@?#[\\] ") {
		return false
	}

	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}

	return true
}

func validRetentionDays(days int) bool {
	switch days {
	case 30, 90, 180, 365:
		return true
	default:
		return false
	}
}
