package analytics

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
)

const maxRequestBodyBytes = 64 << 10

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (handler *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	var event Event
	if err := decodeEvent(w, r, &event); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := handler.service.Ingest(r.Context(), event); err != nil {
		switch {
		case errors.Is(err, ErrUnknownProject):
			writeError(w, http.StatusNotFound, "project not found")
		case errors.Is(err, ErrDisabledProject):
			writeError(w, http.StatusForbidden, "project is disabled")
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
