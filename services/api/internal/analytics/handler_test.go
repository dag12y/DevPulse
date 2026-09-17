package analytics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type stubRepository struct {
	ingest func(context.Context, Event) error
}

func (stub *stubRepository) Ingest(ctx context.Context, event Event) error {
	return stub.ingest(ctx, event)
}

func TestIngestAcceptsValidPageView(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	called := false
	handler := testHandler(now, func(_ context.Context, event Event) error {
		called = true
		if event.ProjectID != "dp_test_tracking_id" || event.Page.Path != "/about" {
			t.Fatalf("unexpected event: %#v", event)
		}
		return nil
	})

	recorder := httptest.NewRecorder()
	handler.Ingest(recorder, request(validEventJSON(now)))

	if recorder.Code != http.StatusAccepted || !called {
		t.Fatalf("status=%d called=%v body=%s", recorder.Code, called, recorder.Body.String())
	}
}

func TestIngestRejectsInvalidJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	testHandler(time.Now(), func(context.Context, Event) error { return nil }).Ingest(recorder, request(`{"event_id":`))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestIngestRejectsInvalidEvent(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	for name, body := range map[string]string{
		"invalid UUID":     strings.Replace(validEventJSON(now), "550e8400-e29b-41d4-a716-446655440000", "invalid", 1),
		"unsupported type": strings.Replace(validEventJSON(now), "page_view", "click", 1),
		"missing visitor":  strings.Replace(validEventJSON(now), "visitor_random_value", "", 1),
		"oversized title":  strings.Replace(validEventJSON(now), "\"About\"", "\""+strings.Repeat("a", maxTitleLength+1)+"\"", 1),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			testHandler(now, func(context.Context, Event) error { t.Fatal("repository called"); return nil }).Ingest(recorder, request(body))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestIngestMapsProjectAndDuplicateErrors(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	for name, testCase := range map[string]struct {
		err    error
		status int
	}{
		"unknown project":  {ErrUnknownProject, http.StatusNotFound},
		"disabled project": {ErrDisabledProject, http.StatusForbidden},
		"duplicate event":  {ErrDuplicateEvent, http.StatusAccepted},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			testHandler(now, func(context.Context, Event) error { return testCase.err }).Ingest(recorder, request(validEventJSON(now)))
			if recorder.Code != testCase.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestIngestDoesNotExposeRepositoryErrors(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	recorder := httptest.NewRecorder()
	testHandler(now, func(context.Context, Event) error { return errors.New("database connection refused") }).Ingest(recorder, request(validEventJSON(now)))
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "database connection refused") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func testHandler(now time.Time, ingest func(context.Context, Event) error) *Handler {
	service := NewService(&stubRepository{ingest: ingest})
	service.now = func() time.Time { return now }
	return NewHandler(service)
}

func request(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/v1/analytics/events", strings.NewReader(body))
}

func validEventJSON(timestamp time.Time) string {
	return `{"event_id":"550e8400-e29b-41d4-a716-446655440000","type":"page_view","project_id":"dp_test_tracking_id","visitor_id":"visitor_random_value","session_id":"session_random_value","timestamp":"` + timestamp.Format(time.RFC3339) + `","page":{"url":"https://example.com/about","path":"/about","title":"About","referrer":"https://google.com/"},"screen":{"width":1920,"height":1080},"viewport":{"width":1200,"height":800},"language":"en-US","timezone":"Africa/Addis_Ababa","campaign":{"source":"google","medium":"organic","campaign":null,"term":null,"content":null}}`
}
