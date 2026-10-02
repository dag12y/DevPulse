package analytics

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/dag12y/devpulse/internal/auth"
)

const maxRequestBodyBytes = 64 << 10

type Handler struct {
	service *Service
	limiter *Limiter
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service, limiter: NewLimiter(maxEventsPerWindow, rateLimitWindow)}
}

func (handler *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	var event Event
	if err := decodeEvent(w, r, &event); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !handler.limiter.Allow(event.ProjectID, ClientIP(r)) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	if err := handler.service.Ingest(r.Context(), event, RequestMeta{
		UserAgent: r.UserAgent(),
		ClientIP:  ClientIP(r),
		Origin:    r.Header.Get("Origin"),
		Referer:   r.Header.Get("Referer"),
	}); err != nil {
		switch {
		case errors.Is(err, ErrUnknownProject):
			writeError(w, http.StatusNotFound, "project not found")
		case errors.Is(err, ErrDisabledProject):
			writeError(w, http.StatusForbidden, "project is disabled")
		case errors.Is(err, ErrOriginNotAllowed):
			writeError(w, http.StatusForbidden, "origin not allowed for this project")
		case errors.Is(err, ErrDuplicateEvent):
			writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
		default:
			var validationError *ValidationError
			if errors.As(err, &validationError) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			log.Printf("ingest analytics event: %v", err)
			writeError(w, http.StatusInternalServerError, "unable to accept event")
		}
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
}

func (handler *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	days, ok := queryInt(w, r, "days", DefaultDays)
	if !ok {
		return
	}
	summary, err := handler.service.Summary(r.Context(), workspaceID, r.URL.Query().Get("project_id"), days)
	if err != nil {
		writeQueryError(w, "load summary", err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (handler *Handler) Traffic(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	days, ok := queryInt(w, r, "days", DefaultDays)
	if !ok {
		return
	}
	points, err := handler.service.Traffic(r.Context(), workspaceID, r.URL.Query().Get("project_id"), days)
	if err != nil {
		writeQueryError(w, "load traffic", err)
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (handler *Handler) TopPages(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	limit, ok := queryInt(w, r, "limit", 20)
	if !ok {
		return
	}
	days, ok := queryInt(w, r, "days", DefaultDays)
	if !ok {
		return
	}
	pages, err := handler.service.TopPages(r.Context(), workspaceID, r.URL.Query().Get("project_id"), limit, days)
	if err != nil {
		writeQueryError(w, "load top pages", err)
		return
	}
	writeJSON(w, http.StatusOK, pages)
}

func (handler *Handler) Sources(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	days, ok := queryInt(w, r, "days", DefaultDays)
	if !ok {
		return
	}
	sources, err := handler.service.Sources(r.Context(), workspaceID, r.URL.Query().Get("project_id"), days)
	if err != nil {
		writeQueryError(w, "load sources", err)
		return
	}
	writeJSON(w, http.StatusOK, sources)
}

func (handler *Handler) Countries(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	days, ok := queryInt(w, r, "days", DefaultDays)
	if !ok {
		return
	}
	countries, err := handler.service.Countries(r.Context(), workspaceID, r.URL.Query().Get("project_id"), days)
	if err != nil {
		writeQueryError(w, "load countries", err)
		return
	}
	writeJSON(w, http.StatusOK, countries)
}

func (handler *Handler) Devices(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	days, ok := queryInt(w, r, "days", DefaultDays)
	if !ok {
		return
	}
	devices, err := handler.service.Devices(r.Context(), workspaceID, r.URL.Query().Get("project_id"), days)
	if err != nil {
		writeQueryError(w, "load devices", err)
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (handler *Handler) Realtime(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	realtime, err := handler.service.Realtime(r.Context(), workspaceID, r.URL.Query().Get("project_id"))
	if err != nil {
		writeQueryError(w, "load realtime", err)
		return
	}
	writeJSON(w, http.StatusOK, realtime)
}

func queryInt(w http.ResponseWriter, r *http.Request, name string, fallback int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, name+" must be an integer")
		return 0, false
	}
	return value, true
}

func writeQueryError(w http.ResponseWriter, action string, err error) {
	switch {
	case errors.Is(err, ErrUnknownProject):
		writeError(w, http.StatusNotFound, "project not found")
	default:
		var validationError *ValidationError
		if errors.As(err, &validationError) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("%s: %v", action, err)
		writeError(w, http.StatusInternalServerError, "unable to "+action)
	}
}

func decodeEvent(w http.ResponseWriter, r *http.Request, target *Event) error {
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
		log.Printf("write analytics JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
