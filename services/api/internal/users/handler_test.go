package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/dag12y/devpulse/internal/email"
	"golang.org/x/crypto/bcrypt"
)

// testBcryptHash hashes at minimum cost for test speed. Production uses
// bcryptCost via the handler paths.
func testBcryptHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}

const (
	testUserID      = "c2eebc99-9c0b-4ef8-bb6d-6bb9bd380c33"
	testWorkspaceID = "d3eebc99-9c0b-4ef8-bb6d-6bb9bd380d44"
)

type stubStore struct {
	createUser        func(context.Context, string, string) (*User, error)
	findUserByEmail   func(context.Context, string) (*User, string, error)
	findUserByID      func(context.Context, string) (*User, error)
	createSession     func(context.Context, string, auth.GeneratedKey, time.Time) error
	revokeSession     func(context.Context, string) error
	createAuthToken   func(context.Context, string, string, string, time.Time) error
	consumeAuthToken  func(context.Context, string, string, time.Time) (string, error)
	setEmailVerified  func(context.Context, string) error
	updatePassword    func(context.Context, string, string) error
	revokeAllSessions func(context.Context, string) error
	createOAuthState  func(context.Context, string, string, string, string, time.Time, time.Time) error
	consumeOAuthState func(context.Context, string, time.Time) (string, string, string, error)
	findOAuthAccount  func(context.Context, string, string) (string, error)
	linkOAuthAccount  func(context.Context, string, string, string, string) error
	createWorkspace   func(context.Context, string, string) (Membership, error)
	listWorkspaces    func(context.Context, string) ([]Membership, error)
	findMembership    func(context.Context, string, string) (string, bool, error)
	listMembers       func(context.Context, string) ([]Member, error)
	addMember         func(context.Context, string, string, string) (*Member, error)
	updateMember      func(context.Context, string, string, string) (*Member, error)
	removeMember      func(context.Context, string, string) error
	renameWorkspace   func(context.Context, string, string, string) (string, error)
	deleteWorkspace   func(context.Context, string, string) error
}

func (s *stubStore) CreateUser(ctx context.Context, email, hash string) (*User, error) {
	return s.createUser(ctx, email, hash)
}
func (s *stubStore) FindUserByEmail(ctx context.Context, email string) (*User, string, error) {
	return s.findUserByEmail(ctx, email)
}
func (s *stubStore) FindUserByID(ctx context.Context, userID string) (*User, error) {
	if s.findUserByID == nil {
		return &User{ID: userID}, nil
	}
	return s.findUserByID(ctx, userID)
}
func (s *stubStore) CreateSession(ctx context.Context, userID string, token auth.GeneratedKey, expires time.Time) error {
	return s.createSession(ctx, userID, token, expires)
}
func (s *stubStore) RevokeSession(ctx context.Context, hash string) error {
	return s.revokeSession(ctx, hash)
}
func (s *stubStore) CreateAuthToken(ctx context.Context, userID, kind, hash string, expires time.Time) error {
	return s.createAuthToken(ctx, userID, kind, hash, expires)
}
func (s *stubStore) ConsumeAuthToken(ctx context.Context, kind, hash string, now time.Time) (string, error) {
	return s.consumeAuthToken(ctx, kind, hash, now)
}
func (s *stubStore) SetEmailVerified(ctx context.Context, userID string) error {
	return s.setEmailVerified(ctx, userID)
}
func (s *stubStore) UpdatePassword(ctx context.Context, userID, hash string) error {
	return s.updatePassword(ctx, userID, hash)
}
func (s *stubStore) RevokeAllSessions(ctx context.Context, userID string) error {
	return s.revokeAllSessions(ctx, userID)
}
func (s *stubStore) CreateOAuthState(ctx context.Context, provider, stateHash, codeVerifier, redirectPath string, now, expires time.Time) error {
	return s.createOAuthState(ctx, provider, stateHash, codeVerifier, redirectPath, now, expires)
}
func (s *stubStore) ConsumeOAuthState(ctx context.Context, stateHash string, now time.Time) (string, string, string, error) {
	return s.consumeOAuthState(ctx, stateHash, now)
}
func (s *stubStore) FindOAuthAccount(ctx context.Context, provider, providerUserID string) (string, error) {
	return s.findOAuthAccount(ctx, provider, providerUserID)
}
func (s *stubStore) LinkOAuthAccount(ctx context.Context, userID, provider, providerUserID, email string) error {
	return s.linkOAuthAccount(ctx, userID, provider, providerUserID, email)
}
func (s *stubStore) CreateWorkspace(ctx context.Context, userID, name string) (Membership, error) {
	return s.createWorkspace(ctx, userID, name)
}
func (s *stubStore) ListWorkspaces(ctx context.Context, userID string) ([]Membership, error) {
	return s.listWorkspaces(ctx, userID)
}
func (s *stubStore) FindMembership(ctx context.Context, userID, workspaceID string) (string, bool, error) {
	return s.findMembership(ctx, userID, workspaceID)
}
func (s *stubStore) ListMembers(ctx context.Context, workspaceID string) ([]Member, error) {
	return s.listMembers(ctx, workspaceID)
}
func (s *stubStore) AddMember(ctx context.Context, workspaceID, email, role string) (*Member, error) {
	return s.addMember(ctx, workspaceID, email, role)
}
func (s *stubStore) UpdateMemberRole(ctx context.Context, workspaceID, userID, role string) (*Member, error) {
	return s.updateMember(ctx, workspaceID, userID, role)
}
func (s *stubStore) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	return s.removeMember(ctx, workspaceID, userID)
}
func (s *stubStore) RenameWorkspace(ctx context.Context, userID, workspaceID, name string) (string, error) {
	return s.renameWorkspace(ctx, userID, workspaceID, name)
}
func (s *stubStore) DeleteWorkspace(ctx context.Context, userID, workspaceID string) error {
	return s.deleteWorkspace(ctx, userID, workspaceID)
}

// recordingSender captures dispatched auth mail. Sends happen on a
// background goroutine (dispatchEmail), so tests poll wait() instead of
// asserting synchronously. It never calls t.* itself: a late send may
// land after the test finished.
type recordingSender struct {
	mu       sync.Mutex
	messages []email.Message
}

func (sender *recordingSender) Send(_ context.Context, message email.Message) error {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	sender.messages = append(sender.messages, message)
	return nil
}

func (sender *recordingSender) wait(t *testing.T, to string) email.Message {
	t.Helper()
	messages := sender.await(t, to, 1)
	return messages[0]
}

// await waits until at least count messages have been delivered to the
// address and returns them in arrival order — so callers can observe a
// re-issued link replacing an earlier one.
func (sender *recordingSender) await(t *testing.T, to string, count int) []email.Message {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		sender.mu.Lock()
		var matched []email.Message
		for _, message := range sender.messages {
			if message.To == to {
				matched = append(matched, message)
			}
		}
		sender.mu.Unlock()
		if len(matched) >= count {
			return matched
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no email delivered to %s", to)
	return nil
}

// tokenFromMessage pulls the raw dpt_ token out of an auth email's
// link. Tokens are fixed length: "dpt_" plus 43 base64url characters.
func tokenFromMessage(t *testing.T, message email.Message) string {
	t.Helper()
	const prefix = "dpt_"
	tokenLength := len(prefix) + 43
	index := strings.Index(message.Text, "token=")
	if index < 0 {
		t.Fatalf("no token link in email body: %s", message.Text)
	}
	raw := message.Text[index+len("token="):]
	if len(raw) < tokenLength {
		t.Fatalf("truncated token in email body: %q", raw)
	}
	raw = raw[:tokenLength]
	if !strings.HasPrefix(raw, prefix) {
		t.Fatalf("token = %q, want %s prefix", raw, prefix)
	}
	return raw
}

func authedRequest(t *testing.T, method, target, body string) *http.Request {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	return req.WithContext(auth.WithUser(req.Context(), testUserID))
}

func TestRegisterCreatesUserWorkspaceAndIssuesVerification(t *testing.T) {
	var gotHash string
	var storedKind, storedHash string
	sender := &recordingSender{}
	handler := NewHandler(&stubStore{
		createUser: func(_ context.Context, email, hash string) (*User, error) {
			if email != "ada@example.com" {
				t.Fatalf("email = %q", email)
			}
			gotHash = hash
			return &User{ID: testUserID, Email: email}, nil
		},
		createWorkspace: func(_ context.Context, userID, name string) (Membership, error) {
			if userID != testUserID || name != "Acme" {
				t.Fatalf("workspace for %q named %q", userID, name)
			}
			return Membership{WorkspaceID: testWorkspaceID, WorkspaceName: name, Role: auth.RoleOwner}, nil
		},
		// No createSession stub: registration must not open a session.
		// Calling it would nil-panic the stub and fail the test.
		createAuthToken: func(_ context.Context, userID, kind, hash string, expires time.Time) error {
			if userID != testUserID || kind != TokenVerifyEmail {
				t.Fatalf("auth token for %q kind %q", userID, kind)
			}
			if !expires.After(time.Now()) {
				t.Fatalf("token expiry = %v, want future", expires)
			}
			storedKind, storedHash = kind, hash
			return nil
		},
	}, WithEmailSender(sender), WithAppURL("https://app.example.com"))
	// Bypass the hourly limiter pressure: fresh handler has empty buckets.
	recorder := httptest.NewRecorder()
	handler.Register(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/register",
		strings.NewReader(`{"email":"Ada@Example.COM","password":"correct-horse-12","workspace_name":"Acme"}`)))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.HasPrefix(gotHash, "$2a$") && !strings.HasPrefix(gotHash, "$2b$") {
		t.Fatalf("password was not bcrypt-hashed: %q", gotHash)
	}
	for _, want := range []string{`"workspace_id"`, `"owner"`, `"verification_required":true`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("missing %s in body=%s", want, recorder.Body.String())
		}
	}
	// No session yet: no token in the body, no cookie on the response.
	if strings.Contains(recorder.Body.String(), `"token"`) {
		t.Fatalf("registration must not issue a session token: %s", recorder.Body.String())
	}
	if cookies := recorder.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("registration must not set cookies, got %v", cookies)
	}
	if storedKind != TokenVerifyEmail || storedHash == "" {
		t.Fatalf("stored token kind=%q hash set=%v", storedKind, storedHash != "")
	}
	message := sender.wait(t, "ada@example.com")
	raw := tokenFromMessage(t, message)
	if !strings.Contains(message.Text, "https://app.example.com/verify-email?token="+raw) {
		t.Fatalf("email body missing verification link: %s", message.Text)
	}
	if auth.Hash(raw) != storedHash {
		t.Fatal("emailed token does not match the stored hash")
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	handler := NewHandler(&stubStore{
		createUser: func(context.Context, string, string) (*User, error) { return nil, ErrEmailTaken },
	})
	recorder := httptest.NewRecorder()
	handler.Register(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/register",
		strings.NewReader(`{"email":"ada@example.com","password":"correct-horse-12"}`)))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestRegisterRejectsWeakPassword(t *testing.T) {
	handler := NewHandler(&stubStore{
		createUser: func(context.Context, string, string) (*User, error) {
			t.Fatal("store must not be called")
			return nil, nil
		},
	})
	recorder := httptest.NewRecorder()
	handler.Register(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/register",
		strings.NewReader(`{"email":"ada@example.com","password":"short"}`)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestLoginIssuesSession(t *testing.T) {
	handler := NewHandler(&stubStore{
		findUserByEmail: func(_ context.Context, email string) (*User, string, error) {
			if email != "ada@example.com" {
				t.Fatalf("email = %q", email)
			}
			// bcrypt hash of "correct-horse-12" at cost 4 for test speed.
			verified := time.Now().UTC()
			return &User{ID: testUserID, Email: email, EmailVerifiedAt: &verified}, testBcryptHash(t, "correct-horse-12"), nil
		},
		createSession: func(context.Context, string, auth.GeneratedKey, time.Time) error { return nil },
		listWorkspaces: func(context.Context, string) ([]Membership, error) {
			return []Membership{{WorkspaceID: testWorkspaceID, WorkspaceName: "Acme", Role: auth.RoleOwner}}, nil
		},
	})
	recorder := httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"ada@example.com","password":"correct-horse-12"}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"token":"dps_`) {
		t.Fatalf("missing session token in body=%s", recorder.Body.String())
	}
}

func TestLoginRejectsUnverifiedEmail(t *testing.T) {
	handler := NewHandler(&stubStore{
		findUserByEmail: func(_ context.Context, email string) (*User, string, error) {
			// Password is correct; only verification is missing, so the
			// 403 must appear after the credential check.
			return &User{ID: testUserID, Email: email}, testBcryptHash(t, "correct-horse-12"), nil
		},
	})
	recorder := httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"ada@example.com","password":"correct-horse-12"}`)))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "email address not verified") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
	if len(recorder.Result().Cookies()) != 0 {
		t.Fatal("unverified login must not set a session cookie")
	}
}

func TestLoginRejectsWrongPasswordWithoutDistinction(t *testing.T) {
	handler := NewHandler(&stubStore{
		findUserByEmail: func(context.Context, string) (*User, string, error) {
			return &User{ID: testUserID}, testBcryptHash(t, "correct-horse-12"), nil
		},
	})
	wrongPassword := httptest.NewRecorder()
	handler.Login(wrongPassword, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"ada@example.com","password":"wrong-password-1"}`)))

	unknownUser := NewHandler(&stubStore{
		findUserByEmail: func(context.Context, string) (*User, string, error) { return nil, "", ErrNotFound },
	})
	unknown := httptest.NewRecorder()
	unknownUser.Login(unknown, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"nobody@example.com","password":"wrong-password-1"}`)))

	if wrongPassword.Code != http.StatusUnauthorized || unknown.Code != http.StatusUnauthorized {
		t.Fatalf("statuses = %d/%d", wrongPassword.Code, unknown.Code)
	}
	if wrongPassword.Body.String() != unknown.Body.String() {
		t.Fatalf("responses differ: %q vs %q", wrongPassword.Body.String(), unknown.Body.String())
	}
}

func TestLoginRateLimited(t *testing.T) {
	handler := NewHandler(&stubStore{
		findUserByEmail: func(context.Context, string) (*User, string, error) { return nil, "", ErrNotFound },
	})
	handler.loginLimiter = newIPLimiter(2, time.Hour)
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
			strings.NewReader(`{"email":"a@example.com","password":"correct-horse-12"}`)))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status=%d", i+1, recorder.Code)
		}
	}
	recorder := httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"a@example.com","password":"correct-horse-12"}`)))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d, want 429", recorder.Code)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	var revoked string
	handler := NewHandler(&stubStore{
		revokeSession: func(_ context.Context, hash string) error {
			revoked = hash
			return nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer dps_test_token_value")
	recorder := httptest.NewRecorder()
	handler.Logout(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d", recorder.Code)
	}
	if revoked != auth.Hash("dps_test_token_value") {
		t.Fatalf("revoked hash mismatch")
	}
}

func TestListMembersRequiresMembership(t *testing.T) {
	handler := NewHandler(&stubStore{
		findMembership: func(context.Context, string, string) (string, bool, error) {
			return "", false, nil
		},
		listMembers: func(context.Context, string) ([]Member, error) {
			t.Fatal("must not list for non-members")
			return nil, nil
		},
	})
	req := authedRequest(t, http.MethodGet, "/v1/workspaces/"+testWorkspaceID+"/members", "")
	req.SetPathValue("id", testWorkspaceID)
	recorder := httptest.NewRecorder()
	handler.ListMembers(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAddMemberRejectsViewer(t *testing.T) {
	handler := NewHandler(&stubStore{
		findMembership: func(context.Context, string, string) (string, bool, error) {
			return auth.RoleViewer, true, nil
		},
		addMember: func(context.Context, string, string, string) (*Member, error) {
			t.Fatal("viewer must not add members")
			return nil, nil
		},
	})
	req := authedRequest(t, http.MethodPost, "/v1/workspaces/"+testWorkspaceID+"/members",
		`{"email":"new@example.com","role":"viewer"}`)
	req.SetPathValue("id", testWorkspaceID)
	recorder := httptest.NewRecorder()
	handler.AddMember(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestRemoveMemberMapsLastOwner(t *testing.T) {
	handler := NewHandler(&stubStore{
		findMembership: func(context.Context, string, string) (string, bool, error) {
			return auth.RoleOwner, true, nil
		},
		removeMember: func(context.Context, string, string) error { return ErrLastOwner },
	})
	req := authedRequest(t, http.MethodDelete, "/", "")
	req.SetPathValue("id", testWorkspaceID)
	req.SetPathValue("userId", "e4eebc99-9c0b-4ef8-bb6d-6bb9bd380e55")
	recorder := httptest.NewRecorder()
	handler.RemoveMember(recorder, req)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status=%d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRenameWorkspaceRenamesForOwner(t *testing.T) {
	var gotName string
	handler := NewHandler(&stubStore{
		findMembership: func(context.Context, string, string) (string, bool, error) {
			return auth.RoleOwner, true, nil
		},
		renameWorkspace: func(_ context.Context, userID, workspaceID, name string) (string, error) {
			if userID != testUserID || workspaceID != testWorkspaceID {
				t.Fatalf("rename args = %q %q", userID, workspaceID)
			}
			gotName = name
			return name, nil
		},
	})
	req := authedRequest(t, http.MethodPatch, "/v1/workspaces/"+testWorkspaceID, `{"name":"  Renamed Co  "}`)
	req.SetPathValue("id", testWorkspaceID)
	recorder := httptest.NewRecorder()
	handler.RenameWorkspace(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d, body=%s", recorder.Code, recorder.Body.String())
	}
	if gotName != "Renamed Co" {
		t.Fatalf("renamed name = %q, want trimmed %q", gotName, "Renamed Co")
	}
	if !strings.Contains(recorder.Body.String(), `"workspace_name":"Renamed Co"`) {
		t.Fatalf("body=%s, want renamed name", recorder.Body.String())
	}
}

func TestRenameWorkspaceRejectsViewer(t *testing.T) {
	handler := NewHandler(&stubStore{
		findMembership: func(context.Context, string, string) (string, bool, error) {
			return auth.RoleViewer, true, nil
		},
		renameWorkspace: func(context.Context, string, string, string) (string, error) {
			t.Fatal("viewer must not rename")
			return "", nil
		},
	})
	req := authedRequest(t, http.MethodPatch, "/v1/workspaces/"+testWorkspaceID, `{"name":"New"}`)
	req.SetPathValue("id", testWorkspaceID)
	recorder := httptest.NewRecorder()
	handler.RenameWorkspace(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestRenameWorkspaceMapsNameTaken(t *testing.T) {
	handler := NewHandler(&stubStore{
		findMembership: func(context.Context, string, string) (string, bool, error) {
			return auth.RoleAdmin, true, nil
		},
		renameWorkspace: func(context.Context, string, string, string) (string, error) {
			return "", ErrNameTaken
		},
	})
	req := authedRequest(t, http.MethodPatch, "/v1/workspaces/"+testWorkspaceID, `{"name":"Acme"}`)
	req.SetPathValue("id", testWorkspaceID)
	recorder := httptest.NewRecorder()
	handler.RenameWorkspace(recorder, req)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status=%d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRenameWorkspaceRejectsBlankName(t *testing.T) {
	handler := NewHandler(&stubStore{
		findMembership: func(context.Context, string, string) (string, bool, error) {
			return auth.RoleOwner, true, nil
		},
		renameWorkspace: func(context.Context, string, string, string) (string, error) {
			t.Fatal("blank name must not reach store")
			return "", nil
		},
	})
	req := authedRequest(t, http.MethodPatch, "/v1/workspaces/"+testWorkspaceID, `{"name":"   "}`)
	req.SetPathValue("id", testWorkspaceID)
	recorder := httptest.NewRecorder()
	handler.RenameWorkspace(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestDeleteWorkspaceDeletesForOwner(t *testing.T) {
	var gotUser, gotWorkspace string
	handler := NewHandler(&stubStore{
		findMembership: func(context.Context, string, string) (string, bool, error) {
			return auth.RoleOwner, true, nil
		},
		deleteWorkspace: func(_ context.Context, userID, workspaceID string) error {
			gotUser, gotWorkspace = userID, workspaceID
			return nil
		},
	})
	req := authedRequest(t, http.MethodDelete, "/v1/workspaces/"+testWorkspaceID, "")
	req.SetPathValue("id", testWorkspaceID)
	recorder := httptest.NewRecorder()
	handler.DeleteWorkspace(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d, body=%s", recorder.Code, recorder.Body.String())
	}
	if gotUser != testUserID || gotWorkspace != testWorkspaceID {
		t.Fatalf("delete args = %q %q", gotUser, gotWorkspace)
	}
}

func TestDeleteWorkspaceRejectsAdmin(t *testing.T) {
	handler := NewHandler(&stubStore{
		findMembership: func(context.Context, string, string) (string, bool, error) {
			return auth.RoleAdmin, true, nil
		},
		deleteWorkspace: func(context.Context, string, string) error {
			t.Fatal("admin must not delete workspace")
			return nil
		},
	})
	req := authedRequest(t, http.MethodDelete, "/v1/workspaces/"+testWorkspaceID, "")
	req.SetPathValue("id", testWorkspaceID)
	recorder := httptest.NewRecorder()
	handler.DeleteWorkspace(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestDeleteWorkspaceRequiresMembership(t *testing.T) {
	handler := NewHandler(&stubStore{
		findMembership: func(context.Context, string, string) (string, bool, error) {
			return "", false, nil
		},
		deleteWorkspace: func(context.Context, string, string) error {
			t.Fatal("non-member must not delete")
			return nil
		},
	})
	req := authedRequest(t, http.MethodDelete, "/v1/workspaces/"+testWorkspaceID, "")
	req.SetPathValue("id", testWorkspaceID)
	recorder := httptest.NewRecorder()
	handler.DeleteWorkspace(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d, body=%s", recorder.Code, recorder.Body.String())
	}
}

// loginStub returns a store stubbed for a successful ada@example.com login.
func loginStub(t *testing.T) *stubStore {
	return &stubStore{
		findUserByEmail: func(_ context.Context, email string) (*User, string, error) {
			verified := time.Now().UTC()
			return &User{ID: testUserID, Email: email, EmailVerifiedAt: &verified}, testBcryptHash(t, "correct-horse-12"), nil
		},
		createSession: func(context.Context, string, auth.GeneratedKey, time.Time) error { return nil },
		listWorkspaces: func(context.Context, string) ([]Membership, error) {
			return []Membership{{WorkspaceID: testWorkspaceID, WorkspaceName: "Acme", Role: auth.RoleOwner}}, nil
		},
	}
}

// sessionCookie extracts the session cookie from a response, whatever its
// name (dp_session or the __Host- variant).
func sessionCookie(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == sessionCookieName || cookie.Name == sessionCookieNameSecure {
			return cookie
		}
	}
	t.Fatal("no session cookie was set")
	return nil
}

func TestLoginSetsSessionCookie(t *testing.T) {
	handler := NewHandler(loginStub(t))
	recorder := httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"ada@example.com","password":"correct-horse-12"}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cookie := sessionCookie(t, recorder)
	if cookie.Name != sessionCookieName {
		t.Fatalf("cookie name = %q, want %q", cookie.Name, sessionCookieName)
	}
	if !strings.HasPrefix(cookie.Value, "dps_") {
		t.Fatalf("cookie value = %q, want dps_ prefix", cookie.Value)
	}
	if !cookie.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if cookie.Path != "/" {
		t.Errorf("cookie path = %q, want /", cookie.Path)
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Secure {
		t.Error("insecure transport must not mark the cookie Secure")
	}
	if cookie.MaxAge <= 0 {
		t.Errorf("cookie MaxAge = %d, want positive lifetime", cookie.MaxAge)
	}
}

func TestSecureSessionCookieUsesHostPrefix(t *testing.T) {
	handler := NewHandler(loginStub(t), WithSecureCookies(true))
	recorder := httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"ada@example.com","password":"correct-horse-12"}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cookie := sessionCookie(t, recorder)
	if cookie.Name != sessionCookieNameSecure {
		t.Fatalf("cookie name = %q, want %q", cookie.Name, sessionCookieNameSecure)
	}
	if !cookie.Secure {
		t.Error("secure cookie must carry the Secure attribute")
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	handler := NewHandler(&stubStore{
		revokeSession: func(context.Context, string) error { return nil },
	}, WithSecureCookies(true))
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer dps_test_token_value")
	recorder := httptest.NewRecorder()
	handler.Logout(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d", recorder.Code)
	}
	cookie := sessionCookie(t, recorder)
	if cookie.MaxAge >= 0 {
		t.Errorf("clearing cookie MaxAge = %d, want negative", cookie.MaxAge)
	}
	if cookie.Value != "" {
		t.Errorf("clearing cookie value = %q, want empty", cookie.Value)
	}
}

// recordingTouchStore embeds the stub store and records TouchSession calls.
type recordingTouchStore struct {
	*stubStore
	touchedHash string
}

func (store *recordingTouchStore) TouchSession(_ context.Context, hash string, _ time.Time) error {
	store.touchedHash = hash
	return nil
}

func TestMeSlidesSessionExpiry(t *testing.T) {
	store := &recordingTouchStore{stubStore: &stubStore{
		findUserByID: func(_ context.Context, id string) (*User, error) {
			return &User{ID: id, Email: "ada@example.com"}, nil
		},
		listWorkspaces: func(context.Context, string) ([]Membership, error) { return nil, nil },
	}}
	handler := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer dps_slide_me")
	req = req.WithContext(auth.WithUser(req.Context(), testUserID))
	recorder := httptest.NewRecorder()
	handler.Me(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if store.touchedHash != auth.Hash("dps_slide_me") {
		t.Fatalf("touched hash = %q, want %q", store.touchedHash, auth.Hash("dps_slide_me"))
	}
}

func TestVerifyEmailConsumesTokenOnce(t *testing.T) {
	var verifiedFor string
	used := false
	const rawToken = "dpt_valid_email_token_0000000000000000000000000"
	handler := NewHandler(&stubStore{
		consumeAuthToken: func(_ context.Context, kind, hash string, _ time.Time) (string, error) {
			if kind != TokenVerifyEmail {
				t.Fatalf("kind = %q, want %q", kind, TokenVerifyEmail)
			}
			if used || hash != auth.Hash(rawToken) {
				return "", ErrNotFound
			}
			used = true
			return testUserID, nil
		},
		setEmailVerified: func(_ context.Context, userID string) error {
			verifiedFor = userID
			return nil
		},
	})
	verify := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.VerifyEmail(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/verify-email",
			strings.NewReader(`{"token":"`+rawToken+`"}`)))
		return recorder
	}
	first := verify()
	if first.Code != http.StatusOK || verifiedFor != testUserID {
		t.Fatalf("first verify: status=%d verifiedFor=%q body=%s", first.Code, verifiedFor, first.Body.String())
	}
	second := verify()
	if second.Code != http.StatusBadRequest {
		t.Fatalf("second verify: status=%d, want 400 (single use)", second.Code)
	}
	if !strings.Contains(second.Body.String(), "invalid or has expired") {
		t.Fatalf("second verify body = %s", second.Body.String())
	}
}

func TestResendVerificationIsIndistinguishable(t *testing.T) {
	sender := &recordingSender{}
	handler := NewHandler(&stubStore{
		findUserByEmail: func(_ context.Context, address string) (*User, string, error) {
			switch address {
			case "fresh@example.com":
				return &User{ID: testUserID, Email: address}, "", nil
			case "done@example.com":
				verified := time.Now().UTC()
				return &User{ID: testUserID, Email: address, EmailVerifiedAt: &verified}, "", nil
			default:
				return nil, "", ErrNotFound
			}
		},
		createAuthToken: func(_ context.Context, userID, kind, _ string, _ time.Time) error {
			if userID != testUserID || kind != TokenVerifyEmail {
				t.Fatalf("token for %q kind %q", userID, kind)
			}
			return nil
		},
	}, WithEmailSender(sender), WithAppURL("https://app.example.com"))
	send := func(address string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ResendVerification(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/resend-verification",
			strings.NewReader(`{"email":"`+address+`"}`)))
		return recorder
	}
	unknown := send("nobody@example.com")
	verified := send("done@example.com")
	pending := send("fresh@example.com")
	for name, recorder := range map[string]*httptest.ResponseRecorder{"unknown": unknown, "verified": verified, "pending": pending} {
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("%s: status=%d body=%s", name, recorder.Code, recorder.Body.String())
		}
	}
	if unknown.Body.String() != verified.Body.String() || verified.Body.String() != pending.Body.String() {
		t.Fatalf("bodies differ: %q / %q / %q", unknown.Body.String(), verified.Body.String(), pending.Body.String())
	}
	// Only the unverified account produced a link.
	sender.wait(t, "fresh@example.com")
	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.messages) != 1 {
		t.Fatalf("sent %d emails, want 1 (fresh only)", len(sender.messages))
	}
	if !strings.Contains(sender.messages[0].Text, "/verify-email?token=dpt_") {
		t.Fatalf("email missing verification link: %s", sender.messages[0].Text)
	}
}

func TestForgotPasswordIsIndistinguishable(t *testing.T) {
	sender := &recordingSender{}
	handler := NewHandler(&stubStore{
		findUserByEmail: func(_ context.Context, address string) (*User, string, error) {
			if address == "known@example.com" {
				return &User{ID: testUserID, Email: address}, "", nil
			}
			return nil, "", ErrNotFound
		},
		createAuthToken: func(_ context.Context, userID, kind, _ string, _ time.Time) error {
			if userID != testUserID || kind != TokenPasswordReset {
				t.Fatalf("token for %q kind %q", userID, kind)
			}
			return nil
		},
	}, WithEmailSender(sender), WithAppURL("https://app.example.com"))
	forgot := func(address string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ForgotPassword(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/forgot-password",
			strings.NewReader(`{"email":"`+address+`"}`)))
		return recorder
	}
	unknown := forgot("nobody@example.com")
	known := forgot("known@example.com")
	if unknown.Code != http.StatusAccepted || known.Code != http.StatusAccepted {
		t.Fatalf("statuses = %d/%d, want 202/202", unknown.Code, known.Code)
	}
	if unknown.Body.String() != known.Body.String() {
		t.Fatalf("bodies differ: %q vs %q", unknown.Body.String(), known.Body.String())
	}
	message := sender.wait(t, "known@example.com")
	if !strings.Contains(message.Text, "/reset-password?token=dpt_") {
		t.Fatalf("email missing reset link: %s", message.Text)
	}
	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.messages) != 1 {
		t.Fatalf("sent %d emails, want 1 (known only)", len(sender.messages))
	}
}

func TestResetPasswordRotatesPasswordAndRevokesSessions(t *testing.T) {
	const rawToken = "dpt_reset_token_000000000000000000000000000"
	consumed := false
	var gotHash, revokedFor string
	handler := NewHandler(&stubStore{
		consumeAuthToken: func(_ context.Context, kind, hash string, _ time.Time) (string, error) {
			if kind != TokenPasswordReset {
				t.Fatalf("kind = %q, want %q", kind, TokenPasswordReset)
			}
			if consumed || hash != auth.Hash(rawToken) {
				return "", ErrNotFound
			}
			consumed = true
			return testUserID, nil
		},
		updatePassword: func(_ context.Context, userID, hash string) error {
			if userID != testUserID {
				t.Fatalf("update password for %q", userID)
			}
			gotHash = hash
			return nil
		},
		revokeAllSessions: func(_ context.Context, userID string) error {
			revokedFor = userID
			return nil
		},
	})
	reset := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ResetPassword(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/reset-password",
			strings.NewReader(body)))
		return recorder
	}
	validBody := `{"token":"` + rawToken + `","password":"brand-new-password-42"}`

	// A weak password fails validation before the token is spent.
	weak := reset(`{"token":"` + rawToken + `","password":"short"}`)
	if weak.Code != http.StatusBadRequest {
		t.Fatalf("weak password: status=%d body=%s", weak.Code, weak.Body.String())
	}
	if consumed || gotHash != "" || revokedFor != "" {
		t.Fatal("rejected request must not consume the token or write anything")
	}

	good := reset(validBody)
	if good.Code != http.StatusOK || !strings.Contains(good.Body.String(), `"password_updated":true`) {
		t.Fatalf("reset: status=%d body=%s", good.Code, good.Body.String())
	}
	if !strings.HasPrefix(gotHash, "$2a$") && !strings.HasPrefix(gotHash, "$2b$") {
		t.Fatalf("new password was not bcrypt-hashed: %q", gotHash)
	}
	if revokedFor != testUserID {
		t.Fatalf("revoked sessions for %q, want %q", revokedFor, testUserID)
	}

	// Single use: the same link never works twice.
	again := reset(validBody)
	if again.Code != http.StatusBadRequest {
		t.Fatalf("reuse: status=%d, want 400", again.Code)
	}
}

func TestResendVerificationRateLimited(t *testing.T) {
	handler := NewHandler(&stubStore{
		findUserByEmail: func(context.Context, string) (*User, string, error) { return nil, "", ErrNotFound },
	})
	handler.emailLimiter = newIPLimiter(2, time.Hour)
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		handler.ResendVerification(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/resend-verification",
			strings.NewReader(`{"email":"a@example.com"}`)))
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("attempt %d: status=%d", i+1, recorder.Code)
		}
	}
	recorder := httptest.NewRecorder()
	handler.ResendVerification(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/resend-verification",
		strings.NewReader(`{"email":"a@example.com"}`)))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d, want 429", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Fatal("429 must carry Retry-After")
	}
}

func TestResetPasswordRateLimited(t *testing.T) {
	handler := NewHandler(&stubStore{
		consumeAuthToken: func(context.Context, string, string, time.Time) (string, error) {
			return "", ErrNotFound
		},
	})
	handler.resetLimiter = newIPLimiter(1, time.Hour)
	body := `{"token":"dpt_unknown","password":"correct-horse-12"}`
	first := httptest.NewRecorder()
	handler.ResetPassword(first, httptest.NewRequest(http.MethodPost, "/v1/auth/reset-password", strings.NewReader(body)))
	if first.Code != http.StatusBadRequest {
		t.Fatalf("first: status=%d, want 400 (invalid token)", first.Code)
	}
	second := httptest.NewRecorder()
	handler.ResetPassword(second, httptest.NewRequest(http.MethodPost, "/v1/auth/reset-password", strings.NewReader(body)))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second: status=%d, want 429", second.Code)
	}
}
