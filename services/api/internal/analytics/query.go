package analytics

import (
	"context"
	"fmt"

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

	pageViewsQuery := `SELECT COUNT(*), COUNT(DISTINCT visitor_id) FROM analytics_page_views`
	sessionsQuery := `SELECT COUNT(*),
		COALESCE(AVG(CASE WHEN is_bounce THEN 1.0 ELSE 0.0 END), 0),
		COALESCE(AVG(EXTRACT(EPOCH FROM (last_seen_at - started_at))), 0)
		FROM analytics_sessions`

	var args []any
	if trackingID != "" {
		projectID, err := repository.resolveProjectID(ctx, trackingID)
		if err != nil {
			return summary, err
		}
		pageViewsQuery += ` WHERE project_id = $1`
		sessionsQuery += ` WHERE project_id = $1`
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
			ON DATE(pv.occurred_at) = day `+projectFilter+`
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
		FROM analytics_page_views`
	var args []any
	if trackingID != "" {
		projectID, err := repository.resolveProjectID(ctx, trackingID)
		if err != nil {
			return pages, err
		}
		query += ` WHERE project_id = $1`
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
