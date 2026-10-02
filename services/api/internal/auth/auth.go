package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleViewer = "viewer"

	keyByteCount = 32
	keyPrefix    = "dpk_"
)

type contextKey string

const (
	workspaceKey contextKey = "devpulse_workspace_id"
	roleKey      contextKey = "devpulse_role"
)

// KeyFinder resolves a hashed API key to its workspace and role.
// Implementations must return ok=false for unknown or revoked keys
// without distinguishing the reason (avoid key enumeration).
type KeyFinder interface {
	FindKey(ctx context.Context, keyHash string) (workspaceID, role string, ok bool, err error)
}

// GeneratedKey is returned once at creation time. Only keyHash and
// keyPrefix are persisted; raw is shown to the caller exactly once.
type GeneratedKey struct {
	Raw    string
	Prefix string
	Hash   string
}

// Generate creates a new random workspace API key.
func Generate() (GeneratedKey, error) {
	raw := make([]byte, keyByteCount)
	if _, err := rand.Read(raw); err != nil {
		return GeneratedKey{}, err
	}
	encoded := keyPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return GeneratedKey{
		Raw:    encoded,
		Prefix: encoded[:12],
		Hash:   Hash(encoded),
	}, nil
}

// Hash returns the hex-encoded SHA-256 of a raw API key.
func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// EqualHash compares a stored hash with a candidate raw key in
// constant time.
func EqualHash(storedHash, candidateRaw string) bool {
	candidate := Hash(candidateRaw)
	if len(storedHash) != len(candidate) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(candidate)) == 1
}

// WithAuth injects workspace identity into a context (used by middleware
// and tests).
func WithAuth(ctx context.Context, workspaceID, role string) context.Context {
	ctx = context.WithValue(ctx, workspaceKey, workspaceID)
	return context.WithValue(ctx, roleKey, role)
}

// WorkspaceFromContext returns the authenticated workspace ID.
func WorkspaceFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(workspaceKey).(string)
	return id, ok && id != ""
}

// RoleFromContext returns the caller's role within the workspace.
func RoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(roleKey).(string)
	return role
}

// CanWrite reports whether a role may mutate workspace resources.
func CanWrite(role string) bool {
	return role == RoleOwner || role == RoleAdmin
}

// RequireAuth enforces Bearer workspace authentication. Ingestion stays
// public and must NOT use this middleware. When requireWrite is true,
// viewers receive 403; otherwise any valid key may read.
func RequireAuth(finder KeyFinder, requireWrite bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		workspaceID, role, found, err := finder.FindKey(r.Context(), Hash(raw))
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "unable to authenticate")
			return
		}
		if !found {
			writeAuthError(w, http.StatusUnauthorized, "invalid API key")
			return
		}
		if requireWrite && !CanWrite(role) {
			writeAuthError(w, http.StatusForbidden, "viewer role cannot modify resources")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithAuth(r.Context(), workspaceID, role)))
	})
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	return token, token != ""
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
