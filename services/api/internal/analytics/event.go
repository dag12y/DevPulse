package analytics

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	_ "time/tzdata"
)

const (
	maxURLLength      = 2048
	maxPathLength     = 1024
	maxTitleLength    = 512
	maxReferrerLength = 2048
	maxLanguageLength = 32
	maxTimezoneLength = 64
	maxUTMLength      = 256
	maxVisitorKey     = 255
	maxSessionKey     = 255
	maxDimension      = 10000
	maxEventAge       = 24 * time.Hour
	maxFutureSkew     = 5 * time.Minute
)

var (
	ErrUnknownProject  = errors.New("unknown project")
	ErrDisabledProject = errors.New("disabled project")
	ErrDuplicateEvent  = errors.New("duplicate event")
)

type Event struct {
	EventID   string    `json:"event_id"`
	Type      string    `json:"type"`
	ProjectID string    `json:"project_id"`
	VisitorID string    `json:"visitor_id"`
	SessionID string    `json:"session_id"`
	Timestamp time.Time `json:"timestamp"`
	// SdkVersion reports the tracker bundle version (e.g. "0.1.0").
	// Accepted for observability; never stored, never required.
	SdkVersion string     `json:"sdk_version,omitempty"`
	Page       Page       `json:"page"`
	Screen     Dimensions `json:"screen"`
	Viewport   Dimensions `json:"viewport"`
	Language   string     `json:"language"`
	Timezone   string     `json:"timezone"`
	Campaign   Campaign   `json:"campaign"`

	// Enrichment is set server-side from request metadata (User-Agent,
	// transient client IP). It is never decoded from the request body.
	Enrichment Enrichment `json:"-"`
}

type Page struct {
	URL      string `json:"url"`
	Path     string `json:"path"`
	Title    string `json:"title"`
	Referrer string `json:"referrer"`
}

type Dimensions struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Campaign struct {
	Source   *string `json:"source"`
	Medium   *string `json:"medium"`
	Campaign *string `json:"campaign"`
	Term     *string `json:"term"`
	Content  *string `json:"content"`
}

func (event *Event) Validate(now time.Time) error {
	var id pgtype.UUID
	if err := id.Scan(event.EventID); err != nil {
		return errors.New("event_id must be a UUID")
	}
	if event.Type != "page_view" {
		return errors.New("type must be page_view")
	}
	if !strings.HasPrefix(event.ProjectID, "dp_") || len(event.ProjectID) > 64 {
		return errors.New("project_id must be a valid tracking ID")
	}
	if err := requiredLength("visitor_id", event.VisitorID, maxVisitorKey); err != nil {
		return err
	}
	if len(event.SdkVersion) > 32 {
		return errors.New("sdk_version must not exceed 32 characters")
	}
	if err := requiredLength("session_id", event.SessionID, maxSessionKey); err != nil {
		return err
	}
	if event.Timestamp.IsZero() || event.Timestamp.Before(now.Add(-maxEventAge)) || event.Timestamp.After(now.Add(maxFutureSkew)) {
		return errors.New("timestamp is outside the accepted event window")
	}
	if err := validatePage(event.Page); err != nil {
		return err
	}
	if err := validateDimensions("screen", event.Screen); err != nil {
		return err
	}
	if err := validateDimensions("viewport", event.Viewport); err != nil {
		return err
	}
	if err := optionalLength("language", event.Language, maxLanguageLength); err != nil {
		return err
	}
	if err := requiredLength("timezone", event.Timezone, maxTimezoneLength); err != nil {
		return err
	}
	if _, err := time.LoadLocation(event.Timezone); err != nil {
		return errors.New("timezone must be a valid IANA timezone")
	}
	return event.Campaign.validate()
}

func validatePage(page Page) error {
	if err := requiredLength("page.url", page.URL, maxURLLength); err != nil {
		return err
	}
	parsedURL, err := url.ParseRequestURI(page.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return errors.New("page.url must be an absolute HTTP(S) URL")
	}
	if err := requiredLength("page.path", page.Path, maxPathLength); err != nil {
		return err
	}
	if !strings.HasPrefix(page.Path, "/") {
		return errors.New("page.path must start with /")
	}
	if err := optionalLength("page.title", page.Title, maxTitleLength); err != nil {
		return err
	}
	return optionalLength("page.referrer", page.Referrer, maxReferrerLength)
}

func validateDimensions(name string, dimensions Dimensions) error {
	if dimensions.Width < 1 || dimensions.Width > maxDimension || dimensions.Height < 1 || dimensions.Height > maxDimension {
		return fmt.Errorf("%s dimensions must be between 1 and %d", name, maxDimension)
	}
	return nil
}

func (campaign Campaign) validate() error {
	for _, field := range []struct {
		name  string
		value *string
	}{
		{"campaign.source", campaign.Source},
		{"campaign.medium", campaign.Medium},
		{"campaign.campaign", campaign.Campaign},
		{"campaign.term", campaign.Term},
		{"campaign.content", campaign.Content},
	} {
		if field.value != nil && len(*field.value) > maxUTMLength {
			return fmt.Errorf("%s must not exceed %d characters", field.name, maxUTMLength)
		}
	}
	return nil
}

func requiredLength(name, value string, maximum int) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return optionalLength(name, value, maximum)
}

func optionalLength(name, value string, maximum int) error {
	if len(value) > maximum {
		return fmt.Errorf("%s must not exceed %d characters", name, maximum)
	}
	return nil
}
