package analytics

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/dag12y/devpulse/internal/metrics"
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
		metrics.AddIngested("invalid")
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !handler.limiter.Allow(event.ProjectID, ClientIP(r)) {
		metrics.AddIngested("rate_limited")
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
			metrics.AddIngested("not_found")
			writeError(w, http.StatusNotFound, "project not found")
		case errors.Is(err, ErrDisabledProject):
			metrics.AddIngested("forbidden")
			writeError(w, http.StatusForbidden, "project is disabled")
		case errors.Is(err, ErrOriginNotAllowed):
			metrics.AddIngested("forbidden")
			writeError(w, http.StatusForbidden, "origin not allowed for this project")
		case errors.Is(err, ErrDuplicateEvent):
			metrics.AddIngested("accepted")
			writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
		default:
			var validationError *ValidationError
			if errors.As(err, &validationError) {
				metrics.AddIngested("invalid")
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			metrics.AddIngested("error")
			slog.Error("ingest analytics event", "error", err)
			writeError(w, http.StatusInternalServerError, "unable to accept event")
		}
		return
	}
	metrics.AddIngested("accepted")
	writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
}

func (handler *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	rg, ok := reportRange(w, r)
	if !ok {
		return
	}
	metrics.AddReport("summary")
	summary, err := handler.service.Summary(r.Context(), workspaceID, r.URL.Query().Get("project_id"), rg)
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
	rg, ok := reportRange(w, r)
	if !ok {
		return
	}
	metrics.AddReport("traffic")
	points, err := handler.service.Traffic(r.Context(), workspaceID, r.URL.Query().Get("project_id"), rg)
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
	rg, ok := reportRange(w, r)
	if !ok {
		return
	}
	metrics.AddReport("pages")
	pages, err := handler.service.TopPages(r.Context(), workspaceID, r.URL.Query().Get("project_id"), limit, rg)
	if err != nil {
		writeQueryError(w, "load top pages", err)
		return
	}
	writeJSON(w, http.StatusOK, pages)
}

func (handler *Handler) LandingPages(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	limit, ok := queryInt(w, r, "limit", 20)
	if !ok {
		return
	}
	rg, ok := reportRange(w, r)
	if !ok {
		return
	}
	metrics.AddReport("landing-pages")
	pages, err := handler.service.LandingPages(r.Context(), workspaceID, r.URL.Query().Get("project_id"), limit, rg)
	if err != nil {
		writeQueryError(w, "load landing pages", err)
		return
	}
	writeJSON(w, http.StatusOK, pages)
}

func (handler *Handler) UTM(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	rg, ok := reportRange(w, r)
	if !ok {
		return
	}
	metrics.AddReport("utm")
	report, err := handler.service.UTMReport(r.Context(), workspaceID, r.URL.Query().Get("project_id"), rg)
	if err != nil {
		writeQueryError(w, "load UTM report", err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (handler *Handler) Sources(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := auth.WorkspaceFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	rg, ok := reportRange(w, r)
	if !ok {
		return
	}
	metrics.AddReport("sources")
	sources, err := handler.service.Sources(r.Context(), workspaceID, r.URL.Query().Get("project_id"), rg)
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
	rg, ok := reportRange(w, r)
	if !ok {
		return
	}
	metrics.AddReport("countries")
	countries, err := handler.service.Countries(r.Context(), workspaceID, r.URL.Query().Get("project_id"), rg)
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
	rg, ok := reportRange(w, r)
	if !ok {
		return
	}
	metrics.AddReport("devices")
	devices, err := handler.service.Devices(r.Context(), workspaceID, r.URL.Query().Get("project_id"), rg)
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
	metrics.AddReport("realtime")
	realtime, err := handler.service.Realtime(r.Context(), workspaceID, r.URL.Query().Get("project_id"))
	if err != nil {
		writeQueryError(w, "load realtime", err)
		return
	}
	writeJSON(w, http.StatusOK, realtime)
}

// reportRange resolves the reporting window query parameters: either
// ?days=N (ending today in the project timezone) or an explicit
// ?start_date=&end_date= pair of project-local YYYY-MM-DD dates.
// The pair is all-or-nothing so a typo can never silently widen a window.
func reportRange(w http.ResponseWriter, r *http.Request) (ReportRange, bool) {
	startValue := r.URL.Query().Get("start_date")
	endValue := r.URL.Query().Get("end_date")
	if startValue != "" || endValue != "" {
		if startValue == "" || endValue == "" {
			writeError(w, http.StatusBadRequest, "start_date and end_date must both be provided")
			return ReportRange{}, false
		}
		start, err := ParseDate(startValue)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return ReportRange{}, false
		}
		end, err := ParseDate(endValue)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return ReportRange{}, false
		}
		if end.Before(start) {
			writeError(w, http.StatusBadRequest, "start_date must not be after end_date")
			return ReportRange{}, false
		}
		rg := ReportRange{Days: int(end.Sub(start).Hours()/24) + 1, EndDate: endValue}
		if err := rg.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return ReportRange{}, false
		}
		return rg, true
	}
	days, ok := queryInt(w, r, "days", DefaultDays)
	if !ok {
		return ReportRange{}, false
	}
	rg := ReportRange{Days: days}
	if err := rg.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return ReportRange{}, false
	}
	return rg, true
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
		slog.Error(action, "error", err)
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
		slog.Warn("write analytics JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
