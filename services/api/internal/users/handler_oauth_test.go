package users

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/dag12y/devpulse/internal/oauth"
)

// stubProvider implements oauth.Provider with canned responses and
// records what the handler sent into the exchange.
type stubProvider struct {
	name        string
	identity    oauth.Identity
	exchangeErr error
	gotCode     string
	gotVerifier string
	gotRedirect string
}

func (provider *stubProvider) Name() string { return provider.name }

func (provider *stubProvider) AuthURL(state, challenge, redirectURI string) string {
	return "https://provider.example/authorize?state=" + url.QueryEscape(state) +
		"&challenge=" + url.QueryEscape(challenge) +
		"&redirect_uri=" + url.QueryEscape(redirectURI)
}

func (provider *stubProvider) Exchange(_ context.Context, code, verifier, redirectURI string) (oauth.Identity, error) {
	provider.gotCode, provider.gotVerifier, provider.gotRedirect = code, verifier, redirectURI
	if provider.exchangeErr != nil {
		return oauth.Identity{}, provider.exchangeErr
	}
	return provider.identity, nil
}

func TestOAuthStartRedirectsAndStoresState(t *testing.T) {
	var storedHash, storedVerifier, storedPath string
	handler := NewHandler(&stubStore{
		createOAuthState: func(_ context.Context, provider, stateHash, verifier, redirectPath string, now, expires time.Time) error {
			if provider != "github" {
				t.Fatalf("state provider = %q, want github", provider)
			}
			if !expires.After(now) {
				t.Fatalf("state expiry %v not after now %v", expires, now)
			}
			storedHash, storedVerifier, storedPath = stateHash, verifier, redirectPath
			return nil
		},
	}, WithOAuthProviders(map[string]oauth.Provider{
		"github": &stubProvider{name: "github"},
	}))

	request := httptest.NewRequest(http.MethodGet, "/v1/auth/oauth/github/start?next=/settings", nil)
	request.SetPathValue("provider", "github")
	recorder := httptest.NewRecorder()
	handler.OAuthStart(recorder, request)

	response := recorder.Result()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302 (body: %s)", response.StatusCode, recorder.Body.String())
	}
	location, err := response.Location()
	if err != nil {
		t.Fatal(err)
	}
	if location.Host != "provider.example" {
		t.Fatalf("redirect host = %q, want provider.example", location.Host)
	}
	rawState := location.Query().Get("state")
	if rawState == "" {
		t.Fatal("authorize URL has no state parameter")
	}
	if storedHash != oauth.HashState(rawState) {
		t.Fatalf("stored hash %q does not match state %q", storedHash, rawState)
	}
	if storedVerifier == "" || location.Query().Get("challenge") == "" {
		t.Fatal("PKCE verifier/challenge missing")
	}
	if storedPath != "/settings" {
		t.Fatalf("stored redirect = %q, want /settings", storedPath)
	}
}

func TestOAuthStartUnknownProvider(t *testing.T) {
	handler := NewHandler(&stubStore{})
	request := httptest.NewRequest(http.MethodGet, "/v1/auth/oauth/github/start", nil)
	request.SetPathValue("provider", "github")
	recorder := httptest.NewRecorder()
	handler.OAuthStart(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

// oauthCallbackStore wires the full store path the callback exercises:
// state consumption, identity resolution, account creation/linking, and
// session issue.
func oauthCallbackStore(t *testing.T, provider, verifier, redirectPath string) *stubStore {
	t.Helper()
	return &stubStore{
		consumeOAuthState: func(_ context.Context, stateHash string, now time.Time) (string, string, string, error) {
			return provider, verifier, redirectPath, nil
		},
		findOAuthAccount: func(context.Context, string, string) (string, error) {
			return "", ErrNotFound
		},
		findUserByEmail: func(context.Context, string) (*User, string, error) {
			return nil, "", ErrNotFound
		},
		createUser: func(_ context.Context, email, hash string) (*User, error) {
			if email != "new@example.com" {
				t.Fatalf("created email = %q, want new@example.com", email)
			}
			if hash != "" {
				t.Fatalf("oauth-created user got password hash %q, want empty", hash)
			}
			return &User{ID: testUserID, Email: email}, nil
		},
		createWorkspace: func(_ context.Context, userID, name string) (Membership, error) {
			return Membership{WorkspaceID: testWorkspaceID, WorkspaceName: name, Role: auth.RoleOwner}, nil
		},
		setEmailVerified: func(context.Context, string) error { return nil },
		linkOAuthAccount: func(_ context.Context, userID, gotProvider, providerUserID, email string) error {
			if userID != testUserID || gotProvider != "github" || providerUserID != "gh-1" || email != "new@example.com" {
				t.Fatalf("link(%s, %s, %s, %s)", userID, gotProvider, providerUserID, email)
			}
			return nil
		},
		createSession: func(_ context.Context, userID string, _ auth.GeneratedKey, _ time.Time, _, _ string) error {
			if userID != testUserID {
				t.Fatalf("session user = %q", userID)
			}
			return nil
		},
	}
}

func TestOAuthCallbackCreatesAccountAndSession(t *testing.T) {
	provider := &stubProvider{
		name: "github",
		identity: oauth.Identity{
			ProviderUserID: "gh-1",
			Email:          "New@Example.com",
			EmailVerified:  true,
		},
	}
	handler := NewHandler(oauthCallbackStore(t, "github", "verifier", "/projects"),
		WithOAuthProviders(map[string]oauth.Provider{"github": provider}))

	request := httptest.NewRequest(http.MethodGet,
		"/v1/auth/oauth/github/callback?state=somestate&code=thecode", nil)
	request.SetPathValue("provider", "github")
	recorder := httptest.NewRecorder()
	handler.OAuthCallback(recorder, request)

	response := recorder.Result()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302 (body: %s)", response.StatusCode, recorder.Body.String())
	}
	if got := response.Header.Get("Location"); got != "/projects" {
		t.Fatalf("redirect = %q, want /projects", got)
	}
	setCookie := response.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, sessionCookieName+"=") {
		t.Fatalf("Set-Cookie = %q, want %s", setCookie, sessionCookieName)
	}
	if !strings.Contains(setCookie, "HttpOnly") {
		t.Fatalf("Set-Cookie = %q, want HttpOnly", setCookie)
	}
	if provider.gotCode != "thecode" || provider.gotVerifier != "verifier" {
		t.Fatalf("exchange got code=%q verifier=%q", provider.gotCode, provider.gotVerifier)
	}
	if want := "http://localhost:3000/api/v1/auth/oauth/github/callback"; provider.gotRedirect != want {
		t.Fatalf("redirect_uri = %q, want %q", provider.gotRedirect, want)
	}
}

func TestOAuthCallbackAdoptsExistingPasswordAccount(t *testing.T) {
	provider := &stubProvider{
		name:     "github",
		identity: oauth.Identity{ProviderUserID: "gh-2", Email: "existing@example.com", EmailVerified: true},
	}
	var verifiedUserID, linkedUserID string
	store := &stubStore{
		consumeOAuthState: func(context.Context, string, time.Time) (string, string, string, error) {
			return "github", "verifier", "/", nil
		},
		findOAuthAccount: func(context.Context, string, string) (string, error) {
			return "", ErrNotFound
		},
		findUserByEmail: func(_ context.Context, email string) (*User, string, error) {
			// Unverified password account: OAuth must verify it.
			return &User{ID: testUserID, Email: email}, "somehash", nil
		},
		setEmailVerified: func(_ context.Context, userID string) error {
			verifiedUserID = userID
			return nil
		},
		createUser: func(context.Context, string, string) (*User, error) {
			t.Fatal("existing account must not be recreated")
			return nil, nil
		},
		linkOAuthAccount: func(_ context.Context, userID, _, _, _ string) error {
			linkedUserID = userID
			return nil
		},
		createSession: func(context.Context, string, auth.GeneratedKey, time.Time, string, string) error { return nil },
	}
	handler := NewHandler(store, WithOAuthProviders(map[string]oauth.Provider{"github": provider}))

	request := httptest.NewRequest(http.MethodGet, "/v1/auth/oauth/github/callback?state=s&code=c", nil)
	request.SetPathValue("provider", "github")
	recorder := httptest.NewRecorder()
	handler.OAuthCallback(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 (body: %s)", recorder.Code, recorder.Body.String())
	}
	if verifiedUserID != testUserID || linkedUserID != testUserID {
		t.Fatalf("verified=%q linked=%q, want %s", verifiedUserID, linkedUserID, testUserID)
	}
}

func TestOAuthCallbackExistingBindingSkipsLinking(t *testing.T) {
	provider := &stubProvider{
		name:     "google",
		identity: oauth.Identity{ProviderUserID: "g-9", Email: "bound@example.com", EmailVerified: true},
	}
	store := &stubStore{
		consumeOAuthState: func(context.Context, string, time.Time) (string, string, string, error) {
			return "google", "verifier", "/", nil
		},
		findOAuthAccount: func(_ context.Context, gotProvider, providerUserID string) (string, error) {
			if gotProvider != "google" || providerUserID != "g-9" {
				t.Fatalf("lookup (%s, %s)", gotProvider, providerUserID)
			}
			return testUserID, nil
		},
		findUserByID: func(_ context.Context, userID string) (*User, error) {
			return &User{ID: userID, Email: "bound@example.com"}, nil
		},
		linkOAuthAccount: func(context.Context, string, string, string, string) error {
			t.Fatal("existing binding must not re-link")
			return nil
		},
		createSession: func(context.Context, string, auth.GeneratedKey, time.Time, string, string) error { return nil },
	}
	handler := NewHandler(store, WithOAuthProviders(map[string]oauth.Provider{"google": provider}))

	request := httptest.NewRequest(http.MethodGet, "/v1/auth/oauth/google/callback?state=s&code=c", nil)
	request.SetPathValue("provider", "google")
	recorder := httptest.NewRecorder()
	handler.OAuthCallback(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 (body: %s)", recorder.Code, recorder.Body.String())
	}
}

func TestOAuthCallbackInvalidState(t *testing.T) {
	handler := NewHandler(&stubStore{
		consumeOAuthState: func(context.Context, string, time.Time) (string, string, string, error) {
			return "", "", "", ErrNotFound
		},
	}, WithOAuthProviders(map[string]oauth.Provider{"github": &stubProvider{name: "github"}}))

	request := httptest.NewRequest(http.MethodGet, "/v1/auth/oauth/github/callback?state=bad&code=c", nil)
	request.SetPathValue("provider", "github")
	recorder := httptest.NewRecorder()
	handler.OAuthCallback(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestOAuthCallbackProviderErrorRedirects(t *testing.T) {
	handler := NewHandler(&stubStore{}, WithOAuthProviders(map[string]oauth.Provider{
		"github": &stubProvider{name: "github"},
	}))

	request := httptest.NewRequest(http.MethodGet,
		"/v1/auth/oauth/github/callback?error=access_denied&error_description=User+said+no", nil)
	request.SetPathValue("provider", "github")
	recorder := httptest.NewRecorder()
	handler.OAuthCallback(recorder, request)

	response := recorder.Result()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", response.StatusCode)
	}
	location, err := response.Location()
	if err != nil {
		t.Fatal(err)
	}
	if location.Path != "/login" || location.Query().Get("error") != "cancelled" {
		t.Fatalf("redirect = %v, want /login?error=cancelled", location)
	}
	if strings.Contains(recorder.Body.String(), "access_denied") ||
		strings.Contains(location.String(), "access_denied") {
		t.Fatal("provider error string leaked into the redirect")
	}
}

func TestOAuthCallbackExchangeFailure(t *testing.T) {
	provider := &stubProvider{name: "github", exchangeErr: errors.New("provider exploded")}
	handler := NewHandler(&stubStore{
		consumeOAuthState: func(context.Context, string, time.Time) (string, string, string, error) {
			return "github", "verifier", "/", nil
		},
	}, WithOAuthProviders(map[string]oauth.Provider{"github": provider}))

	request := httptest.NewRequest(http.MethodGet, "/v1/auth/oauth/github/callback?state=s&code=c", nil)
	request.SetPathValue("provider", "github")
	recorder := httptest.NewRecorder()
	handler.OAuthCallback(recorder, request)

	response := recorder.Result()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", recorder.Code)
	}
	location, _ := response.Location()
	if location.Query().Get("error") != "unable to sign in" {
		t.Fatalf("error = %q, want unable to sign in", location.Query().Get("error"))
	}
}

func TestOAuthProviderList(t *testing.T) {
	handler := NewHandler(&stubStore{}, WithOAuthProviders(map[string]oauth.Provider{
		"github": &stubProvider{name: "github"},
	}))
	recorder := httptest.NewRecorder()
	handler.OAuthProviderList(recorder, httptest.NewRequest(http.MethodGet, "/v1/auth/oauth/providers", nil))
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `"github":true`) || !strings.Contains(body, `"google":false`) {
		t.Fatalf("list = %d %s", recorder.Code, body)
	}
}

func TestSanitizeRedirectPath(t *testing.T) {
	cases := map[string]string{
		"/projects":          "/projects",
		"":                   "/projects",
		"/":                  "/",
		"//evil.com":         "/projects",
		"/\\evil.com":        "/projects",
		"https://x":          "/projects",
		"/a\r\nb":            "/projects",
		"/settings?tab=keys": "/settings?tab=keys",
	}
	for input, want := range cases {
		if got := sanitizeRedirectPath(input); got != want {
			t.Fatalf("sanitizeRedirectPath(%q) = %q, want %q", input, got, want)
		}
	}
}
