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
	"github.com/dag12y/devpulse/internal/email"
	internalhttp "github.com/dag12y/devpulse/internal/http"
	"github.com/dag12y/devpulse/internal/metrics"
	"github.com/dag12y/devpulse/internal/oauth"
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

	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

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

	// Auth email: Resend in production (Validate already refused to boot
	// without a key there), the log sender in development — verification
	// and reset links land in `docker logs` instead of a provider.
	var mailSender email.Sender = email.NewLog()
	if cfg.ResendAPIKey != "" {
		mailSender = email.NewResend(cfg.ResendAPIKey, cfg.EmailFrom)
	}

	// OAuth: each provider needs both halves of its credential pair;
	// anything less means the operator only half-configured it, so the
	// provider stays off (the list endpoint hides the button).
	oauthProviders := map[string]oauth.Provider{}
	if cfg.GitHubClientID != "" && cfg.GitHubClientSecret != "" {
		oauthProviders["github"] = &oauth.GitHub{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
		}
	} else {
		slog.Info("GITHUB_CLIENT_ID/GITHUB_CLIENT_SECRET unset: GitHub login disabled")
	}
	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		oauthProviders["google"] = &oauth.Google{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
		}
	} else {
		slog.Info("GOOGLE_CLIENT_ID/GOOGLE_CLIENT_SECRET unset: Google login disabled")
	}

	usersHandler := users.NewHandler(usersRepository,
		users.WithSecureCookies(cfg.SessionCookieSecure),
		users.WithEmailSender(mailSender),
		users.WithAppURL(cfg.AppURL),
		users.WithOAuthProviders(oauthProviders),
	)

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

	// pingDB reports database reachability with a tight timeout.
	// /ready gates load-balancer traffic; /health stays a pure
	// liveness check that never touches dependencies.
	pingDB := func(w http.ResponseWriter, r *http.Request) {
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
	}
	mux.HandleFunc("/ready", pingDB)
	mux.HandleFunc("/health/db", pingDB)

	// Operational counters in Prometheus text format. Served
	// unauthenticated for scrapers; values contain only aggregate
	// counts, never project IDs or visitor data.
	mux.HandleFunc("GET /metrics", metrics.Expose)

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
	mux.HandleFunc("POST /v1/auth/verify-email", usersHandler.VerifyEmail)
	mux.HandleFunc("POST /v1/auth/resend-verification", usersHandler.ResendVerification)
	mux.HandleFunc("POST /v1/auth/forgot-password", usersHandler.ForgotPassword)
	mux.HandleFunc("POST /v1/auth/reset-password", usersHandler.ResetPassword)
	mux.HandleFunc("GET /v1/auth/oauth/providers", usersHandler.OAuthProviderList)
	mux.HandleFunc("GET /v1/auth/oauth/{provider}/start", usersHandler.OAuthStart)
	mux.HandleFunc("GET /v1/auth/oauth/{provider}/callback", usersHandler.OAuthCallback)
	mux.HandleFunc("POST /v1/auth/2fa/setup", requireSession(usersHandler.TwoFactorSetup))
	mux.HandleFunc("POST /v1/auth/2fa/enable", requireSession(usersHandler.TwoFactorEnable))
	mux.HandleFunc("POST /v1/auth/2fa/disable", requireSession(usersHandler.TwoFactorDisable))
	mux.HandleFunc("GET /v1/auth/sessions", requireSession(usersHandler.ListSessions))
	mux.HandleFunc("DELETE /v1/auth/sessions/{id}", requireSession(usersHandler.RevokeSession))
	mux.HandleFunc("POST /v1/auth/sessions/revoke-others", requireSession(usersHandler.RevokeOtherSessions))
	mux.HandleFunc("GET /v1/auth/me", requireSession(usersHandler.Me))
	mux.HandleFunc("POST /v1/workspaces", requireSession(usersHandler.CreateWorkspace))
	mux.HandleFunc("GET /v1/workspaces", requireSession(usersHandler.ListWorkspaces))
	mux.HandleFunc("PATCH /v1/workspaces/{id}", requireSession(usersHandler.RenameWorkspace))
	mux.HandleFunc("DELETE /v1/workspaces/{id}", requireSession(usersHandler.DeleteWorkspace))
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
	} else if !tracker.HasBundle(cfg.TrackerDir) {
		slog.Warn("TRACKER_DIR has no analytics bundle: /analytics.js will 404; run ./scripts/build-tracker-dist.sh before building the API image")
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
			metrics.AddRetentionRun(true, 0, 0, 0, 0)
			slog.Error("retention cleanup failed", "error", err)
			return
		}
		metrics.AddRetentionRun(false, int64(result.PageViewsDeleted), int64(result.SessionsDeleted), int64(result.VisitorsDeleted), int64(result.AuthSessionsDeleted))
		slog.Info("retention cleanup",
			"projects", result.ProjectsProcessed,
			"page_views", result.PageViewsDeleted,
			"sessions", result.SessionsDeleted,
			"visitors", result.VisitorsDeleted,
			"auth_sessions", result.AuthSessionsDeleted)
	}
	run()
	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		run()
	}
}
