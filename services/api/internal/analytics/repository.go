package analytics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sessionTimeout = 30 * time.Minute

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) Ingest(ctx context.Context, event Event) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin event transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	projectID, err := projectForTrackingID(ctx, tx, event.ProjectID)
	if err != nil {
		return err
	}

	var duplicate bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM analytics_page_views WHERE event_id = $1)`, event.EventID).Scan(&duplicate); err != nil {
		return fmt.Errorf("check duplicate event: %w", err)
	}
	if duplicate {
		return ErrDuplicateEvent
	}

	visitorID, err := upsertVisitor(ctx, tx, projectID, event)
	if err != nil {
		return err
	}
	sessionID, err := upsertSession(ctx, tx, projectID, visitorID, event)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `INSERT INTO analytics_page_views (
		event_id, project_id, visitor_id, session_id, path, title, referrer,
		screen_width, screen_height, viewport_width, viewport_height, language, timezone,
		utm_source, utm_medium, utm_campaign, utm_term, utm_content, occurred_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`,
		event.EventID, projectID, visitorID, sessionID, event.Page.Path, nullable(event.Page.Title), nullable(event.Page.Referrer),
		event.Screen.Width, event.Screen.Height, event.Viewport.Width, event.Viewport.Height, nullable(event.Language), event.Timezone,
		event.Campaign.Source, event.Campaign.Medium, event.Campaign.Campaign, event.Campaign.Term, event.Campaign.Content, event.Timestamp,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateEvent
		}
		return fmt.Errorf("insert page view: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit event transaction: %w", err)
	}
	return nil
}

func projectForTrackingID(ctx context.Context, tx pgx.Tx, trackingID string) (string, error) {
	var projectID string
	var enabled bool
	err := tx.QueryRow(ctx, `SELECT id::text, enabled FROM analytics_projects WHERE tracking_id = $1`, trackingID).Scan(&projectID, &enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnknownProject
	}
	if err != nil {
		return "", fmt.Errorf("find project: %w", err)
	}
	if !enabled {
		return "", ErrDisabledProject
	}
	return projectID, nil
}

func upsertVisitor(ctx context.Context, tx pgx.Tx, projectID string, event Event) (string, error) {
	var visitorID string
	err := tx.QueryRow(ctx, `INSERT INTO analytics_visitors (
		project_id, visitor_key, first_seen_at, last_seen_at, first_referrer
	) VALUES ($1, $2, $3, $3, $4)
	ON CONFLICT (project_id, visitor_key) DO UPDATE SET
		last_seen_at = GREATEST(analytics_visitors.last_seen_at, EXCLUDED.last_seen_at),
		updated_at = NOW()
	RETURNING id::text`, projectID, event.VisitorID, event.Timestamp, nullable(event.Page.Referrer)).Scan(&visitorID)
	if err != nil {
		return "", fmt.Errorf("upsert visitor: %w", err)
	}
	return visitorID, nil
}

func upsertSession(ctx context.Context, tx pgx.Tx, projectID, visitorID string, event Event) (string, error) {
	var sessionID string
	var pageViews int
	err := tx.QueryRow(ctx, `SELECT id::text, page_views FROM analytics_sessions
		WHERE project_id = $1 AND visitor_id = $2 AND last_seen_at BETWEEN $3 AND $4
		ORDER BY last_seen_at DESC LIMIT 1 FOR UPDATE`, projectID, visitorID, event.Timestamp.Add(-sessionTimeout), event.Timestamp).Scan(&sessionID, &pageViews)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO analytics_sessions (
			project_id, visitor_id, started_at, last_seen_at, landing_page, exit_page, page_views, referrer, is_bounce
		) VALUES ($1, $2, $3, $3, $4, $4, 1, $5, true) RETURNING id::text`,
			projectID, visitorID, event.Timestamp, event.Page.Path, nullable(event.Page.Referrer)).Scan(&sessionID)
		if err != nil {
			return "", fmt.Errorf("create session: %w", err)
		}
		return sessionID, nil
	}
	if err != nil {
		return "", fmt.Errorf("find active session: %w", err)
	}

	err = tx.QueryRow(ctx, `UPDATE analytics_sessions SET
		last_seen_at = $1, exit_page = $2, page_views = $3, is_bounce = false
		WHERE id = $4 RETURNING id::text`, event.Timestamp, event.Page.Path, pageViews+1, sessionID).Scan(&sessionID)
	if err != nil {
		return "", fmt.Errorf("update session: %w", err)
	}
	return sessionID, nil
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
