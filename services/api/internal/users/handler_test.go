package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
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
	createUser      func(context.Context, string, string) (*User, error)
	findUserByEmail func(context.Context, string) (*User, string, error)
	findUserByID    func(context.Context, string) (*User, error)
	createSession   func(context.Context, string, auth.GeneratedKey, time.Time) error
	revokeSession   func(context.Context, string) error
	createWorkspace func(context.Context, string, string) (Membership, error)
	listWorkspaces  func(context.Context, string) ([]Membership, error)
	findMembership  func(context.Context, string, string) (string, bool, error)
	listMembers     func(context.Context, string) ([]Member, error)
	addMember       func(context.Context, string, string, string) (*Member, error)
	updateMember    func(context.Context, string, string, string) (*Member, error)
	removeMember    func(context.Context, string, string) error
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

func TestRegisterCreatesUserWorkspaceAndSession(t *testing.T) {
	var gotHash string
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
		createSession: func(context.Context, string, auth.GeneratedKey, time.Time) error { return nil },
	})
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
	for _, want := range []string{`"token":"dps_`, `"workspace_id"`, `"owner"`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("missing %s in body=%s", want, recorder.Body.String())
		}
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
			return &User{ID: testUserID, Email: email}, testBcryptHash(t, "correct-horse-12"), nil
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
