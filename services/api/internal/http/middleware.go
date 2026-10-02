package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
)

// Recover converts panics into 500 JSON responses instead of killing the
// process (and the connection without a body). The request ID is logged
// so the crash can be correlated.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic recovered",
					"request_id", RequestIDFromContext(r),
					"method", r.Method,
					"path", r.URL.Path,
					"panic", recovered)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders sets baseline protections for a JSON API with no UI:
// no sniffing, no framing, no referrer leakage, and HSTS (honored only
// over HTTPS, so local HTTP development is unaffected).
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

// RequestLogger emits one structured line per request. The path is logged
// without the query string so tracking IDs and workspace selectors never
// land in logs; bodies and tokens are never logged.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		slog.Info("http request",
			"request_id", RequestIDFromContext(r),
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", auth.ClientIP(r))
	})
}

// Timeout caps slow handlers with a JSON 408-shaped 503. Dashboard queries
// target <500ms; 25s is a backstop against hung database calls, not a
// performance goal.
func Timeout(next http.Handler) http.Handler {
	return http.TimeoutHandler(next, 25*time.Second, `{"error":"request timed out"}`)
}
