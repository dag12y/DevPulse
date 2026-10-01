package analytics

import (
	"context"
	"fmt"
	"sort"

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
}

// resolveProjectID maps a public tracking ID to the internal project UUID.
// Reads are allowed on disabled projects; only unknown IDs report
// ErrUnknownProject.
func (repository *PostgresRepository) resolveProjectID(ctx context.Context, trackingID string) (string, error) {
	var projectID string
	err := repository.pool.QueryRow(ctx,
		`SELECT id::text FROM analytics_projects WHERE tracking_id = $1`, trackingID).Scan(&projectID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrUnknownProject
		}
		return "", fmt.Errorf("find project: %w", err)
	}
	return projectID, nil
}

func (repository *PostgresRepository) Summary(ctx context.Context, trackingID string) (Summary, error) {
	var summary Summary

	pageViewsQuery := `SELECT COUNT(*), COUNT(DISTINCT visitor_id) FROM analytics_page_views WHERE is_bot = FALSE`
	sessionsQuery := `SELECT COUNT(*),
		COALESCE(AVG(CASE WHEN is_bounce THEN 1.0 ELSE 0.0 END), 0),
		COALESCE(AVG(EXTRACT(EPOCH FROM (last_seen_at - started_at))), 0)
		FROM analytics_sessions WHERE is_bot = FALSE`

	var args []any
	if trackingID != "" {
		projectID, err := repository.resolveProjectID(ctx, trackingID)
		if err != nil {
			return summary, err
		}
		pageViewsQuery += ` AND project_id = $1`
		sessionsQuery += ` AND project_id = $1`
		args = append(args, projectID)
	}

	if err := repository.pool.QueryRow(ctx, pageViewsQuery, args...).Scan(
		&summary.TotalPageViews, &summary.UniqueVisitors); err != nil {
		return summary, fmt.Errorf("query page view summary: %w", err)
	}
	if err := repository.pool.QueryRow(ctx, sessionsQuery, args...).Scan(
		&summary.Sessions, &summary.BounceRate, &summary.AvgSessionDuration); err != nil {
		return summary, fmt.Errorf("query session summary: %w", err)
	}
	return summary, nil
}

func (repository *PostgresRepository) Traffic(ctx context.Context, trackingID string, days int) ([]TrafficPoint, error) {
	points := []TrafficPoint{}

	projectFilter := ""
	args := []any{days}
	if trackingID != "" {
		projectID, err := repository.resolveProjectID(ctx, trackingID)
		if err != nil {
			return points, err
		}
		projectFilter = `AND pv.project_id = $2`
		args = append(args, projectID)
	}

	rows, err := repository.pool.Query(ctx, `
		SELECT TO_CHAR(day, 'YYYY-MM-DD') AS date,
			COUNT(pv.id) AS page_views,
			COUNT(DISTINCT pv.visitor_id) AS visitors
		FROM generate_series(
			CURRENT_DATE - (($1::int - 1) * INTERVAL '1 day'),
			CURRENT_DATE,
			INTERVAL '1 day'
		) AS day
		LEFT JOIN analytics_page_views pv
			ON DATE(pv.occurred_at) = day AND pv.is_bot = FALSE `+projectFilter+`
		GROUP BY day
		ORDER BY day`, args...)
	if err != nil {
		return points, fmt.Errorf("query traffic: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var point TrafficPoint
		if err := rows.Scan(&point.Date, &point.PageViews, &point.Visitors); err != nil {
			return points, fmt.Errorf("scan traffic point: %w", err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return points, fmt.Errorf("iterate traffic points: %w", err)
	}
	return points, nil
}

func (repository *PostgresRepository) TopPages(ctx context.Context, trackingID string, limit int) ([]TopPage, error) {
	pages := []TopPage{}

	query := `SELECT path, COUNT(*) AS views, COUNT(DISTINCT visitor_id) AS unique_visitors
		FROM analytics_page_views WHERE is_bot = FALSE`
	var args []any
	if trackingID != "" {
		projectID, err := repository.resolveProjectID(ctx, trackingID)
		if err != nil {
			return pages, err
		}
		query += ` AND project_id = $1`
		args = append(args, projectID)
	}
	if len(args) == 0 {
		query += ` GROUP BY path ORDER BY views DESC LIMIT $1`
		args = append(args, limit)
	} else {
		query += ` GROUP BY path ORDER BY views DESC LIMIT $2`
		args = append(args, limit)
	}

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

// Sources aggregates non-bot page views by traffic source. Raw
// referrer/UTM groups come from PostgreSQL and are classified in Go
// (ClassifySource), so known hosts collapse to friendly names.
func (repository *PostgresRepository) Sources(ctx context.Context, trackingID string) ([]Source, error) {
	sources := []Source{}

	query := `SELECT COALESCE(referrer, ''), COALESCE(utm_source, ''), COUNT(*), COUNT(DISTINCT visitor_id)
		FROM analytics_page_views WHERE is_bot = FALSE`
	var args []any
	if trackingID != "" {
		projectID, err := repository.resolveProjectID(ctx, trackingID)
		if err != nil {
			return sources, err
		}
		query += ` AND project_id = $1`
		args = append(args, projectID)
	}
	query += ` GROUP BY referrer, utm_source`

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
func (repository *PostgresRepository) Countries(ctx context.Context, trackingID string) ([]Country, error) {
	countries := []Country{}

	query := `SELECT COALESCE(NULLIF(country, ''), 'Unknown'), COUNT(*), COUNT(DISTINCT visitor_id)
		FROM analytics_page_views WHERE is_bot = FALSE`
	var args []any
	if trackingID != "" {
		projectID, err := repository.resolveProjectID(ctx, trackingID)
		if err != nil {
			return countries, err
		}
		query += ` AND project_id = $1`
		args = append(args, projectID)
	}
	query += ` GROUP BY 1 ORDER BY COUNT(*) DESC`

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
func (repository *PostgresRepository) Devices(ctx context.Context, trackingID string) (Devices, error) {
	var devices Devices

	var projectID string
	if trackingID != "" {
		var err error
		projectID, err = repository.resolveProjectID(ctx, trackingID)
		if err != nil {
			return devices, err
		}
	}

	var err error
	if devices.DeviceTypes, err = repository.breakdown(ctx, "device_type", projectID); err != nil {
		return devices, err
	}
	if devices.Browsers, err = repository.breakdown(ctx, "browser", projectID); err != nil {
		return devices, err
	}
	if devices.OperatingSystems, err = repository.breakdown(ctx, "os", projectID); err != nil {
		return devices, err
	}
	return devices, nil
}

// breakdown aggregates a single whitelisted dimension column.
func (repository *PostgresRepository) breakdown(ctx context.Context, column, projectID string) ([]DeviceBreakdown, error) {
	entries := []DeviceBreakdown{}

	var col string
	switch column {
	case "device_type":
		col = `COALESCE(NULLIF(device_type, ''), 'unknown')`
	case "browser":
		col = `COALESCE(NULLIF(browser, ''), 'Other')`
	case "os":
		col = `COALESCE(NULLIF(os, ''), 'Other')`
	default:
		return entries, fmt.Errorf("unknown dimension: %s", column)
	}

	query := `SELECT ` + col + `, COUNT(*), COUNT(DISTINCT visitor_id)
		FROM analytics_page_views WHERE is_bot = FALSE`
	var args []any
	if projectID != "" {
		query += ` AND project_id = $1`
		args = append(args, projectID)
	}
	query += ` GROUP BY 1 ORDER BY COUNT(*) DESC`

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
