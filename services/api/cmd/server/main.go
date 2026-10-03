package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/dag12y/devpulse/internal/analytics"
	"github.com/dag12y/devpulse/internal/auth"
	"github.com/dag12y/devpulse/internal/config"
	"github.com/dag12y/devpulse/internal/database"
	internalhttp "github.com/dag12y/devpulse/internal/http"
	"github.com/dag12y/devpulse/internal/projects"
	"github.com/dag12y/devpulse/internal/retention"
	"github.com/dag12y/devpulse/internal/tracker"
	"github.com/dag12y/devpulse/internal/users"
	"github.com/dag12y/devpulse/internal/workspaces"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()
	setupLogging(cfg.AppEnv)

	ctx := context.Background()

	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.Migrate(ctx, db.Pool); err != nil {
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	// Retention runs in-process on a ticker. Single-replica safe; a
	// multi-replica deployment needs leader election before scaling.
	go runRetentionLoop(db.Pool, cfg.RetentionIntervalMinutes)

	mux := http.NewServeMux()
	workspaceRepository := workspaces.NewRepository(db.Pool)
	workspaceHandler := workspaces.NewHandler(workspaceRepository)
	projectHandler := projects.NewHandler(projects.NewRepository(db.Pool))
	usersRepository := users.NewRepository(db.Pool)
	usersHandler := users.NewHandler(usersRepository)

	var geo analytics.GeoResolver = analytics.NullGeoResolver{}
	if cfg.GeoIPDBPath != "" {
		maxMind, err := analytics.OpenMaxMind(cfg.GeoIPDBPath)
		if err != nil {
			slog.Error("GeoIP database failed", "error", err)
			os.Exit(1)
		}
		defer maxMind.Close()
		geo = maxMind
		slog.Info("GeoIP enrichment enabled", "path", cfg.GeoIPDBPath)
	} else {
		slog.Info("GEOIP_DB_PATH unset: geography will be reported as Unknown")
	}
	analyticsHandler := analytics.NewHandler(analytics.NewServiceWithGeo(analytics.NewRepository(db.Pool), geo))

	readAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAccess(workspaceRepository, usersRepository, usersRepository, time.Now, false, next).ServeHTTP
	}
	writeAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAccess(workspaceRepository, usersRepository, usersRepository, time.Now, true, next).ServeHTTP
	}
	requireSession := func(next http.HandlerFunc) http.HandlerFunc {
		return auth.RequireSession(usersRepository, time.Now, next).ServeHTTP
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
	mux.HandleFunc("GET /v1/analytics/landing-pages", readAuth(analyticsHandler.LandingPages))
	mux.HandleFunc("GET /v1/analytics/utm", readAuth(analyticsHandler.UTM))
	mux.HandleFunc("GET /v1/analytics/sources", readAuth(analyticsHandler.Sources))
	mux.HandleFunc("GET /v1/analytics/countries", readAuth(analyticsHandler.Countries))
	mux.HandleFunc("GET /v1/analytics/devices", readAuth(analyticsHandler.Devices))
	mux.HandleFunc("GET /v1/analytics/realtime", readAuth(analyticsHandler.Realtime))
	mux.HandleFunc("POST /v1/workspaces/bootstrap", workspaceHandler.Bootstrap)
	mux.HandleFunc("POST /v1/workspaces/keys", writeAuth(workspaceHandler.CreateKey))

	mux.HandleFunc("POST /v1/auth/register", usersHandler.Register)
	mux.HandleFunc("POST /v1/auth/login", usersHandler.Login)
	mux.HandleFunc("POST /v1/auth/logout", usersHandler.Logout)
	mux.HandleFunc("GET /v1/auth/me", requireSession(usersHandler.Me))
	mux.HandleFunc("POST /v1/workspaces", requireSession(usersHandler.CreateWorkspace))
	mux.HandleFunc("GET /v1/workspaces", requireSession(usersHandler.ListWorkspaces))
	mux.HandleFunc("GET /v1/workspaces/{id}/members", requireSession(usersHandler.ListMembers))
	mux.HandleFunc("POST /v1/workspaces/{id}/members", requireSession(usersHandler.AddMember))
	mux.HandleFunc("PATCH /v1/workspaces/{id}/members/{userId}", requireSession(usersHandler.UpdateMember))
	mux.HandleFunc("DELETE /v1/workspaces/{id}/members/{userId}", requireSession(usersHandler.RemoveMember))

	addr := ":" + cfg.APIPort

	if len(cfg.AllowedOrigins) == 0 {
		slog.Warn("CORS open to all origins: set CORS_ALLOWED_ORIGINS in production")
	}

	// Middleware order: recovery outermost, then identity, observability,
	// hardening, and finally the timeout backstop around the mux.
	// The tracker bundle is public and served before API routing so a
	// stable HTTPS URL exists for the <script> tag (long immutable cache
	// for versioned files, short cache for /analytics.js).
	trackerHandler := tracker.New(cfg.TrackerDir)
	if cfg.TrackerDir == "" {
		slog.Info("TRACKER_DIR unset: /analytics.js will 404 until the tracker bundle is configured")
	}
	trackedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if trackerHandler.TryServe(w, r) {
			return
		}
		mux.ServeHTTP(w, r)
	})
	handler := internalhttp.Recover(
		internalhttp.RequestID(
			internalhttp.RequestLogger(
				internalhttp.SecurityHeaders(
					internalhttp.CORS(cfg.AllowedOrigins)(
						internalhttp.Timeout(trackedMux))))))

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	slog.Info("DevPulse API listening", "addr", addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// setupLogging emits JSON in production for log aggregation and human
// readable text elsewhere.
func setupLogging(appEnv string) {
	if appEnv == "production" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	}
}

// runRetentionLoop purges expired analytics rows on a schedule. The first
// pass runs at startup so fresh deployments enforce retention promptly.
func runRetentionLoop(pool *pgxpool.Pool, intervalMinutes int) {
	run := func() {
		result, err := retention.RunOnce(context.Background(), pool, time.Now().UTC())
		if err != nil {
			slog.Error("retention cleanup failed", "error", err)
			return
		}
		slog.Info("retention cleanup",
			"projects", result.ProjectsProcessed,
			"page_views", result.PageViewsDeleted,
			"sessions", result.SessionsDeleted,
			"visitors", result.VisitorsDeleted)
	}
	run()
	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		run()
	}
}
