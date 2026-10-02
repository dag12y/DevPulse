package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/dag12y/devpulse/internal/analytics"
	"github.com/dag12y/devpulse/internal/auth"
	"github.com/dag12y/devpulse/internal/config"
	"github.com/dag12y/devpulse/internal/database"
	internalhttp "github.com/dag12y/devpulse/internal/http"
	"github.com/dag12y/devpulse/internal/projects"
	"github.com/dag12y/devpulse/internal/retention"
	"github.com/dag12y/devpulse/internal/workspaces"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()

	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(ctx, db.Pool); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	// Retention runs in-process on a ticker. Single-replica safe; a
	// multi-replica deployment needs leader election before scaling.
	go runRetentionLoop(db.Pool, cfg.RetentionIntervalMinutes)

	mux := http.NewServeMux()
	workspaceRepository := workspaces.NewRepository(db.Pool)
	workspaceHandler := workspaces.NewHandler(workspaceRepository)
	projectHandler := projects.NewHandler(projects.NewRepository(db.Pool))

	var geo analytics.GeoResolver = analytics.NullGeoResolver{}
	if cfg.GeoIPDBPath != "" {
		maxMind, err := analytics.OpenMaxMind(cfg.GeoIPDBPath)
		if err != nil {
			log.Fatalf("GeoIP database failed: %v", err)
		}
		defer maxMind.Close()
		geo = maxMind
		log.Printf("GeoIP enrichment enabled (%s)", cfg.GeoIPDBPath)
	} else {
		log.Printf("GEOIP_DB_PATH unset: geography will be reported as Unknown")
	}
	analyticsHandler := analytics.NewHandler(analytics.NewServiceWithGeo(analytics.NewRepository(db.Pool), geo))

	readAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAuth(workspaceRepository, false, next).ServeHTTP
	}
	writeAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAuth(workspaceRepository, true, next).ServeHTTP
	}

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

	mux.HandleFunc("POST /v1/analytics/projects", writeAuth(projectHandler.Create))
	mux.HandleFunc("GET /v1/analytics/projects", readAuth(projectHandler.List))
	mux.HandleFunc("GET /v1/analytics/projects/{id}", readAuth(projectHandler.Get))
	mux.HandleFunc("PATCH /v1/analytics/projects/{id}", writeAuth(projectHandler.Update))
	mux.HandleFunc("DELETE /v1/analytics/projects/{id}", writeAuth(projectHandler.Delete))
	mux.HandleFunc("POST /v1/analytics/events", analyticsHandler.Ingest)
	mux.HandleFunc("GET /v1/analytics/summary", readAuth(analyticsHandler.Summary))
	mux.HandleFunc("GET /v1/analytics/traffic", readAuth(analyticsHandler.Traffic))
	mux.HandleFunc("GET /v1/analytics/pages", readAuth(analyticsHandler.TopPages))
	mux.HandleFunc("GET /v1/analytics/sources", readAuth(analyticsHandler.Sources))
	mux.HandleFunc("GET /v1/analytics/countries", readAuth(analyticsHandler.Countries))
	mux.HandleFunc("GET /v1/analytics/devices", readAuth(analyticsHandler.Devices))
	mux.HandleFunc("GET /v1/analytics/realtime", readAuth(analyticsHandler.Realtime))
	mux.HandleFunc("POST /v1/workspaces/bootstrap", workspaceHandler.Bootstrap)
	mux.HandleFunc("POST /v1/workspaces/keys", writeAuth(workspaceHandler.CreateKey))

	addr := ":" + cfg.APIPort

	log.Printf("DevPulse API listening on %s", addr)

	if err := http.ListenAndServe(addr, internalhttp.CORS(mux)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// runRetentionLoop purges expired analytics rows on a schedule. The first
// pass runs at startup so fresh deployments enforce retention promptly.
func runRetentionLoop(pool *pgxpool.Pool, intervalMinutes int) {
	run := func() {
		result, err := retention.RunOnce(context.Background(), pool, time.Now().UTC())
		if err != nil {
			log.Printf("retention cleanup failed: %v", err)
			return
		}
		log.Printf("retention cleanup: %d projects, %d page views, %d sessions, %d visitors deleted",
			result.ProjectsProcessed, result.PageViewsDeleted, result.SessionsDeleted, result.VisitorsDeleted)
	}
	run()
	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		run()
	}
}
