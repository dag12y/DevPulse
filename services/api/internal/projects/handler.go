package projects

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxRequestBodyBytes = 1 << 20

type Handler struct {
	repository Repository
}

func NewHandler(repository Repository) *Handler {
	return &Handler{repository: repository}
}

func (handler *Handler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var input CreateInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	for range 5 {
		trackingID, err := NewTrackingID()
		if err != nil {
			slog.Error("generate project tracking ID", "error", err)
			writeError(w, http.StatusInternalServerError, "unable to create project")
			return
		}

		project, err := handler.repository.Create(r.Context(), workspaceID, input, trackingID)
		if errors.Is(err, ErrConflict) {
			continue
		}
		if err != nil {
			slog.Error("create project", "error", err)
			writeError(w, http.StatusInternalServerError, "unable to create project")
			return
		}

		writeJSON(w, http.StatusCreated, project)
		return
	}

	writeError(w, http.StatusInternalServerError, "unable to create project")
}

func (handler *Handler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	projects, err := handler.repository.List(r.Context(), workspaceID)
	if err != nil {
		slog.Error("list projects", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to list projects")
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (handler *Handler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := projectID(w, r)
	if !ok {
		return
	}

	project, err := handler.repository.Get(r.Context(), workspaceID, id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		slog.Error("get project", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to get project")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (handler *Handler) Update(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := projectID(w, r)
	if !ok {
		return
	}

	var input UpdateInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	project, err := handler.repository.Update(r.Context(), workspaceID, id, input)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		slog.Error("update project", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to update project")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (handler *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := projectID(w, r)
	if !ok {
		return
	}

	if err := handler.repository.Delete(r.Context(), workspaceID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		slog.Error("delete project", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to delete project")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func projectID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	var parsedID pgtype.UUID
	if err := parsedID.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "project id must be a UUID")
		return "", false
	}
	return id, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("request body must be valid JSON")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Warn("write JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
