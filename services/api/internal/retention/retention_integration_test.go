package retention

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dag12y/devpulse/internal/analytics"
	"github.com/dag12y/devpulse/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://devpulse:devpulse_dev_password@localhost:5433/devpulse_test?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("postgres unavailable: %v", err)
	}
	if err := database.Migrate(context.Background(), pool); err != nil {
		pool.Close()
		t.Fatalf("migrate test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func randomHex(t *testing.T, n int) string {
	t.Helper()
	value := make([]byte, n)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}

func seedRetentionProject(t *testing.T, ctx context.Context, pool *pgxpool.Pool, retentionDays int) (workspaceID, projectID, trackingID string) {
	t.Helper()
	suffix := randomHex(t, 4)
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces (name) VALUES ($1) RETURNING id::text`,
		"retention ws "+suffix).Scan(&workspaceID); err != nil {
		t.Fatal(err)
	}
	trackingID = "dp_ret_" + suffix
	if err := pool.QueryRow(ctx, `INSERT INTO analytics_projects (workspace_id, name, tracking_id, retention_days)
		VALUES ($1, $2, $3, $4) RETURNING id::text`,
		workspaceID, "retention "+suffix, trackingID, retentionDays).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id = $1`, workspaceID)
	})
	return workspaceID, projectID, trackingID
}

func seedRetentionEvent(t *testing.T, trackingID, visitor string, at time.Time, path string) analytics.Event {
	t.Helper()
	raw := randomHex(t, 16)
	source := "github"
	return analytics.Event{
		EventID:   fmt.Sprintf("%s-%s-%s-%s-%s", raw[0:8], raw[8:12], raw[12:16], raw[16:20], raw[20:32]),
		Type:      "page_view",
		ProjectID: trackingID,
		VisitorID: visitor,
		SessionID: "sess_" + visitor,
		Timestamp: at,
		Page:      analytics.Page{URL: "https://example.com" + path, Path: path, Title: "T"},
		Screen:    analytics.Dimensions{Width: 1920, Height: 1080},
		Viewport:  analytics.Dimensions{Width: 1200, Height: 800},
		Language:  "en-US",
		Timezone:  "UTC",
		Campaign:  analytics.Campaign{Source: &source},
	}
}

func projectCounts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, projectID string) (views, sessions, visitors int64) {
	t.Helper()
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM analytics_page_views WHERE project_id = $1`, projectID).Scan(&views); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM analytics_sessions WHERE project_id = $1`, projectID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM analytics_visitors WHERE project_id = $1`, projectID).Scan(&visitors); err != nil {
		t.Fatal(err)
	}
	return views, sessions, visitors
}

// TestRetentionEnforcesPerProjectPolicies proves expired rows are purged
// per each project's retention_days while in-retention rows survive,
// orphans are collected, and reruns are no-ops.
func TestRetentionEnforcesPerProjectPolicies(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	ingest := analytics.NewRepository(pool)
	now := time.Now().UTC()

	_, shortID, shortTracking := seedRetentionProject(t, ctx, pool, 30)
	_, longID, longTracking := seedRetentionProject(t, ctx, pool, 365)

	// Short policy: one fresh event, one 31-day-old event (distinct visitors
	// so sessions/visitors isolate cleanly).
	if err := ingest.Ingest(ctx, seedRetentionEvent(t, shortTracking, "fresh", now.Add(-time.Hour), "/new"), ""); err != nil {
		t.Fatal(err)
	}
	if err := ingest.Ingest(ctx, seedRetentionEvent(t, shortTracking, "stale", now.Add(-31*24*time.Hour), "/old"), ""); err != nil {
		t.Fatal(err)
	}
	// Long policy: a 100-day-old event must survive a 365-day retention.
	if err := ingest.Ingest(ctx, seedRetentionEvent(t, longTracking, "mid", now.Add(-100*24*time.Hour), "/mid"), ""); err != nil {
		t.Fatal(err)
	}

	result, err := RunOnce(ctx, pool, now)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageViewsDeleted < 1 {
		t.Fatalf("page views deleted = %d, want >= 1", result.PageViewsDeleted)
	}

	if views, sessions, visitors := projectCounts(t, ctx, pool, shortID); views != 1 || sessions != 1 || visitors != 1 {
		t.Fatalf("30-day project = %d views / %d sessions / %d visitors, want 1/1/1", views, sessions, visitors)
	}
	if views, _, _ := projectCounts(t, ctx, pool, longID); views != 1 {
		t.Fatalf("365-day project views = %d, want 1 (100-day-old event kept)", views)
	}

	var finishedAt *time.Time
	var projects, runViews, runSessions, runVisitors int64
	var runErr *string
	if err := pool.QueryRow(ctx, `SELECT finished_at, projects_processed, page_views_deleted,
		sessions_deleted, visitors_deleted, error FROM retention_runs ORDER BY id DESC LIMIT 1`,
	).Scan(&finishedAt, &projects, &runViews, &runSessions, &runVisitors, &runErr); err != nil {
		t.Fatal(err)
	}
	if finishedAt == nil || projects < 2 || runViews < 1 || runSessions < 1 || runVisitors < 1 {
		t.Fatalf("run row = finished=%v projects=%d views=%d sessions=%d visitors=%d",
			finishedAt, projects, runViews, runSessions, runVisitors)
	}
	if runErr != nil {
		t.Fatalf("run error = %q", *runErr)
	}

	// Second pass must be a no-op for our projects.
	second, err := RunOnce(ctx, pool, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if second.PageViewsDeleted != 0 || second.SessionsDeleted != 0 || second.VisitorsDeleted != 0 {
		t.Fatalf("second run deleted rows: %+v, want all zero", second)
	}
	if views, sessions, visitors := projectCounts(t, ctx, pool, shortID); views != 1 || sessions != 1 || visitors != 1 {
		t.Fatalf("after rerun 30-day project = %d/%d/%d, want 1/1/1", views, sessions, visitors)
	}
}
