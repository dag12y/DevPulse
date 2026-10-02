package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubFinder struct {
	workspaceID string
	role        string
	ok          bool
}

func (s stubFinder) FindKey(_ context.Context, _ string) (string, string, bool, error) {
	return s.workspaceID, s.role, s.ok, nil
}

func authedRequest(t *testing.T, role string) *http.Request {
	t.Helper()
	generated, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	finder := stubFinder{workspaceID: "ws_123", role: role, ok: true}
	_ = finder
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+generated.Raw)
	// Rewrite finder to accept the generated key hash.
	next := RequireAuth(keyFunc(func(_ context.Context, hash string) (string, string, bool, error) {
		if hash == Hash(generated.Raw) {
			return "ws_123", role, true, nil
		}
		return "", "", false, nil
	}), false, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workspaceID, ok := WorkspaceFromContext(r.Context())
		if !ok || workspaceID != "ws_123" {
			t.Errorf("workspace not injected")
		}
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d", recorder.Code)
	}
	return req
}

type keyFunc func(context.Context, string) (string, string, bool, error)

func (f keyFunc) FindKey(ctx context.Context, hash string) (string, string, bool, error) {
	return f(ctx, hash)
}

func TestGenerateProducesVerifiableKey(t *testing.T) {
	generated, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Raw) < 20 || len(generated.Hash) != 64 || generated.Prefix == "" {
		t.Fatalf("unexpected key: %#v", generated)
	}
	if !EqualHash(generated.Hash, generated.Raw) {
		t.Fatal("generated key must verify against its hash")
	}
	if EqualHash(generated.Hash, generated.Raw+"tampered") {
		t.Fatal("tampered key must not verify")
	}
}

func TestRequireAuthRejectsMissingOrInvalid(t *testing.T) {
	finder := stubFinder{ok: false}
	handler := RequireAuth(finder, false, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for name, setup := range map[string]func(*http.Request){
		"missing header": func(r *http.Request) {},
		"bad scheme": func(r *http.Request) {
			r.Header.Set("Authorization", "Token abc")
		},
		"unknown key": func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer dpk_unknown")
		},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			setup(req)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRequireAuthEnforcesViewerReadOnly(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	viewerFinder := keyFunc(func(context.Context, string) (string, string, bool, error) {
		return "ws_1", RoleViewer, true, nil
	})

	req := httptest.NewRecorder()
	readReq := httptest.NewRequest(http.MethodGet, "/", nil)
	readReq.Header.Set("Authorization", "Bearer dpk_x")
	RequireAuth(viewerFinder, false, next).ServeHTTP(req, readReq)
	if req.Code != http.StatusOK {
		t.Fatalf("viewer read status=%d", req.Code)
	}

	writeRecorder := httptest.NewRecorder()
	writeReq := httptest.NewRequest(http.MethodPost, "/", nil)
	writeReq.Header.Set("Authorization", "Bearer dpk_x")
	RequireAuth(viewerFinder, true, next).ServeHTTP(writeRecorder, writeReq)
	if writeRecorder.Code != http.StatusForbidden {
		t.Fatalf("viewer write status=%d", writeRecorder.Code)
	}
}

func TestAuthedRequestHelper(t *testing.T) {
	authedRequest(t, RoleAdmin)
}
