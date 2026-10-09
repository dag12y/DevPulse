package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGitHubAuthURL(t *testing.T) {
	provider := &GitHub{ClientID: "cid"}
	location := provider.AuthURL("state123", "challenge456", "https://app.example/api/v1/auth/oauth/github/callback")
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if parsed.Host != "github.com" || parsed.Path != "/login/oauth/authorize" {
		t.Fatalf("authorize endpoint = %s%s", parsed.Host, parsed.Path)
	}
	if query.Get("client_id") != "cid" || query.Get("state") != "state123" ||
		query.Get("code_challenge") != "challenge456" || query.Get("code_challenge_method") != "S256" ||
		query.Get("redirect_uri") != "https://app.example/api/v1/auth/oauth/github/callback" ||
		query.Get("scope") != "read:user user:email" {
		t.Fatalf("authorize query = %v", query)
	}
}

func TestGitHubExchange(t *testing.T) {
	var tokenForm url.Values
	var authHeaders []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/oauth/access_token":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			tokenForm = r.PostForm
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok123"})
		case "/user":
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 42})
		case "/user/emails":
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"email": "alias@example.com", "primary": false, "verified": true},
				{"email": "primary@example.com", "primary": true, "verified": true},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := &GitHub{
		ClientID:     "cid",
		ClientSecret: "sec",
		TokenURL:     server.URL + "/login/oauth/access_token",
		APIURL:       server.URL,
		Client:       server.Client(),
	}
	identity, err := provider.Exchange(context.Background(), "thecode", "theverifier", "https://cb.example/callback")
	if err != nil {
		t.Fatal(err)
	}
	if identity.ProviderUserID != "42" {
		t.Fatalf("provider user id = %q, want 42", identity.ProviderUserID)
	}
	if identity.Email != "primary@example.com" || !identity.EmailVerified {
		t.Fatalf("identity = %+v, want primary verified email", identity)
	}
	if tokenForm.Get("code") != "thecode" || tokenForm.Get("code_verifier") != "theverifier" ||
		tokenForm.Get("redirect_uri") != "https://cb.example/callback" {
		t.Fatalf("token form = %v", tokenForm)
	}
	for _, header := range authHeaders {
		if header != "Bearer tok123" {
			t.Fatalf("api Authorization = %q, want Bearer tok123", header)
		}
	}
}

func TestGitHubExchangeRejectsUnverifiedEmails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/oauth/access_token":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 7})
		case "/user/emails":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"email": "unverified@example.com", "primary": true, "verified": false},
			})
		}
	}))
	defer server.Close()

	provider := &GitHub{
		ClientID: "cid", ClientSecret: "sec",
		TokenURL: server.URL + "/login/oauth/access_token",
		APIURL:   server.URL,
		Client:   server.Client(),
	}
	if _, err := provider.Exchange(context.Background(), "c", "v", "r"); err == nil ||
		!strings.Contains(err.Error(), "verified email") {
		t.Fatalf("err = %v, want verified-email failure", err)
	}
}

func TestGoogleAuthURL(t *testing.T) {
	provider := &Google{ClientID: "gid"}
	location := provider.AuthURL("st", "ch", "https://app.example/api/v1/auth/oauth/google/callback")
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if parsed.Host != "accounts.google.com" {
		t.Fatalf("authorize host = %q", parsed.Host)
	}
	if query.Get("response_type") != "code" || query.Get("code_challenge") != "ch" ||
		query.Get("code_challenge_method") != "S256" || query.Get("state") != "st" ||
		!strings.Contains(query.Get("scope"), "email") {
		t.Fatalf("authorize query = %v", query)
	}
}

func TestGoogleExchange(t *testing.T) {
	var tokenForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			tokenForm = r.PostForm
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "gtok"})
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer gtok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub":            "google-99",
				"email":          "person@example.com",
				"email_verified": true,
			})
		}
	}))
	defer server.Close()

	provider := &Google{
		ClientID: "gid", ClientSecret: "gsec",
		TokenURL:    server.URL + "/token",
		UserInfoURL: server.URL + "/userinfo",
		Client:      server.Client(),
	}
	identity, err := provider.Exchange(context.Background(), "code1", "verifier1", "https://cb.example/callback")
	if err != nil {
		t.Fatal(err)
	}
	if identity.ProviderUserID != "google-99" || identity.Email != "person@example.com" || !identity.EmailVerified {
		t.Fatalf("identity = %+v", identity)
	}
	if tokenForm.Get("code_verifier") != "verifier1" || tokenForm.Get("grant_type") != "authorization_code" {
		t.Fatalf("token form = %v", tokenForm)
	}
}

func TestGoogleExchangeRejectsUnverifiedEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "gtok"})
		case "/userinfo":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub":            "google-1",
				"email":          "u@example.com",
				"email_verified": false,
			})
		}
	}))
	defer server.Close()

	provider := &Google{
		ClientID: "gid", ClientSecret: "gsec",
		TokenURL:    server.URL + "/token",
		UserInfoURL: server.URL + "/userinfo",
		Client:      server.Client(),
	}
	if _, err := provider.Exchange(context.Background(), "c", "v", "r"); err == nil ||
		!strings.Contains(err.Error(), "not verified") {
		t.Fatalf("err = %v, want unverified-email failure", err)
	}
}

func TestGenerateStateAndVerifier(t *testing.T) {
	raw, hash, err := GenerateState()
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || hash != HashState(raw) {
		t.Fatalf("state raw=%q hash=%q", raw, hash)
	}
	raw2, hash2, err := GenerateState()
	if err != nil {
		t.Fatal(err)
	}
	if raw == raw2 || hash == hash2 {
		t.Fatal("state values must be unique")
	}

	verifier, challenge, err := GenerateVerifier()
	if err != nil {
		t.Fatal(err)
	}
	if verifier == "" || challenge == "" || verifier == challenge {
		t.Fatalf("verifier=%q challenge=%q", verifier, challenge)
	}
	// S256: challenge = base64url(sha256(verifier)), raw unpadded.
	if len(challenge) != 43 || strings.ContainsAny(challenge, "=+/") {
		t.Fatalf("challenge %q is not base64url raw sha256", challenge)
	}
}
