package analytics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
)

const testWorkspaceID = "b1eebc99-9c0b-4ef8-bb6d-6bb9bd380b22"

type stubRepository struct {
	ingest    func(context.Context, Event, string) error
	summary   func(context.Context, string, string) (Summary, error)
	traffic   func(context.Context, string, string, int) ([]TrafficPoint, error)
	topPages  func(context.Context, string, string, int) ([]TopPage, error)
	sources   func(context.Context, string, string) ([]Source, error)
	countries func(context.Context, string, string) ([]Country, error)
	devices   func(context.Context, string, string) (Devices, error)
	realtime  func(context.Context, string, string) (Realtime, error)
}

func (stub *stubRepository) Ingest(ctx context.Context, event Event, originHost string) error {
	return stub.ingest(ctx, event, originHost)
}

func (stub *stubRepository) Summary(ctx context.Context, workspaceID, trackingID string) (Summary, error) {
	return stub.summary(ctx, workspaceID, trackingID)
}

func (stub *stubRepository) Traffic(ctx context.Context, workspaceID, trackingID string, days int) ([]TrafficPoint, error) {
	return stub.traffic(ctx, workspaceID, trackingID, days)
}

func (stub *stubRepository) TopPages(ctx context.Context, workspaceID, trackingID string, limit int) ([]TopPage, error) {
	return stub.topPages(ctx, workspaceID, trackingID, limit)
}

func (stub *stubRepository) Sources(ctx context.Context, workspaceID, trackingID string) ([]Source, error) {
	return stub.sources(ctx, workspaceID, trackingID)
}

func (stub *stubRepository) Countries(ctx context.Context, workspaceID, trackingID string) ([]Country, error) {
	return stub.countries(ctx, workspaceID, trackingID)
}

func (stub *stubRepository) Devices(ctx context.Context, workspaceID, trackingID string) (Devices, error) {
	return stub.devices(ctx, workspaceID, trackingID)
}

func (stub *stubRepository) Realtime(ctx context.Context, workspaceID, trackingID string) (Realtime, error) {
	return stub.realtime(ctx, workspaceID, trackingID)
}

func TestIngestAcceptsValidPageView(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	called := false
	handler := testHandler(now, func(_ context.Context, event Event, _ string) error {
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
	testHandler(time.Now(), func(context.Context, Event, string) error { return nil }).Ingest(recorder, request(`{"event_id":`))
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
			testHandler(now, func(context.Context, Event, string) error { t.Fatal("repository called"); return nil }).Ingest(recorder, request(body))
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
		"unknown project":     {ErrUnknownProject, http.StatusNotFound},
		"disabled project":    {ErrDisabledProject, http.StatusForbidden},
		"origin not allowed":  {ErrOriginNotAllowed, http.StatusForbidden},
		"duplicate event":     {ErrDuplicateEvent, http.StatusAccepted},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			testHandler(now, func(context.Context, Event, string) error { return testCase.err }).Ingest(recorder, request(validEventJSON(now)))
			if recorder.Code != testCase.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestIngestDoesNotExposeRepositoryErrors(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	recorder := httptest.NewRecorder()
	testHandler(now, func(context.Context, Event, string) error { return errors.New("database connection refused") }).Ingest(recorder, request(validEventJSON(now)))
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "database connection refused") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestIngestRateLimitsExcessiveEvents(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	handler := testHandler(now, func(context.Context, Event, string) error { return nil })
	handler.limiter = NewLimiter(2, time.Minute)

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		handler.Ingest(recorder, request(validEventJSON(now)))
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("request %d: status=%d body=%s", i+1, recorder.Code, recorder.Body.String())
		}
	}
	recorder := httptest.NewRecorder()
	handler.Ingest(recorder, request(validEventJSON(now)))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func testHandler(now time.Time, ingest func(context.Context, Event, string) error) *Handler {
	service := NewService(&stubRepository{ingest: ingest})
	service.now = func() time.Time { return now }
	return NewHandler(service)
}

func TestIngestEnrichesEventFromRequest(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	var captured Event
	handler := testHandler(now, func(_ context.Context, event Event, _ string) error {
		captured = event
		return nil
	})

	req := request(validEventJSON(now))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.RemoteAddr = "203.0.113.7:51234"
	handler.Ingest(httptest.NewRecorder(), req)

	if captured.Enrichment.DeviceType != "desktop" || captured.Enrichment.Browser != "Chrome" || captured.Enrichment.BrowserVersion != "131.0.0.0" {
		t.Fatalf("unexpected enrichment: %#v", captured.Enrichment)
	}
	if captured.Enrichment.OS != "Windows" || captured.Enrichment.OSVersion != "10" {
		t.Fatalf("unexpected os: %#v", captured.Enrichment)
	}
	if captured.Enrichment.IsBot {
		t.Fatal("desktop Chrome must not be flagged as bot")
	}
}

func TestIngestFlagsBotUserAgent(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	var captured Event
	handler := testHandler(now, func(_ context.Context, event Event, _ string) error {
		captured = event
		return nil
	})

	req := request(validEventJSON(now))
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	handler.Ingest(httptest.NewRecorder(), req)

	if !captured.Enrichment.IsBot || captured.Enrichment.DeviceType != "bot" {
		t.Fatalf("expected bot enrichment: %#v", captured.Enrichment)
	}
}

func TestIngestPassesOriginHost(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	var capturedOrigin string
	handler := testHandler(now, func(_ context.Context, _ Event, originHost string) error {
		capturedOrigin = originHost
		return nil
	})

	req := request(validEventJSON(now))
	req.Header.Set("Origin", "https://example.com")
	handler.Ingest(httptest.NewRecorder(), req)

	if capturedOrigin != "example.com" {
		t.Fatalf("origin host = %q", capturedOrigin)
	}
}

func queryHandler() *Handler {
	return NewHandler(NewService(&stubRepository{
		ingest: func(context.Context, Event, string) error { return nil },
		summary: func(_ context.Context, workspaceID, _ string) (Summary, error) {
			if workspaceID != testWorkspaceID {
				return Summary{}, ErrUnknownProject
			}
			return Summary{TotalPageViews: 10, UniqueVisitors: 4, Sessions: 5, BounceRate: 0.2, AvgSessionDuration: 60}, nil
		},
		traffic: func(_ context.Context, workspaceID, _ string, days int) ([]TrafficPoint, error) {
			if workspaceID != testWorkspaceID {
				return []TrafficPoint{}, ErrUnknownProject
			}
			if days == 99 {
				return []TrafficPoint{}, ErrUnknownProject
			}
			return []TrafficPoint{{Date: "2026-09-30", PageViews: 3, Visitors: 2}}, nil
		},
		topPages: func(_ context.Context, workspaceID, _ string, _ int) ([]TopPage, error) {
			if workspaceID != testWorkspaceID {
				return []TopPage{}, ErrUnknownProject
			}
			return []TopPage{{Path: "/about", Views: 7, UniqueVisitors: 5}}, nil
		},
		sources: func(_ context.Context, workspaceID, _ string) ([]Source, error) {
			if workspaceID != testWorkspaceID {
				return []Source{}, ErrUnknownProject
			}
			return []Source{{Source: "Google", Category: "Organic Search", PageViews: 7, Visitors: 5, Percentage: 70}}, nil
		},
		countries: func(_ context.Context, workspaceID, _ string) ([]Country, error) {
			if workspaceID != testWorkspaceID {
				return []Country{}, ErrUnknownProject
			}
			return []Country{{Country: "ET", PageViews: 7, Visitors: 5, Percentage: 70}}, nil
		},
		devices: func(_ context.Context, workspaceID, _ string) (Devices, error) {
			if workspaceID != testWorkspaceID {
				return Devices{}, ErrUnknownProject
			}
			return Devices{
				DeviceTypes:      []DeviceBreakdown{{Name: "desktop", PageViews: 7, Visitors: 5, Percentage: 70}},
				Browsers:         []DeviceBreakdown{{Name: "Chrome", PageViews: 7, Visitors: 5, Percentage: 100}},
				OperatingSystems: []DeviceBreakdown{{Name: "Windows", PageViews: 7, Visitors: 5, Percentage: 100}},
			}, nil
		},
		realtime: func(_ context.Context, workspaceID, _ string) (Realtime, error) {
			if workspaceID != testWorkspaceID {
				return Realtime{}, ErrUnknownProject
			}
			return Realtime{ActiveVisitors: 2, Pages: []RealtimePage{{Path: "/", Visitors: 2}}}, nil
		},
	}))
}

func authedQuery(target string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	return req.WithContext(auth.WithAuth(req.Context(), testWorkspaceID, auth.RoleAdmin))
}

func TestSummaryReturnsMetrics(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Summary(recorder, authedQuery("/v1/analytics/summary"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, want := range []string{`"total_page_views":10`, `"bounce_rate":0.2`, `"avg_session_duration":60`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("missing %s in body=%s", want, recorder.Body.String())
		}
	}
}

func TestQueryRequiresAuth(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Summary(recorder, httptest.NewRequest(http.MethodGet, "/v1/analytics/summary", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestQueryIsWorkspaceIsolated(t *testing.T) {
	handler := queryHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/summary", nil)
	req = req.WithContext(auth.WithAuth(req.Context(), "other-workspace", auth.RoleAdmin))
	recorder := httptest.NewRecorder()
	handler.Summary(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("cross-workspace read must 404, got status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestTrafficDefaultsTo30Days(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Traffic(recorder, authedQuery("/v1/analytics/traffic"))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"date":"2026-09-30"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestTrafficRejectsInvalidDays(t *testing.T) {
	for _, target := range []string{"/v1/analytics/traffic?days=abc", "/v1/analytics/traffic?days=0", "/v1/analytics/traffic?days=400"} {
		recorder := httptest.NewRecorder()
		queryHandler().Traffic(recorder, authedQuery(target))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("target=%s status=%d body=%s", target, recorder.Code, recorder.Body.String())
		}
	}
}

func TestQueryMapsUnknownProject(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Traffic(recorder, authedQuery("/v1/analytics/traffic?days=99"))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestTopPagesRejectsInvalidLimit(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().TopPages(recorder, authedQuery("/v1/analytics/pages?limit=101"))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestQueryRejectsInvalidTrackingID(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Summary(recorder, authedQuery("/v1/analytics/summary?project_id=bad"))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSourcesReturnsMetrics(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Sources(recorder, authedQuery("/v1/analytics/sources"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, want := range []string{`"source":"Google"`, `"category":"Organic Search"`, `"page_views":7`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("missing %s in body=%s", want, recorder.Body.String())
		}
	}
}

func TestCountriesReturnsMetrics(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Countries(recorder, authedQuery("/v1/analytics/countries"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, want := range []string{`"country":"ET"`, `"page_views":7`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("missing %s in body=%s", want, recorder.Body.String())
		}
	}
}

func TestDevicesReturnsMetrics(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Devices(recorder, authedQuery("/v1/analytics/devices"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, want := range []string{`"device_types"`, `"browsers"`, `"operating_systems"`, `"name":"Chrome"`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("missing %s in body=%s", want, recorder.Body.String())
		}
	}
}

func TestRealtimeReturnsMetrics(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Realtime(recorder, authedQuery("/v1/analytics/realtime"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, want := range []string{`"active_visitors":2`, `"path":"/"`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("missing %s in body=%s", want, recorder.Body.String())
		}
	}
}

func request(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/v1/analytics/events", strings.NewReader(body))
}

func validEventJSON(timestamp time.Time) string {
	return `{"event_id":"550e8400-e29b-41d4-a716-446655440000","type":"page_view","project_id":"dp_test_tracking_id","visitor_id":"visitor_random_value","session_id":"session_random_value","timestamp":"` + timestamp.Format(time.RFC3339) + `","page":{"url":"https://example.com/about","path":"/about","title":"About","referrer":"https://google.com/"},"screen":{"width":1920,"height":1080},"viewport":{"width":1200,"height":800},"language":"en-US","timezone":"Africa/Addis_Ababa","campaign":{"source":"google","medium":"organic","campaign":null,"term":null,"content":null}}`
}
