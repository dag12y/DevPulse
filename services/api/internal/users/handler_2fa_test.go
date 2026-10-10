package users

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/dag12y/devpulse/internal/totp"
)

// totpCodeFor returns the live code for secret at now, so tests exercise
// Validate against real time rather than stubbing it.
func totpCodeFor(t *testing.T, secret string, now time.Time) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestTwoFactorSetupEnableDisable(t *testing.T) {
	var storedSecret string
	enabled := false
	handler := NewHandler(&stubStore{
		findUserByID: func(context.Context, string) (*User, error) {
			return &User{ID: testUserID, Email: "ada@example.com", TOTPEnabled: enabled}, nil
		},
		setUserTOTPSecret: func(_ context.Context, _, secret string) error {
			storedSecret = secret
			return nil
		},
		getUserTOTP: func(context.Context, string) (string, bool, error) {
			return storedSecret, enabled, nil
		},
		enableUserTOTP: func(context.Context, string) error {
			enabled = true
			return nil
		},
		disableUserTOTP: func(context.Context, string) error {
			enabled = false
			storedSecret = ""
			return nil
		},
	})

	// Setup returns a usable secret, URL, and QR.
	recorder := httptest.NewRecorder()
	handler.TwoFactorSetup(recorder, authedRequest(t, http.MethodPost, "/v1/auth/2fa/setup", "{}"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("setup status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var setup struct {
		Secret     string `json:"secret"`
		OtpauthURL string `json:"otpauth_url"`
		QRBase64   string `json:"qr_png_base64"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &setup); err != nil {
		t.Fatal(err)
	}
	if setup.Secret == "" || !strings.HasPrefix(setup.OtpauthURL, "otpauth://totp/") || setup.QRBase64 == "" {
		t.Fatalf("setup payload incomplete: %+v", setup)
	}
	if storedSecret != setup.Secret {
		t.Fatalf("stored secret = %q, want %q", storedSecret, setup.Secret)
	}

	// Wrong code is refused before anything flips.
	recorder = httptest.NewRecorder()
	handler.TwoFactorEnable(recorder, authedRequest(t, http.MethodPost, "/v1/auth/2fa/enable", `{"code":"000000"}`))
	if recorder.Code == http.StatusOK || enabled {
		t.Fatalf("enable with bad code: status = %d, enabled = %v", recorder.Code, enabled)
	}

	// Live code enables.
	code := totpCodeFor(t, setup.Secret, time.Now())
	recorder = httptest.NewRecorder()
	handler.TwoFactorEnable(recorder, authedRequest(t, http.MethodPost, "/v1/auth/2fa/enable", `{"code":"`+code+`"}`))
	if recorder.Code != http.StatusOK || !enabled {
		t.Fatalf("enable: status = %d, enabled = %v, body = %s", recorder.Code, enabled, recorder.Body.String())
	}

	// Setup while enabled conflicts.
	recorder = httptest.NewRecorder()
	handler.TwoFactorSetup(recorder, authedRequest(t, http.MethodPost, "/v1/auth/2fa/setup", "{}"))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("setup while enabled: status = %d", recorder.Code)
	}

	// Disable requires a live code too.
	code = totpCodeFor(t, setup.Secret, time.Now())
	recorder = httptest.NewRecorder()
	handler.TwoFactorDisable(recorder, authedRequest(t, http.MethodPost, "/v1/auth/2fa/disable", `{"code":"000000"}`))
	if recorder.Code == http.StatusOK || !enabled {
		t.Fatalf("disable with bad code: status = %d, enabled = %v", recorder.Code, enabled)
	}
	recorder = httptest.NewRecorder()
	handler.TwoFactorDisable(recorder, authedRequest(t, http.MethodPost, "/v1/auth/2fa/disable", `{"code":"`+code+`"}`))
	if recorder.Code != http.StatusOK || enabled {
		t.Fatalf("disable: status = %d, enabled = %v, body = %s", recorder.Code, enabled, recorder.Body.String())
	}
}

func TestLoginRequiresTwoFactorCode(t *testing.T) {
	secret, err := totp.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(&stubStore{
		findUserByEmail: func(context.Context, string) (*User, string, error) {
			verified := time.Now().UTC()
			return &User{ID: testUserID, Email: "ada@example.com", EmailVerifiedAt: &verified, TOTPEnabled: true},
				testBcryptHash(t, "correct-horse-12"), nil
		},
		getUserTOTP: func(context.Context, string) (string, bool, error) {
			return secret, true, nil
		},
		createSession: func(context.Context, string, auth.GeneratedKey, time.Time, string, string) error { return nil },
		listWorkspaces: func(context.Context, string) ([]Membership, error) {
			return []Membership{}, nil
		},
	})
	loginBody := `{"email":"ada@example.com","password":"correct-horse-12"}`

	// No code → 401 asking for it, no session.
	recorder := httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(loginBody)))
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "two-factor code required") {
		t.Fatalf("missing code: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	// Wrong code → 401, still no session.
	recorder = httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"ada@example.com","password":"correct-horse-12","totp_code":"000000"}`)))
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "invalid two-factor code") {
		t.Fatalf("wrong code: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	// Live code → session.
	code := totpCodeFor(t, secret, time.Now())
	recorder = httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"ada@example.com","password":"correct-horse-12","totp_code":"`+code+`"}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("live code: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestLoginLocksAfterThresholdFailures(t *testing.T) {
	failures := 0
	var lockedUntil time.Time
	handler := NewHandler(&stubStore{
		findUserByEmail: func(context.Context, string) (*User, string, error) {
			verified := time.Now().UTC()
			return &User{ID: testUserID, Email: "ada@example.com", EmailVerifiedAt: &verified},
				testBcryptHash(t, "correct-horse-12"), nil
		},
		accountLockState: func(_ context.Context, _ string, now time.Time) (time.Time, error) {
			if lockedUntil.After(now) {
				return lockedUntil, nil
			}
			return time.Time{}, nil
		},
		recordFailedLogin: func(_ context.Context, _ string, now time.Time, threshold int, lockUntil time.Time) (time.Time, error) {
			failures++
			if failures >= threshold {
				lockedUntil = lockUntil
				return lockUntil, nil
			}
			return time.Time{}, nil
		},
		createSession:  func(context.Context, string, auth.GeneratedKey, time.Time, string, string) error { return nil },
		listWorkspaces: func(context.Context, string) ([]Membership, error) { return []Membership{}, nil },
	})

	attempt := func(password string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
			strings.NewReader(`{"email":"ada@example.com","password":"`+password+`"}`)))
		return recorder
	}

	// Below threshold: plain 401s.
	for range loginLockoutThreshold - 1 {
		recorder := attempt("wrong-password-1")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("pre-threshold status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
	}

	// Threshold crossing flips to 429 + Retry-After.
	recorder := attempt("wrong-password-1")
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("threshold status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Fatal("locked response missing Retry-After")
	}

	// Locked account: even the CORRECT password is refused while locked.
	recorder = attempt("correct-horse-12")
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("locked status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	// Unlock (lock expired): the correct password signs in again.
	lockedUntil = time.Time{}
	recorder = attempt("correct-horse-12")
	if recorder.Code != http.StatusOK {
		t.Fatalf("after unlock: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestLoginClearsLockoutOnSuccess(t *testing.T) {
	cleared := false
	handler := NewHandler(&stubStore{
		findUserByEmail: func(context.Context, string) (*User, string, error) {
			verified := time.Now().UTC()
			return &User{ID: testUserID, Email: "ada@example.com", EmailVerifiedAt: &verified},
				testBcryptHash(t, "correct-horse-12"), nil
		},
		clearLoginFailures: func(context.Context, string) error {
			cleared = true
			return nil
		},
		createSession:  func(context.Context, string, auth.GeneratedKey, time.Time, string, string) error { return nil },
		listWorkspaces: func(context.Context, string) ([]Membership, error) { return []Membership{}, nil },
	})
	recorder := httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"ada@example.com","password":"correct-horse-12"}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !cleared {
		t.Fatal("successful login did not clear lockout counters")
	}
}

func TestListSessionsMarksCurrentAndRevokeScoping(t *testing.T) {
	currentHash := auth.Hash("dps_current")
	handler := NewHandler(&stubStore{
		listSessions: func(_ context.Context, userID, hash string, _ time.Time) ([]SessionInfo, error) {
			if userID != testUserID {
				t.Fatalf("user = %q", userID)
			}
			return []SessionInfo{
				{ID: "1", TokenPrefix: "dps_current", Current: hash == currentHash},
				{ID: "2", TokenPrefix: "dps_other", Current: false},
			}, nil
		},
		revokeSessionByID: func(_ context.Context, userID, sessionID string) error {
			if userID != testUserID {
				t.Fatalf("revoke user = %q", userID)
			}
			// Not ours (valid UUID, wrong owner) and unknown both 404.
			if sessionID == "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" || sessionID == "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" {
				return ErrNotFound
			}
			return nil
		},
		revokeOther: func(_ context.Context, userID, keepHash string) (int64, error) {
			if keepHash != currentHash {
				t.Fatalf("keep hash = %q", keepHash)
			}
			return 3, nil
		},
	})

	// List marks the caller's own token.
	request := authedRequest(t, http.MethodGet, "/v1/auth/sessions", "")
	request.Header.Set("Authorization", "Bearer dps_current")
	recorder := httptest.NewRecorder()
	handler.ListSessions(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d", recorder.Code)
	}
	var sessions []SessionInfo
	if err := json.Unmarshal(recorder.Body.Bytes(), &sessions); err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 || !sessions[0].Current {
		t.Fatalf("sessions = %+v", sessions)
	}

	// Foreign (another user's) ID → 404.
	request = authedRequest(t, http.MethodDelete, "/v1/auth/sessions/x", "")
	request.SetPathValue("id", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	recorder = httptest.NewRecorder()
	handler.RevokeSession(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("revoke foreign status = %d", recorder.Code)
	}

	// Revoke-others reports the count and keeps this session's hash.
	request = authedRequest(t, http.MethodPost, "/v1/auth/sessions/revoke-others", "")
	request.Header.Set("Authorization", "Bearer dps_current")
	recorder = httptest.NewRecorder()
	handler.RevokeOtherSessions(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"revoked":3`) {
		t.Fatalf("revoke-others: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
