package projects

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dag12y/devpulse/internal/auth"
)

const testProjectID = "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
const testWorkspaceID = "b1eebc99-9c0b-4ef8-bb6d-6bb9bd380b22"

type stubRepository struct {
	create func(context.Context, string, CreateInput, string) (*Project, error)
	list   func(context.Context, string) ([]Project, error)
	get    func(context.Context, string, string) (*Project, error)
	update func(context.Context, string, string, UpdateInput) (*Project, error)
	delete func(context.Context, string, string) error
}

func (stub *stubRepository) Create(ctx context.Context, workspaceID string, input CreateInput, trackingID string) (*Project, error) {
	return stub.create(ctx, workspaceID, input, trackingID)
}

func (stub *stubRepository) List(ctx context.Context, workspaceID string) ([]Project, error) {
	return stub.list(ctx, workspaceID)
}

func (stub *stubRepository) Get(ctx context.Context, workspaceID string, id string) (*Project, error) {
	return stub.get(ctx, workspaceID, id)
}

func (stub *stubRepository) Update(ctx context.Context, workspaceID string, id string, input UpdateInput) (*Project, error) {
	return stub.update(ctx, workspaceID, id, input)
}

func (stub *stubRepository) Delete(ctx context.Context, workspaceID string, id string) error {
	return stub.delete(ctx, workspaceID, id)
}

func withWorkspace(r *http.Request) *http.Request {
	return r.WithContext(auth.WithAuth(r.Context(), testWorkspaceID, auth.RoleAdmin))
}

func TestCreateProject(t *testing.T) {
	called := false
	handler := NewHandler(&stubRepository{
		create: func(_ context.Context, workspaceID string, input CreateInput, trackingID string) (*Project, error) {
			called = true
			if workspaceID != testWorkspaceID {
				t.Fatalf("workspace = %q", workspaceID)
			}
			if input.Name != "My Portfolio" {
				t.Fatalf("name = %q", input.Name)
			}
			if len(input.AllowedDomains) != 1 || input.AllowedDomains[0] != "example.com" {
				t.Fatalf("allowed domains = %#v", input.AllowedDomains)
			}
			if !strings.HasPrefix(trackingID, "dp_") {
				t.Fatalf("tracking ID = %q", trackingID)
			}
			return &Project{ID: testProjectID, WorkspaceID: workspaceID, Name: input.Name, TrackingID: trackingID}, nil
		},
	})

	request := withWorkspace(httptest.NewRequest(http.MethodPost, "/v1/analytics/projects", strings.NewReader(`{"name":"My Portfolio","allowed_domains":["example.com"],"timezone":"Africa/Addis_Ababa","retention_days":90}`)))
	recorder := httptest.NewRecorder()

	handler.Create(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !called {
		t.Fatal("repository Create was not called")
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("content type = %q", contentType)
	}
}

func TestCreateProjectRequiresAuth(t *testing.T) {
	handler := NewHandler(&stubRepository{
		create: func(context.Context, string, CreateInput, string) (*Project, error) {
			t.Fatal("repository must not be called without auth")
			return nil, nil
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/analytics/projects", strings.NewReader(`{"name":"My Portfolio"}`))
	recorder := httptest.NewRecorder()

	handler.Create(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateProjectRejectsInvalidInput(t *testing.T) {
	handler := NewHandler(&stubRepository{})
	request := withWorkspace(httptest.NewRequest(http.MethodPost, "/v1/analytics/projects", strings.NewReader(`{"name":"","retention_days":7}`)))
	recorder := httptest.NewRecorder()

	handler.Create(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestGetProjectRejectsMalformedUUID(t *testing.T) {
	handler := NewHandler(&stubRepository{})
	request := withWorkspace(httptest.NewRequest(http.MethodGet, "/v1/analytics/projects/not-a-uuid", nil))
	request.SetPathValue("id", "not-a-uuid")
	recorder := httptest.NewRecorder()

	handler.Get(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestGetProjectReturnsNotFound(t *testing.T) {
	handler := NewHandler(&stubRepository{
		get: func(context.Context, string, string) (*Project, error) { return nil, ErrNotFound },
	})
	request := withWorkspace(httptest.NewRequest(http.MethodGet, "/v1/analytics/projects/"+testProjectID, nil))
	request.SetPathValue("id", testProjectID)
	recorder := httptest.NewRecorder()

	handler.Get(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestGetProjectIsWorkspaceScoped(t *testing.T) {
	handler := NewHandler(&stubRepository{
		get: func(_ context.Context, workspaceID string, id string) (*Project, error) {
			if workspaceID != testWorkspaceID {
				t.Fatalf("workspace = %q", workspaceID)
			}
			// Simulate a project owned by another workspace: not found.
			if id == testProjectID && workspaceID != "other-workspace" {
				return &Project{ID: id, WorkspaceID: workspaceID}, nil
			}
			return nil, ErrNotFound
		},
	})
	request := withWorkspace(httptest.NewRequest(http.MethodGet, "/v1/analytics/projects/"+testProjectID, nil))
	request.SetPathValue("id", testProjectID)
	recorder := httptest.NewRecorder()

	handler.Get(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUpdateProject(t *testing.T) {
	handler := NewHandler(&stubRepository{
		update: func(_ context.Context, workspaceID string, id string, input UpdateInput) (*Project, error) {
			if workspaceID != testWorkspaceID {
				t.Fatalf("workspace = %q", workspaceID)
			}
			if id != testProjectID || input.Enabled == nil || *input.Enabled {
				t.Fatalf("unexpected update: id=%q enabled=%v", id, input.Enabled)
			}
			return &Project{ID: id, WorkspaceID: workspaceID, Enabled: false}, nil
		},
	})
	request := withWorkspace(httptest.NewRequest(http.MethodPatch, "/v1/analytics/projects/"+testProjectID, strings.NewReader(`{"enabled":false}`)))
	request.SetPathValue("id", testProjectID)
	recorder := httptest.NewRecorder()

	handler.Update(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestDeleteProject(t *testing.T) {
	called := false
	handler := NewHandler(&stubRepository{
		delete: func(_ context.Context, workspaceID string, id string) error {
			called = true
			if workspaceID != testWorkspaceID {
				t.Fatalf("workspace = %q", workspaceID)
			}
			if id != testProjectID {
				t.Fatalf("id = %q", id)
			}
			return nil
		},
	})
	request := withWorkspace(httptest.NewRequest(http.MethodDelete, "/v1/analytics/projects/"+testProjectID, nil))
	request.SetPathValue("id", testProjectID)
	recorder := httptest.NewRecorder()

	handler.Delete(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !called {
		t.Fatal("repository Delete was not called")
	}
}

func TestUpdateProjectNotFound(t *testing.T) {
	handler := NewHandler(&stubRepository{
		update: func(context.Context, string, string, UpdateInput) (*Project, error) { return nil, ErrNotFound },
	})
	request := withWorkspace(httptest.NewRequest(http.MethodPatch, "/v1/analytics/projects/"+testProjectID, strings.NewReader(`{"name":"Updated"}`)))
	request.SetPathValue("id", testProjectID)
	recorder := httptest.NewRecorder()

	handler.Update(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestRepositoryErrorsAreNotExposed(t *testing.T) {
	handler := NewHandler(&stubRepository{
		get: func(context.Context, string, string) (*Project, error) { return nil, errors.New("database connection refused") },
	})
	request := withWorkspace(httptest.NewRequest(http.MethodGet, "/v1/analytics/projects/"+testProjectID, nil))
	request.SetPathValue("id", testProjectID)
	recorder := httptest.NewRecorder()

	handler.Get(recorder, request)

	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "database connection refused") {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}
