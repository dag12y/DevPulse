package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// Result types for the analytics query endpoints. Field names match the
// JSON contract consumed by apps/dashboard/lib/api.ts.

type Summary struct {
	TotalPageViews     int64   `json:"total_page_views"`
	UniqueVisitors     int64   `json:"unique_visitors"`
	Sessions           int64   `json:"sessions"`
	BounceRate         float64 `json:"bounce_rate"`
	AvgSessionDuration float64 `json:"avg_session_duration"`
	Days               int     `json:"days"`

	// Previous-window totals cover the days immediately before the
	// current window. Change fields are nil when the previous window
	// is empty (no baseline to compare against).
	PrevPageViews    int64    `json:"prev_page_views"`
	PrevVisitors     int64    `json:"prev_visitors"`
	PrevSessions     int64    `json:"prev_sessions"`
	PageViewsChange  *float64 `json:"page_views_change"`
	VisitorsChange   *float64 `json:"visitors_change"`
	SessionsChange   *float64 `json:"sessions_change"`
	PrevBounceRate   float64  `json:"prev_bounce_rate"`
	BounceRateChange float64  `json:"bounce_rate_change"`

	// NewVisitors first appeared inside the current window;
	// ReturningVisitors were seen before it and came back.
	NewVisitors       int64 `json:"new_visitors"`
	ReturningVisitors int64 `json:"returning_visitors"`
}

type TrafficPoint struct {
	Date      string `json:"date"`
	PageViews int64  `json:"page_views"`
	Visitors  int64  `json:"visitors"`
}

type TopPage struct {
	Path           string `json:"path"`
	Views          int64  `json:"views"`
	UniqueVisitors int64  `json:"unique_visitors"`
}

type LandingPage struct {
	Path     string  `json:"path"`
	Sessions int64   `json:"sessions"`
	Visitors int64   `json:"visitors"`
	Share    float64 `json:"share"`
}

type UTMBreakdown struct {
	Name       string  `json:"name"`
	PageViews  int64   `json:"page_views"`
	Visitors   int64   `json:"visitors"`
	Percentage float64 `json:"percentage"`
}

type UTMReport struct {
	Sources   []UTMBreakdown `json:"sources"`
	Mediums   []UTMBreakdown `json:"mediums"`
	Campaigns []UTMBreakdown `json:"campaigns"`
}

type Source struct {
	Source     string  `json:"source"`
	Category   string  `json:"category"`
	PageViews  int64   `json:"page_views"`
	Visitors   int64   `json:"visitors"`
	Percentage float64 `json:"percentage"`
}

type Country struct {
	Country    string  `json:"country"`
	PageViews  int64   `json:"page_views"`
	Visitors   int64   `json:"visitors"`
	Percentage float64 `json:"percentage"`
}

type DeviceBreakdown struct {
	Name       string  `json:"name"`
	PageViews  int64   `json:"page_views"`
	Visitors   int64   `json:"visitors"`
	Percentage float64 `json:"percentage"`
}

type Devices struct {
	DeviceTypes      []DeviceBreakdown `json:"device_types"`
	Browsers         []DeviceBreakdown `json:"browsers"`
	OperatingSystems []DeviceBreakdown `json:"operating_systems"`
	Screens          []DeviceBreakdown `json:"screens"`
	Viewports        []DeviceBreakdown `json:"viewports"`
}

type RealtimePage struct {
	Path     string `json:"path"`
	Visitors int64  `json:"visitors"`
}

type Realtime struct {
	ActiveVisitors int64          `json:"active_visitors"`
	Pages          []RealtimePage `json:"pages"`
}

// realtimeWindow defines how recently a visitor must have been seen
// to count as online. Matches REALTIME_WINDOW_MINUTES in .env.example.
const realtimeWindow = "5 minutes"

// projectRef identifies the internal project row plus its display
// timezone for window math.
type projectRef struct {
	id       string
	timezone string
}

// resolveProject maps a public tracking ID to the internal project UUID
// within the authenticated workspace. Reads are allowed on disabled
// projects; unknown IDs — including IDs from other workspaces — report
// ErrUnknownProject so workspace membership is never leaked.
func (repository *PostgresRepository) resolveProject(ctx context.Context, workspaceID, trackingID string) (projectRef, error) {
	var ref projectRef
	var query string
	var args []any
	if workspaceID == "" {
		query = `SELECT id::text, timezone FROM analytics_projects WHERE tracking_id = $1`
		args = []any{trackingID}
	} else {
		query = `SELECT id::text, timezone FROM analytics_projects WHERE tracking_id = $1 AND workspace_id = $2`
		args = []any{trackingID, workspaceID}
	}
	err := repository.pool.QueryRow(ctx, query, args...).Scan(&ref.id, &ref.timezone)
	if err != nil {
		if err == pgx.ErrNoRows {
			return projectRef{}, ErrUnknownProject
		}
		return projectRef{}, fmt.Errorf("find project: %w", err)
	}
	return ref, nil
}

// resolveProjectID maps a public tracking ID to the internal project UUID.
// It exists for ingestion-adjacent lookups that do not need a window.
func (repository *PostgresRepository) resolveProjectID(ctx context.Context, workspaceID, trackingID string) (string, error) {
	ref, err := repository.resolveProject(ctx, workspaceID, trackingID)
	if err != nil {
		return "", err
	}
	return ref.id, nil
}

// windowFor returns the reporting window for a query: project-local days
// (ending on the range's last day, or today) when a project is selected,
// UTC otherwise.
func windowFor(ref projectRef, hasProject bool, now time.Time, rg ReportRange) Window {
	if !hasProject {
		return ResolveWindowRange(now, "UTC", rg)
	}
	return ResolveWindowRange(now, ref.timezone, rg)
}

func (repository *PostgresRepository) Summary(ctx context.Context, workspaceID, trackingID string, rg ReportRange, now time.Time) (Summary, error) {
	var summary Summary
	summary.Days = rg.Days

	var ref projectRef
	hasProject := trackingID != ""
	if hasProject {
		var err error
		ref, err = repository.resolveProject(ctx, workspaceID, trackingID)
		if err != nil {
			return summary, err
		}
	}
	window := windowFor(ref, hasProject, now, rg)

	current, err := repository.summaryWindow(ctx, ref.id, window.Start, window.End)
	if err != nil {
		return summary, err
	}
	previous, err := repository.summaryWindow(ctx, ref.id, window.PrevStart, window.PrevEnd)
	if err != nil {
		return summary, err
	}

	summary.TotalPageViews = current.pageViews
	summary.UniqueVisitors = current.visitors
	summary.Sessions = current.sessions
	summary.BounceRate = current.bounceRate
	summary.AvgSessionDuration = current.avgDuration
	summary.PrevPageViews = previous.pageViews
	summary.PrevVisitors = previous.visitors
	summary.PrevSessions = previous.sessions
	summary.PrevBounceRate = previous.bounceRate
	summary.PageViewsChange = ChangePct(float64(current.pageViews), float64(previous.pageViews))
	summary.VisitorsChange = ChangePct(float64(current.visitors), float64(previous.visitors))
	summary.SessionsChange = ChangePct(float64(current.sessions), float64(previous.sessions))
	summary.BounceRateChange = (current.bounceRate - previous.bounceRate) * 100

	newVisitors, err := repository.newVisitors(ctx, ref.id, window.Start, window.End)
	if err != nil {
		return summary, err
	}
	summary.NewVisitors = newVisitors
	summary.ReturningVisitors = current.visitors - newVisitors
	return summary, nil
}

type summaryCounts struct {
	pageViews   int64
	visitors    int64
	sessions    int64
	bounceRate  float64
	avgDuration float64
}

// summaryWindow aggregates one half-open window. Sessions are counted by
// started_at (sessions started in the window), so the metric is stable
// for a rolling window.
func (repository *PostgresRepository) summaryWindow(ctx context.Context, projectID string, start, end time.Time) (summaryCounts, error) {
	var counts summaryCounts

	pageViewsQuery := `SELECT COUNT(*), COUNT(DISTINCT visitor_id) FROM analytics_page_views
		WHERE is_bot = FALSE AND occurred_at >= $1 AND occurred_at < $2`
	sessionsQuery := `SELECT COUNT(*),
		COALESCE(AVG(CASE WHEN is_bounce THEN 1.0 ELSE 0.0 END), 0),
		COALESCE(AVG(EXTRACT(EPOCH FROM (last_seen_at - started_at))), 0)
		FROM analytics_sessions
		WHERE is_bot = FALSE AND started_at >= $1 AND started_at < $2`

	pageArgs := []any{start, end}
	sessionArgs := []any{start, end}
	if projectID != "" {
		pageViewsQuery += ` AND project_id = $3`
		sessionsQuery += ` AND project_id = $3`
		pageArgs = append(pageArgs, projectID)
		sessionArgs = append(sessionArgs, projectID)
	}

	if err := repository.pool.QueryRow(ctx, pageViewsQuery, pageArgs...).Scan(
		&counts.pageViews, &counts.visitors); err != nil {
		return counts, fmt.Errorf("query page view summary: %w", err)
	}
	if err := repository.pool.QueryRow(ctx, sessionsQuery, sessionArgs...).Scan(
		&counts.sessions, &counts.bounceRate, &counts.avgDuration); err != nil {
		return counts, fmt.Errorf("query session summary: %w", err)
	}
	return counts, nil
}

// newVisitors counts distinct visitors active in the window whose very
// first sighting falls inside it. Everyone else active is returning.
func (repository *PostgresRepository) newVisitors(ctx context.Context, projectID string, start, end time.Time) (int64, error) {
	var count int64
	query := `SELECT COUNT(DISTINCT pv.visitor_id)
		FROM analytics_page_views pv
		JOIN analytics_visitors v ON v.id = pv.visitor_id
		WHERE pv.is_bot = FALSE AND pv.occurred_at >= $1 AND pv.occurred_at < $2
		AND v.first_seen_at >= $1 AND v.first_seen_at < $2`
	args := []any{start, end}
	if projectID != "" {
		query += ` AND pv.project_id = $3`
		args = append(args, projectID)
	}
	if err := repository.pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("query new visitors: %w", err)
	}
	return count, nil
}

func (repository *PostgresRepository) Traffic(ctx context.Context, workspaceID, trackingID string, rg ReportRange, now time.Time) ([]TrafficPoint, error) {
	points := []TrafficPoint{}

	var ref projectRef
	hasProject := trackingID != ""
	if hasProject {
		var err error
		ref, err = repository.resolveProject(ctx, workspaceID, trackingID)
		if err != nil {
			return points, err
		}
	}
	timezone := SafeTimezone(ref.timezone)
	window := windowFor(ref, hasProject, now, rg)

	query := `SELECT ((occurred_at AT TIME ZONE $1)::date)::text AS day,
			COUNT(*), COUNT(DISTINCT visitor_id)
		FROM analytics_page_views
		WHERE is_bot = FALSE AND occurred_at >= $2 AND occurred_at < $3`
	args := []any{timezone, window.Start, window.End}
	if hasProject {
		query += ` AND project_id = $4`
		args = append(args, ref.id)
	}
	query += ` GROUP BY 1`

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return points, fmt.Errorf("query traffic: %w", err)
	}
	defer rows.Close()

	byDay := map[string]TrafficPoint{}
	for rows.Next() {
		var point TrafficPoint
		if err := rows.Scan(&point.Date, &point.PageViews, &point.Visitors); err != nil {
			return points, fmt.Errorf("scan traffic point: %w", err)
		}
		byDay[point.Date] = point
	}
	if err := rows.Err(); err != nil {
		return points, fmt.Errorf("iterate traffic points: %w", err)
	}
	// Fill days without events so charts share one axis.
	for _, label := range DayLabelsRange(now, timezone, rg) {
		if point, ok := byDay[label]; ok {
			points = append(points, point)
		} else {
			points = append(points, TrafficPoint{Date: label})
		}
	}
	return points, nil
}

func (repository *PostgresRepository) TopPages(ctx context.Context, workspaceID, trackingID string, limit int, rg ReportRange, now time.Time) ([]TopPage, error) {
	pages := []TopPage{}

	var ref projectRef
	hasProject := trackingID != ""
	if hasProject {
		var err error
		ref, err = repository.resolveProject(ctx, workspaceID, trackingID)
		if err != nil {
			return pages, err
		}
	}
	window := windowFor(ref, hasProject, now, rg)

	query := `SELECT path, COUNT(*) AS views, COUNT(DISTINCT visitor_id) AS unique_visitors
		FROM analytics_page_views
		WHERE is_bot = FALSE AND occurred_at >= $1 AND occurred_at < $2`
	args := []any{window.Start, window.End}
	if hasProject {
		query += ` AND project_id = $3`
		args = append(args, ref.id)
	}
	if hasProject {
		query += ` GROUP BY path ORDER BY views DESC LIMIT $4`
	} else {
		query += ` GROUP BY path ORDER BY views DESC LIMIT $3`
	}
	args = append(args, limit)

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return pages, fmt.Errorf("query top pages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var page TopPage
		if err := rows.Scan(&page.Path, &page.Views, &page.UniqueVisitors); err != nil {
			return pages, fmt.Errorf("scan top page: %w", err)
		}
		pages = append(pages, page)
	}
	if err := rows.Err(); err != nil {
		return pages, fmt.Errorf("iterate top pages: %w", err)
	}
	return pages, nil
}

// LandingPages aggregates non-bot sessions by the first page of the
// session. Share is the fraction of sessions starting there.
func (repository *PostgresRepository) LandingPages(ctx context.Context, workspaceID, trackingID string, limit int, rg ReportRange, now time.Time) ([]LandingPage, error) {
	pages := []LandingPage{}

	var ref projectRef
	hasProject := trackingID != ""
	if hasProject {
		var err error
		ref, err = repository.resolveProject(ctx, workspaceID, trackingID)
		if err != nil {
			return pages, err
		}
	}
	window := windowFor(ref, hasProject, now, rg)

	query := `SELECT COALESCE(NULLIF(landing_page, ''), '(unknown)'), COUNT(*), COUNT(DISTINCT visitor_id)
		FROM analytics_sessions
		WHERE is_bot = FALSE AND started_at >= $1 AND started_at < $2`
	args := []any{window.Start, window.End}
	if hasProject {
		query += ` AND project_id = $3`
		args = append(args, ref.id)
	}
	if hasProject {
		query += ` GROUP BY 1 ORDER BY COUNT(*) DESC LIMIT $4`
	} else {
		query += ` GROUP BY 1 ORDER BY COUNT(*) DESC LIMIT $3`
	}
	args = append(args, limit)

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return pages, fmt.Errorf("query landing pages: %w", err)
	}
	defer rows.Close()

	var total int64
	type row struct {
		path     string
		sessions int64
		visitors int64
	}
	var collected []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.path, &r.sessions, &r.visitors); err != nil {
			return pages, fmt.Errorf("scan landing page: %w", err)
		}
		collected = append(collected, r)
		total += r.sessions
	}
	if err := rows.Err(); err != nil {
		return pages, fmt.Errorf("iterate landing pages: %w", err)
	}
	for _, r := range collected {
		entry := LandingPage{Path: r.path, Sessions: r.sessions, Visitors: r.visitors}
		if total > 0 {
			entry.Share = float64(r.sessions*10000/total) / 100
		}
		pages = append(pages, entry)
	}
	return pages, nil
}

// UTMReport breaks non-bot page views down by campaign source, medium
// and campaign name. Rows without a value collapse to "(not set)".
func (repository *PostgresRepository) UTMReport(ctx context.Context, workspaceID, trackingID string, rg ReportRange, now time.Time) (UTMReport, error) {
	var report UTMReport

	var ref projectRef
	hasProject := trackingID != ""
	if hasProject {
		var err error
		ref, err = repository.resolveProject(ctx, workspaceID, trackingID)
		if err != nil {
			return report, err
		}
	}
	window := windowFor(ref, hasProject, now, rg)

	var err error
	if report.Sources, err = repository.utmBreakdown(ctx, "utm_source", ref.id, window.Start, window.End); err != nil {
		return report, err
	}
	if report.Mediums, err = repository.utmBreakdown(ctx, "utm_medium", ref.id, window.Start, window.End); err != nil {
		return report, err
	}
	if report.Campaigns, err = repository.utmBreakdown(ctx, "utm_campaign", ref.id, window.Start, window.End); err != nil {
		return report, err
	}
	return report, nil
}

// utmBreakdown aggregates a single whitelisted UTM column, capped at 50
// rows so high-cardinality custom values cannot bloat the response.
func (repository *PostgresRepository) utmBreakdown(ctx context.Context, column, projectID string, start, end time.Time) ([]UTMBreakdown, error) {
	entries := []UTMBreakdown{}

	var col string
	switch column {
	case "utm_source":
		col = `COALESCE(NULLIF(utm_source, ''), '(not set)')`
	case "utm_medium":
		col = `COALESCE(NULLIF(utm_medium, ''), '(not set)')`
	case "utm_campaign":
		col = `COALESCE(NULLIF(utm_campaign, ''), '(not set)')`
	default:
		return entries, fmt.Errorf("unknown UTM dimension: %s", column)
	}

	query := `SELECT ` + col + `, COUNT(*), COUNT(DISTINCT visitor_id)
		FROM analytics_page_views
		WHERE is_bot = FALSE AND occurred_at >= $1 AND occurred_at < $2`
	args := []any{start, end}
	if projectID != "" {
		query += ` AND project_id = $3`
		args = append(args, projectID)
	}
	query += ` GROUP BY 1 ORDER BY COUNT(*) DESC LIMIT 50`

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return entries, fmt.Errorf("query %s: %w", column, err)
	}
	defer rows.Close()

	var total int64
	type row struct {
		name     string
		views    int64
		visitors int64
	}
	var collected []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.name, &r.views, &r.visitors); err != nil {
			return entries, fmt.Errorf("scan %s: %w", column, err)
		}
		collected = append(collected, r)
		total += r.views
	}
	if err := rows.Err(); err != nil {
		return entries, fmt.Errorf("iterate %s: %w", column, err)
	}
	for _, r := range collected {
		entry := UTMBreakdown{Name: r.name, PageViews: r.views, Visitors: r.visitors}
		if total > 0 {
			entry.Percentage = float64(r.views*10000/total) / 100
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Sources aggregates non-bot page views by traffic source. Raw
// referrer/UTM groups come from PostgreSQL and are classified in Go
// (ClassifySource), so known hosts collapse to friendly names.
func (repository *PostgresRepository) Sources(ctx context.Context, workspaceID, trackingID string, rg ReportRange, now time.Time) ([]Source, error) {
	sources := []Source{}

	var ref projectRef
	hasProject := trackingID != ""
	if hasProject {
		var err error
		ref, err = repository.resolveProject(ctx, workspaceID, trackingID)
		if err != nil {
			return sources, err
		}
	}
	window := windowFor(ref, hasProject, now, rg)

	query := `SELECT COALESCE(referrer, ''), COALESCE(utm_source, ''), COUNT(*), COUNT(DISTINCT visitor_id)
		FROM analytics_page_views
		WHERE is_bot = FALSE AND occurred_at >= $1 AND occurred_at < $2`
	args := []any{window.Start, window.End}
	if hasProject {
		query += ` AND project_id = $3`
		args = append(args, ref.id)
	}
	query += ` GROUP BY referrer, utm_source ORDER BY COUNT(*) DESC LIMIT 1000`

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return sources, fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	merged := map[string]*Source{}
	order := []string{}
	var total int64
	for rows.Next() {
		var referrer, utmSource string
		var views, visitors int64
		if err := rows.Scan(&referrer, &utmSource, &views, &visitors); err != nil {
			return sources, fmt.Errorf("scan source: %w", err)
		}
		name, category := ClassifySource(referrer, utmSource)
		entry, ok := merged[name]
		if !ok {
			entry = &Source{Source: name, Category: category}
			merged[name] = entry
			order = append(order, name)
		}
		entry.PageViews += views
		entry.Visitors += visitors
		total += views
	}
	if err := rows.Err(); err != nil {
		return sources, fmt.Errorf("iterate sources: %w", err)
	}
	for _, name := range order {
		entry := merged[name]
		if total > 0 {
			entry.Percentage = float64(entry.PageViews*10000/total) / 100
		}
		sources = append(sources, *entry)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].PageViews > sources[j].PageViews })
	return sources, nil
}

// Countries aggregates non-bot page views by country code.
// Rows without geography (NullGeoResolver era) collapse to "Unknown".
func (repository *PostgresRepository) Countries(ctx context.Context, workspaceID, trackingID string, rg ReportRange, now time.Time) ([]Country, error) {
	countries := []Country{}

	var ref projectRef
	hasProject := trackingID != ""
	if hasProject {
		var err error
		ref, err = repository.resolveProject(ctx, workspaceID, trackingID)
		if err != nil {
			return countries, err
		}
	}
	window := windowFor(ref, hasProject, now, rg)

	query := `SELECT COALESCE(NULLIF(country, ''), 'Unknown'), COUNT(*), COUNT(DISTINCT visitor_id)
		FROM analytics_page_views
		WHERE is_bot = FALSE AND occurred_at >= $1 AND occurred_at < $2`
	args := []any{window.Start, window.End}
	if hasProject {
		query += ` AND project_id = $3`
		args = append(args, ref.id)
	}
	query += ` GROUP BY 1 ORDER BY COUNT(*) DESC LIMIT 100`

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return countries, fmt.Errorf("query countries: %w", err)
	}
	defer rows.Close()

	var total int64
	type row struct {
		name     string
		views    int64
		visitors int64
	}
	var rows_ []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.name, &r.views, &r.visitors); err != nil {
			return countries, fmt.Errorf("scan country: %w", err)
		}
		rows_ = append(rows_, r)
		total += r.views
	}
	if err := rows.Err(); err != nil {
		return countries, fmt.Errorf("iterate countries: %w", err)
	}
	for _, r := range rows_ {
		entry := Country{Country: r.name, PageViews: r.views, Visitors: r.visitors}
		if total > 0 {
			entry.Percentage = float64(r.views*10000/total) / 100
		}
		countries = append(countries, entry)
	}
	return countries, nil
}

// Devices aggregates non-bot page views by device type, browser and OS.
func (repository *PostgresRepository) Devices(ctx context.Context, workspaceID, trackingID string, rg ReportRange, now time.Time) (Devices, error) {
	var devices Devices

	var ref projectRef
	hasProject := trackingID != ""
	if hasProject {
		var err error
		ref, err = repository.resolveProject(ctx, workspaceID, trackingID)
		if err != nil {
			return devices, err
		}
	}
	window := windowFor(ref, hasProject, now, rg)

	var err error
	if devices.DeviceTypes, err = repository.breakdown(ctx, "device_type", ref.id, window.Start, window.End, 100); err != nil {
		return devices, err
	}
	if devices.Browsers, err = repository.breakdown(ctx, "browser", ref.id, window.Start, window.End, 100); err != nil {
		return devices, err
	}
	if devices.OperatingSystems, err = repository.breakdown(ctx, "os", ref.id, window.Start, window.End, 100); err != nil {
		return devices, err
	}
	if devices.Screens, err = repository.breakdown(ctx, "screen", ref.id, window.Start, window.End, 25); err != nil {
		return devices, err
	}
	if devices.Viewports, err = repository.breakdown(ctx, "viewport", ref.id, window.Start, window.End, 25); err != nil {
		return devices, err
	}
	return devices, nil
}

// breakdown aggregates a single whitelisted dimension column. A positive
// limit caps high-cardinality dimensions (screen/viewport pairs); zero
// leaves the grouping uncapped.
func (repository *PostgresRepository) breakdown(ctx context.Context, column, projectID string, start, end time.Time, limit int) ([]DeviceBreakdown, error) {
	entries := []DeviceBreakdown{}

	var col string
	switch column {
	case "device_type":
		col = `COALESCE(NULLIF(device_type, ''), 'unknown')`
	case "browser":
		col = `COALESCE(NULLIF(browser, ''), 'Other')`
	case "os":
		col = `COALESCE(NULLIF(os, ''), 'Other')`
	case "screen":
		col = `COALESCE(screen_width, 0) || '×' || COALESCE(screen_height, 0)`
	case "viewport":
		col = `COALESCE(viewport_width, 0) || '×' || COALESCE(viewport_height, 0)`
	default:
		return entries, fmt.Errorf("unknown dimension: %s", column)
	}

	query := `SELECT ` + col + `, COUNT(*), COUNT(DISTINCT visitor_id)
		FROM analytics_page_views
		WHERE is_bot = FALSE AND occurred_at >= $1 AND occurred_at < $2`
	args := []any{start, end}
	if projectID != "" {
		query += ` AND project_id = $3`
		args = append(args, projectID)
	}
	query += ` GROUP BY 1 ORDER BY COUNT(*) DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return entries, fmt.Errorf("query %s: %w", column, err)
	}
	defer rows.Close()

	var total int64
	type row struct {
		name     string
		views    int64
		visitors int64
	}
	var rows_ []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.name, &r.views, &r.visitors); err != nil {
			return entries, fmt.Errorf("scan %s: %w", column, err)
		}
		rows_ = append(rows_, r)
		total += r.views
	}
	if err := rows.Err(); err != nil {
		return entries, fmt.Errorf("iterate %s: %w", column, err)
	}
	for _, r := range rows_ {
		entry := DeviceBreakdown{Name: r.name, PageViews: r.views, Visitors: r.visitors}
		if total > 0 {
			entry.Percentage = float64(r.views*10000/total) / 100
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Realtime reports visitors seen within the activity window, plus the
// pages they were last seen on. Bots are excluded.
func (repository *PostgresRepository) Realtime(ctx context.Context, workspaceID, trackingID string) (Realtime, error) {
	realtime := Realtime{Pages: []RealtimePage{}}

	filter := `occurred_at >= NOW() - INTERVAL '` + realtimeWindow + `' AND is_bot = FALSE`
	var args []any
	if trackingID != "" {
		projectID, err := repository.resolveProjectID(ctx, workspaceID, trackingID)
		if err != nil {
			return realtime, err
		}
		filter += ` AND project_id = $1`
		args = append(args, projectID)
	}

	if err := repository.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT visitor_id) FROM analytics_page_views WHERE `+filter, args...).Scan(&realtime.ActiveVisitors); err != nil {
		return realtime, fmt.Errorf("query realtime visitors: %w", err)
	}

	rows, err := repository.pool.Query(ctx, `SELECT path, COUNT(DISTINCT visitor_id) FROM analytics_page_views WHERE `+filter+` GROUP BY path ORDER BY COUNT(*) DESC LIMIT 50`, args...)
	if err != nil {
		return realtime, fmt.Errorf("query realtime pages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var page RealtimePage
		if err := rows.Scan(&page.Path, &page.Visitors); err != nil {
			return realtime, fmt.Errorf("scan realtime page: %w", err)
		}
		realtime.Pages = append(realtime.Pages, page)
	}
	if err := rows.Err(); err != nil {
		return realtime, fmt.Errorf("iterate realtime pages: %w", err)
	}
	return realtime, nil
}
