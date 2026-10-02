package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type stubSessions struct {
	userID string
	ok     bool
}

func (s stubSessions) FindSession(_ context.Context, _ string, _ time.Time) (string, bool, error) {
	return s.userID, s.ok, nil
}

type stubMembers struct {
	role string
	ok   bool
}

func (s stubMembers) FindMembership(_ context.Context, _, _ string) (string, bool, error) {
	return s.role, s.ok, nil
}

func sessionRequest(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func TestRequireSessionAcceptsValidToken(t *testing.T) {
	next := RequireSession(stubSessions{userID: "u1", ok: true}, time.Now,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserFromContext(r.Context())
			if !ok || userID != "u1" {
				t.Errorf("user not injected")
			}
			w.WriteHeader(http.StatusOK)
		}))
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, sessionRequest("dps_valid"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestRequireSessionRejectsInvalid(t *testing.T) {
	next := RequireSession(stubSessions{ok: false}, time.Now,
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Error("must not reach handler")
		}))
	for name, req := range map[string]*http.Request{
		"missing": sessionRequest(""),
		"unknown": sessionRequest("dps_unknown"),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			next.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d", recorder.Code)
			}
		})
	}
}

func TestRequireAccessPrefersAPIKey(t *testing.T) {
	keys := stubFinder{workspaceID: "ws_key", role: RoleViewer, ok: true}
	// Sessions would fail if consulted: proves key path short-circuits.
	sessions := stubSessions{ok: false}
	next := RequireAccess(keys, sessions, stubMembers{}, time.Now, false,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			workspaceID, ok := WorkspaceFromContext(r.Context())
			if !ok || workspaceID != "ws_key" {
				t.Errorf("workspace not injected")
			}
			if _, hasUser := UserFromContext(r.Context()); hasUser {
				t.Errorf("key callers must not carry a user")
			}
			w.WriteHeader(http.StatusOK)
		}))
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, sessionRequest("dpk_anything"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRequireAccessResolvesSessionMembership(t *testing.T) {
	keys := stubFinder{ok: false}
	sessions := stubSessions{userID: "u9", ok: true}
	members := stubMembers{role: RoleAdmin, ok: true}
	next := RequireAccess(keys, sessions, members, time.Now, true,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			workspaceID, _ := WorkspaceFromContext(r.Context())
			userID, _ := UserFromContext(r.Context())
			if workspaceID != "ws_9" || userID != "u9" {
				t.Errorf("workspace=%q user=%q", workspaceID, userID)
			}
			w.WriteHeader(http.StatusOK)
		}))
	req := sessionRequest("dps_human")
	req.Header.Set("X-Workspace-ID", "ws_9")
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRequireAccessEnforcesSessionRoles(t *testing.T) {
	keys := stubFinder{ok: false}
	sessions := stubSessions{userID: "u9", ok: true}
	for name, testCase := range map[string]struct {
		role       string
		member     bool
		write      bool
		wantStatus int
	}{
		"viewer read":  {RoleViewer, true, false, http.StatusOK},
		"viewer write": {RoleViewer, true, true, http.StatusForbidden},
		"admin write":  {RoleAdmin, true, true, http.StatusOK},
		"non-member":   {RoleViewer, false, false, http.StatusForbidden},
	} {
		t.Run(name, func(t *testing.T) {
			next := RequireAccess(keys, sessions, stubMembers{role: testCase.role, ok: testCase.member}, time.Now,
				testCase.write, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			req := sessionRequest("dps_human")
			req.Header.Set("X-Workspace-ID", "ws_9")
			recorder := httptest.NewRecorder()
			next.ServeHTTP(recorder, req)
			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status=%d, want %d", recorder.Code, testCase.wantStatus)
			}
		})
	}
}

func TestRequireAccessRequiresWorkspaceSelection(t *testing.T) {
	next := RequireAccess(stubFinder{ok: false}, stubSessions{userID: "u9", ok: true}, stubMembers{}, time.Now,
		false, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, sessionRequest("dps_human"))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401 workspace selection required", recorder.Code)
	}
}
