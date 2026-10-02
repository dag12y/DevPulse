package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

// requestIDHeader carries the ID in both directions so dashboard error
// reports can reference the exact server-side log lines.
const requestIDHeader = "X-Request-ID"

type contextKey string

const requestIDKey contextKey = "devpulse_request_id"

// RequestIDFromContext returns the ID assigned to this request.
func RequestIDFromContext(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey).(string)
	return id
}

// RequestID assigns every request an ID: a client-supplied one when it
// looks sane, otherwise a fresh random value. The ID is echoed back so
// callers can quote it in bug reports.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := sanitizeRequestID(r.Header.Get(requestIDHeader))
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func sanitizeRequestID(candidate string) string {
	if len(candidate) == 0 || len(candidate) > 64 {
		return ""
	}
	for _, character := range candidate {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' {
			return ""
		}
	}
	return candidate
}

func newRequestID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(value)
}
