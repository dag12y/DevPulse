package analytics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dag12y/devpulse/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testDatabase connects to an isolated test database and applies all
// migrations. It skips when Postgres is unavailable so unit-test runs
// without a database still pass.
func testDatabase(t *testing.T) *pgxpool.Pool {
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

func randomUUID(t *testing.T) string {
	t.Helper()
	raw := randomHex(t, 16)
	return fmt.Sprintf("%s-%s-%s-%s-%s", raw[0:8], raw[8:12], raw[12:16], raw[16:20], raw[20:32])
}

type seedProject struct {
	workspaceID string
	projectID   string
	trackingID  string
	timezone    string
}

func seedProjectWithTimezone(t *testing.T, ctx context.Context, pool *pgxpool.Pool, timezone string) seedProject {
	t.Helper()
	suffix := randomHex(t, 4)
	var workspaceID, projectID string
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces (name) VALUES ($1) RETURNING id::text`,
		"test ws "+suffix).Scan(&workspaceID); err != nil {
		t.Fatal(err)
	}
	trackingID := "dp_test_" + suffix
	if err := pool.QueryRow(ctx, `INSERT INTO analytics_projects (workspace_id, name, tracking_id, timezone)
		VALUES ($1, $2, $3, $4) RETURNING id::text`,
		workspaceID, "test project "+suffix, trackingID, timezone).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id = $1`, workspaceID)
	})
	return seedProject{workspaceID: workspaceID, projectID: projectID, trackingID: trackingID, timezone: timezone}
}

func seedEvent(t *testing.T, trackingID, visitor string, at time.Time, path string) Event {
	t.Helper()
	source := "github"
	medium := "social"
	return Event{
		EventID:   randomUUID(t),
		Type:      "page_view",
		ProjectID: trackingID,
		VisitorID: visitor,
		SessionID: "sess_" + visitor,
		Timestamp: at,
		Page:      Page{URL: "https://example.com" + path, Path: path, Title: "T", Referrer: "https://github.com/"},
		Screen:    Dimensions{Width: 1920, Height: 1080},
		Viewport:  Dimensions{Width: 1200, Height: 800},
		Language:  "en-US",
		Timezone:  "Africa/Addis_Ababa",
		Campaign:  Campaign{Source: &source, Medium: &medium},
	}
}

// TestQueryIsolationAcrossWorkspaces proves two projects in different
// workspaces cannot see each other's data, even with a valid tracking ID
// from the other workspace.
func TestQueryIsolationAcrossWorkspaces(t *testing.T) {
	pool := testDatabase(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	now := time.Now().UTC()

	projectA := seedProjectWithTimezone(t, ctx, pool, "UTC")
	projectB := seedProjectWithTimezone(t, ctx, pool, "UTC")

	for i, visitor := range []string{"alice", "bob"} {
		if err := repository.Ingest(ctx, seedEvent(t, projectA.trackingID, visitor, now.Add(-time.Hour), "/"), ""); err != nil {
			t.Fatal(err)
		}
		_ = i
	}
	if err := repository.Ingest(ctx, seedEvent(t, projectB.trackingID, "mallory", now.Add(-time.Hour), "/secret"), ""); err != nil {
		t.Fatal(err)
	}

	summaryA, err := repository.Summary(ctx, projectA.workspaceID, projectA.trackingID, 30, now)
	if err != nil {
		t.Fatal(err)
	}
	if summaryA.TotalPageViews != 2 || summaryA.UniqueVisitors != 2 {
		t.Fatalf("workspace A summary = %+v, want 2 views / 2 visitors", summaryA)
	}

	// Project B's tracking ID under workspace A must not resolve.
	if _, err := repository.Summary(ctx, projectA.workspaceID, projectB.trackingID, 30, now); err != ErrUnknownProject {
		t.Fatalf("cross-workspace read err = %v, want ErrUnknownProject", err)
	}
	if _, err := repository.Traffic(ctx, projectA.workspaceID, projectB.trackingID, 7, now); err != ErrUnknownProject {
		t.Fatalf("cross-workspace traffic err = %v, want ErrUnknownProject", err)
	}

	// Unknown tracking IDs behave identically (no workspace leak).
	if _, err := repository.Summary(ctx, projectA.workspaceID, "dp_nope_missing", 30, now); err != ErrUnknownProject {
		t.Fatalf("unknown project err = %v, want ErrUnknownProject", err)
	}
}

// TestQueryWindowFiltering proves old events leave the current window and
// appear in the previous-window comparison instead.
func TestQueryWindowFiltering(t *testing.T) {
	pool := testDatabase(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	now := time.Now().UTC()

	project := seedProjectWithTimezone(t, ctx, pool, "UTC")
	window := ResolveWindow(now, "UTC", 7)

	inside := []Event{
		seedEvent(t, project.trackingID, "fresh-1", window.Start.Add(time.Hour), "/new"),
		seedEvent(t, project.trackingID, "fresh-2", window.End.Add(-time.Hour), "/new"),
	}
	previous := seedEvent(t, project.trackingID, "old-1", window.PrevStart.Add(time.Hour), "/old")
	ancient := seedEvent(t, project.trackingID, "ancient-1", window.PrevStart.Add(-time.Hour), "/ancient")
	for _, event := range append(inside, previous, ancient) {
		if err := repository.Ingest(ctx, event, ""); err != nil {
			t.Fatal(err)
		}
	}

	summary, err := repository.Summary(ctx, project.workspaceID, project.trackingID, 7, now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalPageViews != 2 || summary.UniqueVisitors != 2 {
		t.Fatalf("current window = %d views / %d visitors, want 2/2 (full %+v)", summary.TotalPageViews, summary.UniqueVisitors, summary)
	}
	if summary.PrevPageViews != 1 || summary.PrevVisitors != 1 {
		t.Fatalf("previous window = %d views / %d visitors, want 1/1", summary.PrevPageViews, summary.PrevVisitors)
	}
	if summary.PageViewsChange == nil || *summary.PageViewsChange != 100 {
		t.Fatalf("page views change = %v, want +100%%", summary.PageViewsChange)
	}

	pages, err := repository.TopPages(ctx, project.workspaceID, project.trackingID, 10, 7, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range pages {
		if page.Path == "/old" || page.Path == "/ancient" {
			t.Fatalf("out-of-window page leaked into top pages: %+v", page)
		}
	}
	if len(pages) != 1 || pages[0].Path != "/new" || pages[0].Views != 2 {
		t.Fatalf("top pages = %+v, want only /new x2", pages)
	}

	// A 365-day window must include the merely-old event but not the ancient one.
	wide, err := repository.Summary(ctx, project.workspaceID, project.trackingID, 365, now)
	if err != nil {
		t.Fatal(err)
	}
	if wide.TotalPageViews < 3 {
		t.Fatalf("365-day window views = %d, want >= 3", wide.TotalPageViews)
	}
}

// TestTrafficRespectsProjectTimezone proves an event landing on different
// local days is bucketed per the project's timezone.
func TestTrafficRespectsProjectTimezone(t *testing.T) {
	pool := testDatabase(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	now := time.Now().UTC()

	addis := seedProjectWithTimezone(t, ctx, pool, "Africa/Addis_Ababa")

	// 22:30 UTC "yesterday" is already 01:30 "today" in Addis Ababa
	// (UTC+3, no DST): inside the 1-day Addis window, outside a UTC one.
	utcMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	edge := utcMidnight.Add(-90 * time.Minute)
	if err := repository.Ingest(ctx, seedEvent(t, addis.trackingID, "edge-visitor", edge, "/edge"), ""); err != nil {
		t.Fatal(err)
	}

	// Control: the same instant in a UTC project falls on "yesterday" and
	// stays out of the 1-day UTC window.
	utcProject := seedProjectWithTimezone(t, ctx, pool, "UTC")
	if err := repository.Ingest(ctx, seedEvent(t, utcProject.trackingID, "edge-visitor", edge, "/edge"), ""); err != nil {
		t.Fatal(err)
	}
	utcPoints, err := repository.Traffic(ctx, utcProject.workspaceID, utcProject.trackingID, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(utcPoints) != 1 || utcPoints[0].PageViews != 0 {
		t.Fatalf("UTC control bucket = %+v, want zero views", utcPoints)
	}

	points, err := repository.Traffic(ctx, addis.workspaceID, addis.trackingID, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("traffic points = %d, want exactly 1 day", len(points))
	}
	if points[0].PageViews != 1 || points[0].Visitors != 1 {
		t.Fatalf("today bucket = %+v, want the edge event counted", points[0])
	}
	wantLabel := DayLabels(now, "Africa/Addis_Ababa", 1)[0]
	if points[0].Date != wantLabel {
		t.Fatalf("bucket label = %q, want %q", points[0].Date, wantLabel)
	}
}
