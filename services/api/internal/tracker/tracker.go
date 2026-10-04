package tracker

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var versionedPattern = regexp.MustCompile(`^/analytics-[A-Za-z0-9][A-Za-z0-9._-]*\.js$`)

// Handler serves the production tracker bundle over HTTPS with cacheable,
// version-immutable URLs. It is public by design: the bundle contains no
// secrets, only the measurement code sites embed with a public tracking ID.
type Handler struct {
	dir string
}

// New returns a Handler serving .js files from dir. An empty dir disables
// serving: TryServe always reports false so the API behaves as before.
func New(dir string) *Handler {
	return &Handler{dir: strings.TrimSpace(dir)}
}

// HasBundle reports whether dir contains at least one analytics bundle
// (analytics.js or a versioned analytics-<version>.js). A configured but
// empty directory serves 404s, which is always a build/packaging mistake.
func HasBundle(dir string) bool {
	matches, err := filepath.Glob(filepath.Join(dir, "analytics*.js"))
	if err != nil {
		return false
	}
	return len(matches) > 0
}

// TryServe serves GET/HEAD requests for /analytics.js (short cache) and
// /analytics-<version>.js (immutable long cache). It reports whether the
// request path belongs to the tracker, even when the file is missing.
func (handler *Handler) TryServe(w http.ResponseWriter, r *http.Request) bool {
	if handler == nil || handler.dir == "" {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	path := r.URL.Path
	if path != "/analytics.js" && !versionedPattern.MatchString(path) {
		return false
	}

	immutable := path != "/analytics.js"
	file := filepath.Join(handler.dir, filepath.Base(path))
	data, err := os.ReadFile(file)
	if err != nil {
		http.Error(w, "tracker bundle not found", http.StatusNotFound)
		return true
	}

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	// The bundle is same-origin safe to load cross-site via <script>.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	if immutable {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
	return true
}
