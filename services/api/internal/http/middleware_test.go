package http

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIDGeneratedAndPropagated(t *testing.T) {
	var seen string
	next := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r)
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Header().Get(requestIDHeader) == "" {
		t.Fatal("response must carry X-Request-ID")
	}
	if seen != recorder.Header().Get(requestIDHeader) {
		t.Fatalf("context id %q != header id %q", seen, recorder.Header().Get(requestIDHeader))
	}
	if len(seen) != 32 {
		t.Fatalf("generated id %q should be 32 hex chars", seen)
	}
}

func TestRequestIDHonorsSaneClientValue(t *testing.T) {
	next := RequestID(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, candidate := range []string{"abc-123-DEF", "req-1"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(requestIDHeader, candidate)
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, req)
		if recorder.Header().Get(requestIDHeader) != candidate {
			t.Fatalf("client id %q was not propagated", candidate)
		}
	}
}

func TestRequestIDRegeneratesGarbage(t *testing.T) {
	next := RequestID(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, candidate := range []string{"has spaces", "with\nnewline", strings.Repeat("x", 65), "semi;colon"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(requestIDHeader, candidate)
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, req)
		if got := recorder.Header().Get(requestIDHeader); got == candidate || got == "" {
			t.Fatalf("garbage id %q was echoed", candidate)
		}
	}
}

func TestRecoverConvertsPanicToJSON(t *testing.T) {
	next := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "internal server error") {
		t.Fatalf("body=%s", recorder.Body.String())
	}
}

func TestSecurityHeadersPresent(t *testing.T) {
	next := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	for header, want := range map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "no-referrer",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	} {
		if got := recorder.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
	if got := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'none'") {
		t.Fatalf("CSP = %q", got)
	}
}

func TestCORSOpenModeReflectsOrigin(t *testing.T) {
	called := false
	next := CORS(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, req)
	if !called {
		t.Fatal("non-preflight request must reach handler")
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("origin = %q", got)
	}
}

func TestCORSAllowlistEnforced(t *testing.T) {
	next := CORS([]string{"https://app.example.com"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	allowed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.example.com")
	next.ServeHTTP(allowed, req)
	if allowed.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatal("listed origin must be allowed")
	}

	denied := httptest.NewRecorder()
	badReq := httptest.NewRequest(http.MethodGet, "/", nil)
	badReq.Header.Set("Origin", "https://evil.example.com")
	next.ServeHTTP(denied, badReq)
	if denied.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unlisted origin must not be reflected")
	}
	if denied.Code != http.StatusOK {
		t.Fatalf("denied CORS must still serve non-browser callers, status=%d", denied.Code)
	}
}

func TestCORSPreflightShortCircuits(t *testing.T) {
	called := false
	next := CORS([]string{"https://app.example.com"})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, req)
	if called {
		t.Fatal("preflight must not reach the handler")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d", recorder.Code)
	}
	for header, want := range map[string]string{
		"Access-Control-Allow-Origin":   "https://app.example.com",
		"Access-Control-Allow-Headers":  "Content-Type, Authorization, X-Workspace-ID",
		"Access-Control-Expose-Headers": requestIDHeader,
	} {
		if got := recorder.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestRequestLoggerOmitsQueryAndTokens(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(previous)

	next := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/summary?project_id=dp_secret_tracking&days=7", nil)
	req.Header.Set("Authorization", "Bearer dps_secret_token")
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, req)

	line := output.String()
	if !strings.Contains(line, "/v1/analytics/summary") {
		t.Fatalf("path missing from log: %s", line)
	}
	for _, secret := range []string{"dp_secret_tracking", "dps_secret_token", "days=7"} {
		if strings.Contains(line, secret) {
			t.Fatalf("secret leaked into log: %s", line)
		}
	}
}

func TestTimeoutPassesFastHandlers(t *testing.T) {
	next := Timeout(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d", recorder.Code)
	}
}
