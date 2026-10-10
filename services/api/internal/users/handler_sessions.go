package users

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/dag12y/devpulse/internal/auth"
)

// ListSessions returns the caller's live sessions (newest activity
// first), with the row backing the current request marked `current` so
// the dashboard can badge it and withhold its revoke button.
func (handler *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	currentHash := ""
	if raw, hasToken := auth.BearerToken(r); hasToken {
		currentHash = auth.Hash(raw)
	}
	sessions, err := handler.store.ListSessions(r.Context(), userID, currentHash, handler.now().UTC())
	if err != nil {
		slog.Error("list sessions", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to list sessions")
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

// RevokeSession signs out one session by ID. Another user's ID (and an
// unknown one) both answer 404, so the endpoint never confirms whether
// a guessed UUID exists.
func (handler *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	sessionID, ok := pathUUID(w, r, "id", "session id must be a UUID")
	if !ok {
		return
	}
	if err := handler.store.RevokeSessionByID(r.Context(), userID, sessionID); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		slog.Error("revoke session", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to revoke session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RevokeOtherSessions signs out every device except the caller's.
func (handler *Handler) RevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	raw, hasToken := auth.BearerToken(r)
	if !hasToken {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	revoked, err := handler.store.RevokeOtherSessions(r.Context(), userID, auth.Hash(raw))
	if err != nil {
		slog.Error("revoke other sessions", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to revoke sessions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"revoked": revoked})
}
