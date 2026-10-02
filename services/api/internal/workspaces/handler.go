package workspaces

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/dag12y/devpulse/internal/auth"
)

// Handler exposes workspace bootstrap and key management.
// Bootstrap is public but single-use: it succeeds only when no
// workspace exists yet, giving the first operator an owner key.
type Handler struct {
	repository *Repository
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{repository: repository}
}

type bootstrapInput struct {
	WorkspaceName string `json:"workspace_name"`
	KeyName       string `json:"key_name"`
}

func (h *Handler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	count, err := h.repository.Count(r.Context())
	if err != nil {
		log.Printf("bootstrap count workspaces: %v", err)
		writeError(w, http.StatusInternalServerError, "unable to bootstrap workspace")
		return
	}
	if count > 0 {
		writeError(w, http.StatusForbidden, "workspace already initialized")
		return
	}

	var input bootstrapInput
	if r.ContentLength > 0 {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input)
	}
	workspaceName := strings.TrimSpace(input.WorkspaceName)
	if workspaceName == "" {
		workspaceName = "Default workspace"
	}
	keyName := strings.TrimSpace(input.KeyName)
	if keyName == "" {
		keyName = "Initial owner key"
	}

	workspace, err := h.repository.CreateWorkspace(r.Context(), workspaceName)
	if err != nil {
		log.Printf("bootstrap create workspace: %v", err)
		writeError(w, http.StatusInternalServerError, "unable to bootstrap workspace")
		return
	}
	generated, err := auth.Generate()
	if err != nil {
		log.Printf("bootstrap generate key: %v", err)
		writeError(w, http.StatusInternalServerError, "unable to bootstrap workspace")
		return
	}
	key, err := h.repository.CreateKey(r.Context(), workspace.ID, keyName, auth.RoleOwner, generated)
	if err != nil {
		log.Printf("bootstrap create key: %v", err)
		writeError(w, http.StatusInternalServerError, "unable to bootstrap workspace")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"workspace": workspace,
		"api_key": map[string]any{
			"id":         key.ID,
			"name":       key.Name,
			"key_prefix": key.Prefix,
			"role":       key.Role,
			"api_key":    generated.Raw,
			"warning":    "Store this key now. It is shown exactly once.",
		},
	})
}

type createKeyInput struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

// CreateKey issues a new workspace-scoped key. Callers must already be
// authenticated with a write-capable role (enforced by middleware).
func (h *Handler) CreateKey(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var input createKeyInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	switch input.Role {
	case "", auth.RoleAdmin:
		input.Role = auth.RoleAdmin
	case auth.RoleOwner, auth.RoleViewer:
	default:
		writeError(w, http.StatusBadRequest, "role must be one of owner, admin, or viewer")
		return
	}

	generated, err := auth.Generate()
	if err != nil {
		log.Printf("generate API key: %v", err)
		writeError(w, http.StatusInternalServerError, "unable to create API key")
		return
	}
	key, err := h.repository.CreateKey(r.Context(), workspaceID, input.Name, input.Role, generated)
	if err != nil {
		log.Printf("create API key: %v", err)
		writeError(w, http.StatusInternalServerError, "unable to create API key")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         key.ID,
		"name":       key.Name,
		"key_prefix": key.Prefix,
		"role":       key.Role,
		"api_key":    generated.Raw,
		"warning":    "Store this key now. It is shown exactly once.",
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write workspaces JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
