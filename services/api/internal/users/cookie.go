package users

import (
	"net/http"
	"time"
)

const (
	// sessionCookieName is the browser session cookie in development
	// (plain HTTP): the __Host- prefix would be rejected without Secure.
	sessionCookieName = "dp_session"
	// sessionCookieNameSecure carries the __Host- prefix in production.
	// The browser then refuses any copy of this cookie that declares a
	// Domain, so a sibling subdomain cannot shadow the dashboard session.
	sessionCookieNameSecure = "__Host-dp_session"
)

// setSessionCookie issues the HttpOnly session cookie alongside the JSON
// login response. The dashboard's BFF proxy passes Set-Cookie through, so
// the browser scopes it to the dashboard origin; direct API clients (curl)
// simply ignore it and keep using the token from the response body.
func (handler *Handler) setSessionCookie(w http.ResponseWriter, raw string, expiresAt time.Time) {
	maxAge := int(expiresAt.Sub(handler.now().UTC()).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{
		Name:     handler.sessionCookieName(),
		Value:    raw,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   handler.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie expires the session cookie on logout. MaxAge < 0
// makes browsers delete it immediately.
func (handler *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     handler.sessionCookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   handler.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (handler *Handler) sessionCookieName() string {
	if handler.secureCookies {
		return sessionCookieNameSecure
	}
	return sessionCookieName
}
