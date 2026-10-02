package auth

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

// SessionStore resolves a hashed login token to its user. Expired and
// revoked sessions report ok=false, exactly like unknown tokens.
type SessionStore interface {
	FindSession(ctx context.Context, tokenHash string, now time.Time) (userID string, ok bool, err error)
}

// MembershipStore resolves a user's role in a workspace.
type MembershipStore interface {
	FindMembership(ctx context.Context, userID, workspaceID string) (role string, ok bool, err error)
}

// NowFunc allows tests to fix the clock; production passes time.Now.
type NowFunc func() time.Time

// BearerToken extracts a Bearer token from the Authorization header.
func BearerToken(r *http.Request) (string, bool) {
	return bearerToken(r)
}

// ClientIP extracts the caller IP, preferring the leftmost X-Forwarded-For
// entry and falling back to the connection remote address. Used for rate
// limiting (never stored as analytics data).
func ClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if first, _, _ := strings.Cut(forwarded, ","); strings.TrimSpace(first) != "" {
			return stripPort(strings.TrimSpace(first))
		}
	}
	return stripPort(r.RemoteAddr)
}

func stripPort(address string) string {
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return strings.Trim(address, "[]")
}

// RequireSession enforces human login. It injects the user ID but no
// workspace: workspace-scoped handlers must resolve membership themselves
// (or use RequireAccess).
func RequireSession(sessions SessionStore, now NowFunc, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		userID, found, err := sessions.FindSession(r.Context(), Hash(raw), now().UTC())
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "unable to authenticate")
			return
		}
		if !found {
			writeAuthError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), userID)))
	})
}

// RequireAccess accepts either a workspace API key (automation) or a human
// login session plus workspace selection. Sessions select their workspace
// via X-Workspace-ID (or ?workspace_id=), then membership supplies the
// role — including viewer read-only enforcement when requireWrite is set.
// Handlers downstream keep reading WorkspaceFromContext/RoleFromContext
// unchanged, and UserFromContext distinguishes humans from automation.
func RequireAccess(keys KeyFinder, sessions SessionStore, members MembershipStore, now NowFunc, requireWrite bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if workspaceID, role, found, err := keys.FindKey(r.Context(), Hash(raw)); err != nil {
			writeAuthError(w, http.StatusInternalServerError, "unable to authenticate")
			return
		} else if found {
			if requireWrite && !CanWrite(role) {
				writeAuthError(w, http.StatusForbidden, "viewer role cannot modify resources")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithAuth(r.Context(), workspaceID, role)))
			return
		}
		userID, found, err := sessions.FindSession(r.Context(), Hash(raw), now().UTC())
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "unable to authenticate")
			return
		}
		if !found {
			writeAuthError(w, http.StatusUnauthorized, "invalid API key or session")
			return
		}
		workspaceID := r.Header.Get("X-Workspace-ID")
		if workspaceID == "" {
			workspaceID = r.URL.Query().Get("workspace_id")
		}
		if workspaceID == "" {
			writeAuthError(w, http.StatusUnauthorized, "workspace selection required")
			return
		}
		role, member, err := members.FindMembership(r.Context(), userID, workspaceID)
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "unable to authenticate")
			return
		}
		if !member {
			writeAuthError(w, http.StatusForbidden, "no access to this workspace")
			return
		}
		if requireWrite && !CanWrite(role) {
			writeAuthError(w, http.StatusForbidden, "viewer role cannot modify resources")
			return
		}
		ctx := WithAuth(r.Context(), workspaceID, role)
		next.ServeHTTP(w, r.WithContext(WithUser(ctx, userID)))
	})
}
