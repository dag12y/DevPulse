package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExposeRendersCounters(t *testing.T) {
	AddIngested("accepted")
	AddReport("summary")
	AddRetentionRun(false, 10, 2, 1, 3)

	recorder := httptest.NewRecorder()
	Expose(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := recorder.Body.String()
	for _, want := range []string{
		`events_ingested_total{outcome="accepted"}`,
		`report_requests_total{report="summary"}`,
		`retention_runs_total{status="ok"}`,
		`retention_rows_deleted_total{kind="page_views"} 10`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in body:\n%s", want, body)
		}
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("unexpected content type %q", contentType)
	}
}
