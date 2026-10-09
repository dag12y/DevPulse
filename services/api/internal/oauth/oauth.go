// Package oauth implements the GitHub and Google login providers: the
// authorize redirect, the PKCE code exchange, and identity extraction.
// Only provider-verified emails yield an Identity — an unverified
// address must never become (or link to) a local account.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Identity is the subset of a provider profile DevPulse trusts enough
// to create or link an account.
type Identity struct {
	ProviderUserID string
	Email          string
	EmailVerified  bool
}

// Provider is one OAuth login integration.
type Provider interface {
	Name() string
	// AuthURL builds the authorize redirect for an in-flight login.
	AuthURL(state, codeChallenge, redirectURI string) string
	// Exchange trades the callback code for an identity.
	Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (Identity, error)
}

const defaultHTTPTimeout = 10 * time.Second

func sharedClient() *http.Client { return &http.Client{Timeout: defaultHTTPTimeout} }

// GitHub implements login via github.com. Email comes from the
// /user/emails list (the profile's public email may be private).
type GitHub struct {
	ClientID     string
	ClientSecret string
	// Endpoint fields exist so tests can point the provider at a stub
	// server; empty values use the production defaults.
	AuthorizeURL string
	TokenURL     string
	APIURL       string
	Client       *http.Client
}

func (provider *GitHub) Name() string { return "github" }

func (provider *GitHub) AuthURL(state, codeChallenge, redirectURI string) string {
	endpoint := provider.AuthorizeURL
	if endpoint == "" {
		endpoint = "https://github.com/login/oauth/authorize"
	}
	query := url.Values{
		"client_id":             {provider.ClientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"scope":                 {"read:user user:email"},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	return endpoint + "?" + query.Encode()
}

func (provider *GitHub) Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (Identity, error) {
	tokenURL := provider.TokenURL
	if tokenURL == "" {
		tokenURL = "https://github.com/login/oauth/access_token"
	}
	form := url.Values{
		"client_id":     {provider.ClientID},
		"client_secret": {provider.ClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {codeVerifier},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Identity{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	client := provider.client()
	response, err := client.Do(request)
	if err != nil {
		return Identity{}, fmt.Errorf("github token exchange: %w", err)
	}
	defer response.Body.Close()
	var token struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return Identity{}, fmt.Errorf("decode github token response: %w", err)
	}
	if response.StatusCode != http.StatusOK || token.AccessToken == "" {
		if token.Error != "" {
			return Identity{}, fmt.Errorf("github token exchange: %s", token.Error)
		}
		return Identity{}, fmt.Errorf("github token exchange: status %d", response.StatusCode)
	}

	apiURL := provider.APIURL
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}
	var user struct {
		ID json.Number `json:"id"`
	}
	if err := provider.getJSON(ctx, client, apiURL+"/user", token.AccessToken, &user); err != nil {
		return Identity{}, err
	}
	if user.ID.String() == "" {
		return Identity{}, fmt.Errorf("github user response missing id")
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := provider.getJSON(ctx, client, apiURL+"/user/emails", token.AccessToken, &emails); err != nil {
		return Identity{}, err
	}

	identity := Identity{ProviderUserID: user.ID.String()}
	for _, entry := range emails {
		if entry.Verified && (identity.Email == "" || entry.Primary) {
			identity.Email = entry.Email
			identity.EmailVerified = true
			if entry.Primary {
				break
			}
		}
	}
	if identity.Email == "" {
		return Identity{}, fmt.Errorf("github account has no verified email")
	}
	return identity, nil
}

func (provider *GitHub) client() *http.Client {
	if provider.Client != nil {
		return provider.Client
	}
	return sharedClient()
}

func (provider *GitHub) getJSON(ctx context.Context, client *http.Client, endpoint, accessToken string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "DevPulse")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("github api %s: %w", endpoint, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("github api %s: status %d", endpoint, response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode github api response: %w", err)
	}
	return nil
}

// Google implements login via OpenID Connect. The identity comes from
// the userinfo endpoint, which reflects the token's live claims
// (including email_verified) instead of a possibly stale ID token.
type Google struct {
	ClientID     string
	ClientSecret string
	// Endpoint fields exist so tests can point the provider at a stub
	// server; empty values use the production defaults.
	AuthorizeURL string
	TokenURL     string
	UserInfoURL  string
	Client       *http.Client
}

func (provider *Google) Name() string { return "google" }

func (provider *Google) AuthURL(state, codeChallenge, redirectURI string) string {
	endpoint := provider.AuthorizeURL
	if endpoint == "" {
		endpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	query := url.Values{
		"client_id":             {provider.ClientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	return endpoint + "?" + query.Encode()
}

func (provider *Google) Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (Identity, error) {
	tokenURL := provider.TokenURL
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}
	form := url.Values{
		"client_id":     {provider.ClientID},
		"client_secret": {provider.ClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {codeVerifier},
		"grant_type":    {"authorization_code"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Identity{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := provider.client()
	response, err := client.Do(request)
	if err != nil {
		return Identity{}, fmt.Errorf("google token exchange: %w", err)
	}
	defer response.Body.Close()
	var token struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return Identity{}, fmt.Errorf("decode google token response: %w", err)
	}
	if response.StatusCode != http.StatusOK || token.AccessToken == "" {
		if token.Error != "" {
			return Identity{}, fmt.Errorf("google token exchange: %s (%s)", token.Error, token.ErrorDesc)
		}
		return Identity{}, fmt.Errorf("google token exchange: status %d", response.StatusCode)
	}

	userInfoURL := provider.UserInfoURL
	if userInfoURL == "" {
		userInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	}
	infoRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return Identity{}, err
	}
	infoRequest.Header.Set("Authorization", "Bearer "+token.AccessToken)
	infoResponse, err := client.Do(infoRequest)
	if err != nil {
		return Identity{}, fmt.Errorf("google userinfo: %w", err)
	}
	defer infoResponse.Body.Close()
	if infoResponse.StatusCode != http.StatusOK {
		return Identity{}, fmt.Errorf("google userinfo: status %d", infoResponse.StatusCode)
	}
	var info struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.NewDecoder(io.LimitReader(infoResponse.Body, 1<<20)).Decode(&info); err != nil {
		return Identity{}, fmt.Errorf("decode google userinfo: %w", err)
	}
	if info.Sub == "" || info.Email == "" {
		return Identity{}, fmt.Errorf("google userinfo missing sub or email")
	}
	if !info.EmailVerified {
		return Identity{}, fmt.Errorf("google account email is not verified")
	}
	return Identity{ProviderUserID: info.Sub, Email: info.Email, EmailVerified: true}, nil
}

func (provider *Google) client() *http.Client {
	if provider.Client != nil {
		return provider.Client
	}
	return sharedClient()
}

// GenerateState returns a raw CSRF state value and the hash stored
// server-side (the raw value only ever travels to the provider).
func GenerateState() (raw, hash string, err error) {
	raw, err = randomValue()
	if err != nil {
		return "", "", err
	}
	return raw, HashState(raw), nil
}

// HashState is the server-side digest of a raw state value.
func HashState(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// GenerateVerifier returns a PKCE code_verifier and its S256 challenge.
func GenerateVerifier() (verifier, challenge string, err error) {
	verifier, err = randomValue()
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func randomValue() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
