package tracker

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeBundle(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTryServeUnversionedShortCache(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, "analytics.js", "/* devpulse */")
	handler := New(dir)

	request := httptest.NewRequest(http.MethodGet, "/analytics.js", nil)
	recorder := httptest.NewRecorder()
	if !handler.TryServe(recorder, request) {
		t.Fatal("expected tracker to serve /analytics.js")
	}
	response := recorder.Result()
	if response.Header.Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("unexpected cache header: %q", response.Header.Get("Cache-Control"))
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "application/javascript; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	if recorder.Body.String() != "/* devpulse */" {
		t.Fatalf("unexpected body: %q", recorder.Body.String())
	}
}

func TestTryServeVersionedImmutable(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, "analytics-0.1.0.js", "/* v */")
	handler := New(dir)

	request := httptest.NewRequest(http.MethodGet, "/analytics-0.1.0.js", nil)
	recorder := httptest.NewRecorder()
	if !handler.TryServe(recorder, request) {
		t.Fatal("expected tracker to serve versioned bundle")
	}
	if got := recorder.Result().Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("unexpected immutable cache header: %q", got)
	}
}

func TestTryServeRejectsTraversalAndNonTracker(t *testing.T) {
	dir := t.TempDir()
	handler := New(dir)

	for _, path := range []string{"/v1/analytics/events", "/analytics.js.map", "/analytics-.js", "/ANALYTICS.JS"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if handler.TryServe(httptest.NewRecorder(), request) {
			t.Fatalf("expected %s to fall through to API mux", path)
		}
	}
}

func TestTryServeMissingBundle404(t *testing.T) {
	handler := New(t.TempDir())
	request := httptest.NewRequest(http.MethodGet, "/analytics.js", nil)
	recorder := httptest.NewRecorder()
	if !handler.TryServe(recorder, request) {
		t.Fatal("expected tracker path to be claimed even when file is missing")
	}
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
}

func TestDisabledWhenDirEmpty(t *testing.T) {
	handler := New("")
	request := httptest.NewRequest(http.MethodGet, "/analytics.js", nil)
	if handler.TryServe(httptest.NewRecorder(), request) {
		t.Fatal("expected disabled tracker to fall through")
	}
}

func TestHasBundle(t *testing.T) {
	if HasBundle(t.TempDir()) {
		t.Fatal("expected empty dir to have no bundle")
	}
	dir := t.TempDir()
	writeBundle(t, dir, "analytics.js", "/* devpulse */")
	if !HasBundle(dir) {
		t.Fatal("expected dir with analytics.js to have a bundle")
	}
	versioned := t.TempDir()
	writeBundle(t, versioned, "analytics-0.1.0.js", "/* devpulse */")
	if !HasBundle(versioned) {
		t.Fatal("expected dir with versioned bundle to have a bundle")
	}
	if HasBundle(filepath.Join(t.TempDir(), "does-not-exist")) {
		t.Fatal("expected missing dir to have no bundle")
	}
}
