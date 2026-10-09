package users

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/dag12y/devpulse/internal/oauth"
)

// oauthStateTTL bounds how long an in-flight login may sit between the
// authorize click and the provider's callback. Short enough that a
// leaked state link ages out quickly, long enough for a slow consent
// screen.
const oauthStateTTL = 10 * time.Minute

// OAuthProviderList reports which providers the API has credentials
// for, so the dashboard can hide buttons for the rest instead of
// linking users to a 404.
func (handler *Handler) OAuthProviderList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"github": handler.oauthProviders["github"] != nil,
		"google": handler.oauthProviders["google"] != nil,
	})
}

// OAuthStart stores a one-time state (plus PKCE verifier) and redirects
// the browser to the provider's consent screen.
func (handler *Handler) OAuthStart(w http.ResponseWriter, r *http.Request) {
	if !handler.oauthLimiter.Allow(auth.ClientIP(r)) {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, "too many sign-in attempts")
		return
	}
	provider := handler.oauthProviders[r.PathValue("provider")]
	if provider == nil {
		writeError(w, http.StatusNotFound, "unknown oauth provider")
		return
	}
	state, stateHash, err := oauth.GenerateState()
	if err != nil {
		slog.Error("generate oauth state", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to start sign-in")
		return
	}
	verifier, challenge, err := oauth.GenerateVerifier()
	if err != nil {
		slog.Error("generate pkce verifier", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to start sign-in")
		return
	}
	redirectPath := sanitizeRedirectPath(r.URL.Query().Get("next"))
	now := handler.now().UTC()
	if err := handler.store.CreateOAuthState(r.Context(), provider.Name(), stateHash, verifier, redirectPath, now, now.Add(oauthStateTTL)); err != nil {
		slog.Error("create oauth state", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to start sign-in")
		return
	}
	http.Redirect(w, r,
		provider.AuthURL(state, challenge, handler.oauthRedirectURI(provider.Name())),
		http.StatusFound)
}

// OAuthCallback finishes the provider round-trip: consume the state,
// exchange the code, resolve (or create) the account, and set the
// session cookie before bouncing the browser back into the app.
func (handler *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := handler.oauthProviders[r.PathValue("provider")]
	if provider == nil {
		writeError(w, http.StatusNotFound, "unknown oauth provider")
		return
	}
	query := r.URL.Query()
	if query.Get("error") != "" {
		// The provider's error string is untrusted; never echo it —
		// only a fixed code reaches the dashboard.
		handler.oauthFail(w, r, "cancelled")
		return
	}
	state, code := query.Get("state"), query.Get("code")
	if state == "" || code == "" {
		writeError(w, http.StatusBadRequest, "sign-in callback is missing parameters")
		return
	}
	stateProvider, verifier, redirectPath, err := handler.store.ConsumeOAuthState(r.Context(), oauth.HashState(state), handler.now().UTC())
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusBadRequest, "sign-in is invalid or has expired")
		return
	}
	if err != nil {
		slog.Error("consume oauth state", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to sign in")
		return
	}
	// A GitHub state replayed against the Google callback must not
	// cross the streams.
	if stateProvider != provider.Name() {
		writeError(w, http.StatusBadRequest, "sign-in is invalid or has expired")
		return
	}
	identity, err := provider.Exchange(r.Context(), code, verifier, handler.oauthRedirectURI(provider.Name()))
	if err != nil {
		slog.Error("oauth exchange failed", "provider", provider.Name(), "error", err)
		handler.oauthFail(w, r, "unable to sign in")
		return
	}
	user, err := handler.oauthUser(r.Context(), provider.Name(), identity)
	if err != nil {
		if errors.Is(err, ErrOAuthConflict) {
			handler.oauthFail(w, r, "account conflict")
			return
		}
		slog.Error("resolve oauth user", "provider", provider.Name(), "error", err)
		handler.oauthFail(w, r, "unable to sign in")
		return
	}
	token, expiresAt, err := handler.newSession(r.Context(), user.ID)
	if err != nil {
		slog.Error("create oauth session", "error", err)
		handler.oauthFail(w, r, "unable to sign in")
		return
	}
	handler.setSessionCookie(w, token, expiresAt)
	http.Redirect(w, r, redirectPath, http.StatusFound)
}

// oauthUser maps a provider identity onto a local account: an existing
// binding short-circuits; otherwise a same-email password account is
// adopted (the provider vouches for the address, so the account is
// verified), and only then is a fresh account created — with its own
// personal workspace, exactly like password registration.
func (handler *Handler) oauthUser(ctx context.Context, provider string, identity oauth.Identity) (*User, error) {
	userID, err := handler.store.FindOAuthAccount(ctx, provider, identity.ProviderUserID)
	if err == nil {
		return handler.store.FindUserByID(ctx, userID)
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	email, err := NormalizeEmail(identity.Email)
	if err != nil {
		return nil, err
	}
	user, _, err := handler.store.FindUserByEmail(ctx, email)
	switch {
	case errors.Is(err, ErrNotFound):
		user, err = handler.store.CreateUser(ctx, email, "")
		if err != nil {
			return nil, err
		}
		if _, err := handler.store.CreateWorkspace(ctx, user.ID, "My workspace"); err != nil {
			return nil, err
		}
		if err := handler.store.SetEmailVerified(ctx, user.ID); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	default:
		// Adopting a password account: a provider-verified address is
		// exactly the evidence email verification asks for.
		if user.EmailVerifiedAt == nil {
			if err := handler.store.SetEmailVerified(ctx, user.ID); err != nil {
				return nil, err
			}
		}
	}
	if err := handler.store.LinkOAuthAccount(ctx, user.ID, provider, identity.ProviderUserID, email); err != nil {
		return nil, err
	}
	return user, nil
}

// oauthRedirectURI is where the provider sends the browser back. It is
// dashboard-origin + /api/…: the BFF proxy forwards the request, and
// the Set-Cookie header on the response travels back through the same
// proxy — one origin, no CORS involved.
func (handler *Handler) oauthRedirectURI(provider string) string {
	return handler.appURL + "/api/v1/auth/oauth/" + provider + "/callback"
}

// oauthFail bounces the browser to the login page with a fixed error
// code (never a raw provider message).
func (handler *Handler) oauthFail(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, handler.appURL+"/login?error="+url.QueryEscape(code), http.StatusFound)
}

// sanitizeRedirectPath keeps the post-login destination on-site: only
// same-origin absolute paths survive (no scheme, no protocol-relative
// "//host", no backslash tricks).
func sanitizeRedirectPath(raw string) string {
	if raw == "" ||
		!strings.HasPrefix(raw, "/") ||
		strings.HasPrefix(raw, "//") ||
		strings.HasPrefix(raw, "/\\") ||
		strings.ContainsAny(raw, "\\\r\n") {
		return "/projects"
	}
	return raw
}
