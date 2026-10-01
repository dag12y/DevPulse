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
	ingest    func(context.Context, Event) error
	summary   func(context.Context, string) (Summary, error)
	traffic   func(context.Context, string, int) ([]TrafficPoint, error)
	topPages  func(context.Context, string, int) ([]TopPage, error)
	sources   func(context.Context, string) ([]Source, error)
	countries func(context.Context, string) ([]Country, error)
	devices   func(context.Context, string) (Devices, error)
	realtime  func(context.Context, string) (Realtime, error)
}

func (stub *stubRepository) Ingest(ctx context.Context, event Event) error {
	return stub.ingest(ctx, event)
}

func (stub *stubRepository) Summary(ctx context.Context, trackingID string) (Summary, error) {
	return stub.summary(ctx, trackingID)
}

func (stub *stubRepository) Traffic(ctx context.Context, trackingID string, days int) ([]TrafficPoint, error) {
	return stub.traffic(ctx, trackingID, days)
}

func (stub *stubRepository) TopPages(ctx context.Context, trackingID string, limit int) ([]TopPage, error) {
	return stub.topPages(ctx, trackingID, limit)
}

func (stub *stubRepository) Sources(ctx context.Context, trackingID string) ([]Source, error) {
	return stub.sources(ctx, trackingID)
}

func (stub *stubRepository) Countries(ctx context.Context, trackingID string) ([]Country, error) {
	return stub.countries(ctx, trackingID)
}

func (stub *stubRepository) Devices(ctx context.Context, trackingID string) (Devices, error) {
	return stub.devices(ctx, trackingID)
}

func (stub *stubRepository) Realtime(ctx context.Context, trackingID string) (Realtime, error) {
	return stub.realtime(ctx, trackingID)
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

func TestIngestRateLimitsExcessiveEvents(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	handler := testHandler(now, func(context.Context, Event) error { return nil })
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

func testHandler(now time.Time, ingest func(context.Context, Event) error) *Handler {
	service := NewService(&stubRepository{ingest: ingest})
	service.now = func() time.Time { return now }
	return NewHandler(service)
}

func TestIngestEnrichesEventFromRequest(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	var captured Event
	handler := testHandler(now, func(_ context.Context, event Event) error {
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
	handler := testHandler(now, func(_ context.Context, event Event) error {
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

func queryHandler() *Handler {
	return NewHandler(NewService(&stubRepository{
		ingest: func(context.Context, Event) error { return nil },
		summary: func(context.Context, string) (Summary, error) {
			return Summary{TotalPageViews: 10, UniqueVisitors: 4, Sessions: 5, BounceRate: 0.2, AvgSessionDuration: 60}, nil
		},
		traffic: func(_ context.Context, _ string, days int) ([]TrafficPoint, error) {
			if days == 99 {
				return []TrafficPoint{}, ErrUnknownProject
			}
			return []TrafficPoint{{Date: "2026-09-30", PageViews: 3, Visitors: 2}}, nil
		},
		topPages: func(context.Context, string, int) ([]TopPage, error) {
			return []TopPage{{Path: "/about", Views: 7, UniqueVisitors: 5}}, nil
		},
		sources: func(context.Context, string) ([]Source, error) {
			return []Source{{Source: "Google", Category: "Organic Search", PageViews: 7, Visitors: 5, Percentage: 70}}, nil
		},
		countries: func(context.Context, string) ([]Country, error) {
			return []Country{{Country: "ET", PageViews: 7, Visitors: 5, Percentage: 70}}, nil
		},
		devices: func(context.Context, string) (Devices, error) {
			return Devices{
				DeviceTypes:      []DeviceBreakdown{{Name: "desktop", PageViews: 7, Visitors: 5, Percentage: 70}},
				Browsers:         []DeviceBreakdown{{Name: "Chrome", PageViews: 7, Visitors: 5, Percentage: 100}},
				OperatingSystems: []DeviceBreakdown{{Name: "Windows", PageViews: 7, Visitors: 5, Percentage: 100}},
			}, nil
		},
		realtime: func(context.Context, string) (Realtime, error) {
			return Realtime{ActiveVisitors: 2, Pages: []RealtimePage{{Path: "/", Visitors: 2}}}, nil
		},
	}))
}

func TestSummaryReturnsMetrics(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Summary(recorder, httptest.NewRequest(http.MethodGet, "/v1/analytics/summary", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, want := range []string{`"total_page_views":10`, `"bounce_rate":0.2`, `"avg_session_duration":60`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("missing %s in body=%s", want, recorder.Body.String())
		}
	}
}

func TestTrafficDefaultsTo30Days(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Traffic(recorder, httptest.NewRequest(http.MethodGet, "/v1/analytics/traffic", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"date":"2026-09-30"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestTrafficRejectsInvalidDays(t *testing.T) {
	for _, target := range []string{"/v1/analytics/traffic?days=abc", "/v1/analytics/traffic?days=0", "/v1/analytics/traffic?days=400"} {
		recorder := httptest.NewRecorder()
		queryHandler().Traffic(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("target=%s status=%d body=%s", target, recorder.Code, recorder.Body.String())
		}
	}
}

func TestQueryMapsUnknownProject(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Traffic(recorder, httptest.NewRequest(http.MethodGet, "/v1/analytics/traffic?days=99", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestTopPagesRejectsInvalidLimit(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().TopPages(recorder, httptest.NewRequest(http.MethodGet, "/v1/analytics/pages?limit=101", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestQueryRejectsInvalidTrackingID(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Summary(recorder, httptest.NewRequest(http.MethodGet, "/v1/analytics/summary?project_id=bad", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSourcesReturnsMetrics(t *testing.T) {
	recorder := httptest.NewRecorder()
	queryHandler().Sources(recorder, httptest.NewRequest(http.MethodGet, "/v1/analytics/sources", nil))
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
	queryHandler().Countries(recorder, httptest.NewRequest(http.MethodGet, "/v1/analytics/countries", nil))
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
	queryHandler().Devices(recorder, httptest.NewRequest(http.MethodGet, "/v1/analytics/devices", nil))
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
	queryHandler().Realtime(recorder, httptest.NewRequest(http.MethodGet, "/v1/analytics/realtime", nil))
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
