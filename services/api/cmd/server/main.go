package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/dag12y/devpulse/internal/analytics"
	"github.com/dag12y/devpulse/internal/config"
	"github.com/dag12y/devpulse/internal/database"
	internalhttp "github.com/dag12y/devpulse/internal/http"
	"github.com/dag12y/devpulse/internal/projects"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()

	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	projectHandler := projects.NewHandler(projects.NewRepository(db.Pool))
	analyticsHandler := analytics.NewHandler(analytics.NewService(analytics.NewRepository(db.Pool)))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/health/db", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Health(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"error","database":"unavailable"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","database":"connected"}`))
	})

	mux.HandleFunc("POST /v1/analytics/projects", projectHandler.Create)
	mux.HandleFunc("GET /v1/analytics/projects", projectHandler.List)
	mux.HandleFunc("GET /v1/analytics/projects/{id}", projectHandler.Get)
	mux.HandleFunc("PATCH /v1/analytics/projects/{id}", projectHandler.Update)
	mux.HandleFunc("DELETE /v1/analytics/projects/{id}", projectHandler.Delete)
	mux.HandleFunc("POST /v1/analytics/events", analyticsHandler.Ingest)
	mux.HandleFunc("GET /v1/analytics/summary", analyticsHandler.Summary)
	mux.HandleFunc("GET /v1/analytics/traffic", analyticsHandler.Traffic)
	mux.HandleFunc("GET /v1/analytics/pages", analyticsHandler.TopPages)

	addr := ":" + cfg.APIPort

	log.Printf("DevPulse API listening on %s", addr)

	if err := http.ListenAndServe(addr, internalhttp.CORS(mux)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
