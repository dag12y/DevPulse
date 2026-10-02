package http

import (
	"net/http"
	"strings"
)

// CORS enforces the configured origin allowlist. An empty list means open
// mode (reflect any origin): convenient for development, forbidden in
// production — main logs a warning when it happens. Preflight responses
// allow the headers the dashboard actually sends, including the auth and
// workspace selectors. Query strings are never part of the decision.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (len(allowed) == 0 || isAllowedOrigin(allowed, origin)) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Workspace-ID")
			w.Header().Set("Access-Control-Expose-Headers", requestIDHeader)
			w.Header().Set("Access-Control-Max-Age", "600")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isAllowedOrigin(allowed map[string]struct{}, origin string) bool {
	_, ok := allowed[origin]
	return ok
}
