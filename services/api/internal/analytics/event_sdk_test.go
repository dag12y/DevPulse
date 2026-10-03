package analytics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEventAcceptsSDKVersion(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	valid := Event{
		EventID:   "550e8400-e29b-41d4-a716-446655440000",
		Type:      "page_view",
		ProjectID: "dp_test_tracking_id",
		VisitorID: "visitor_random_value",
		SessionID: "session_random_value",
		Timestamp: now,
		Page:      Page{URL: "https://example.com/about", Path: "/about", Title: "About"},
		Screen:    Dimensions{Width: 1920, Height: 1080},
		Viewport:  Dimensions{Width: 1200, Height: 800},
		Language:  "en-US",
		Timezone:  "Africa/Addis_Ababa",
	}
	valid.SdkVersion = "0.1.0"
	if err := valid.Validate(now); err != nil {
		t.Fatalf("expected sdk_version to be accepted: %v", err)
	}

	valid.SdkVersion = ""
	if err := valid.Validate(now); err != nil {
		t.Fatalf("expected empty sdk_version to be accepted: %v", err)
	}

	valid.SdkVersion = strings.Repeat("v", 33)
	if err := valid.Validate(now); err == nil {
		t.Fatal("expected oversized sdk_version to be rejected")
	}
}

func TestIngestAcceptsSDKVersionField(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	var gotVersion string
	handler := testHandler(now, func(_ context.Context, event Event, _ string) error {
		gotVersion = event.SdkVersion
		return nil
	})

	body := strings.Replace(validEventJSON(now), `"language":"en-US"`, `"language":"en-US","sdk_version":"0.1.0"`, 1)
	recorder := httptest.NewRecorder()
	handler.Ingest(recorder, request(body))

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if gotVersion != "0.1.0" {
		t.Fatalf("expected sdk_version to pass through, got %q", gotVersion)
	}
}
