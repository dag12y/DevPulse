// Package retention enforces per-project data retention by deleting
// expired analytics rows. Cutoff per project is now - retention_days:
//
//  1. page views older than the cutoff,
//  2. sessions idle since before the cutoff with no remaining page views,
//  3. visitors unseen since before the cutoff with no remaining rows.
//
// The NOT EXISTS guards make each step safe to re-run and order
// independent: a parent row survives while any child row references it.
package retention

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Result summarizes one cleanup pass. It is also persisted to
// retention_runs so cleanup stays observable without log access.
type Result struct {
	ProjectsProcessed int
	PageViewsDeleted  int64
	SessionsDeleted   int64
	VisitorsDeleted   int64
	// AuthSessionsDeleted counts expired or revoked login rows removed.
	AuthSessionsDeleted int64
}

// revokedSessionGrace is how long a revoked or expired login row is kept
// before deletion. The grace period means an operator still sees recent
// logouts, and it guarantees a token stays valid for at least as long as
// its own expires_at even if a sweep runs moments before expiry.
const revokedSessionGrace = 24 * time.Hour

// Cutoff returns the instant before which a project's rows expire.
func Cutoff(now time.Time, retentionDays int) time.Time {
	return now.AddDate(0, 0, -retentionDays)
}

// RunOnce processes every project exactly once. It records the run even
// when nothing expired so operators can distinguish "ran clean" from
// "never ran".
func RunOnce(ctx context.Context, pool *pgxpool.Pool, now time.Time) (Result, error) {
	var result Result

	var runID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO retention_runs DEFAULT VALUES RETURNING id`).Scan(&runID); err != nil {
		return result, fmt.Errorf("record retention run: %w", err)
	}
	finish := func(runErr error) (Result, error) {
		message := ""
		if runErr != nil {
			message = runErr.Error()
		}
		if _, err := pool.Exec(ctx, `UPDATE retention_runs SET
			finished_at = NOW(), projects_processed = $2,
			page_views_deleted = $3, sessions_deleted = $4,
			visitors_deleted = $5, auth_sessions_deleted = $6,
			error = NULLIF($7, '')
			WHERE id = $1`,
			runID, result.ProjectsProcessed, result.PageViewsDeleted,
			result.SessionsDeleted, result.VisitorsDeleted,
			result.AuthSessionsDeleted, message); err != nil {
			return result, fmt.Errorf("finish retention run: %w", err)
		}
		return result, runErr
	}

	rows, err := pool.Query(ctx, `SELECT id, retention_days FROM analytics_projects`)
	if err != nil {
		return finish(fmt.Errorf("list projects: %w", err))
	}
	type project struct {
		id            string
		retentionDays int
	}
	var projects []project
	for rows.Next() {
		var item project
		if err := rows.Scan(&item.id, &item.retentionDays); err != nil {
			rows.Close()
			return finish(fmt.Errorf("scan project: %w", err))
		}
		projects = append(projects, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return finish(fmt.Errorf("iterate projects: %w", err))
	}

	for _, item := range projects {
		cutoff := Cutoff(now, item.retentionDays)
		deleted, err := purgeProject(ctx, pool, item.id, cutoff)
		if err != nil {
			return finish(fmt.Errorf("purge project %s: %w", item.id, err))
		}
		result.ProjectsProcessed++
		result.PageViewsDeleted += deleted.pageViews
		result.SessionsDeleted += deleted.sessions
		result.VisitorsDeleted += deleted.visitors
	}

	// Login sessions are not project analytics data and are cleaned on their
	// own schedule. Expiry is already enforced when a session is read, so this
	// only bounds table growth and keeps recent logouts visible.
	authDeleted, err := purgeAuthSessions(ctx, pool, now)
	if err != nil {
		return finish(fmt.Errorf("purge auth sessions: %w", err))
	}
	result.AuthSessionsDeleted = authDeleted

	return finish(nil)
}

// purgeAuthSessions deletes login rows that expired or were revoked more than
// revokedSessionGrace ago. Live sessions are never touched, so this cannot
// log anyone out.
func purgeAuthSessions(ctx context.Context, pool *pgxpool.Pool, now time.Time) (int64, error) {
	tag, err := pool.Exec(ctx,
		`DELETE FROM user_sessions
		  WHERE (expires_at <= $1 OR revoked_at IS NOT NULL)
		    AND COALESCE(revoked_at, expires_at) < $2`,
		now, now.Add(-revokedSessionGrace))
	if err != nil {
		return 0, fmt.Errorf("delete user_sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

type purged struct {
	pageViews int64
	sessions  int64
	visitors  int64
}

func purgeProject(ctx context.Context, pool *pgxpool.Pool, projectID string, cutoff time.Time) (purged, error) {
	var result purged

	pageViews, err := pool.Exec(ctx,
		`DELETE FROM analytics_page_views WHERE project_id = $1 AND occurred_at < $2`,
		projectID, cutoff)
	if err != nil {
		return result, fmt.Errorf("delete page views: %w", err)
	}
	result.pageViews = pageViews.RowsAffected()

	sessions, err := pool.Exec(ctx,
		`DELETE FROM analytics_sessions s
		  WHERE s.project_id = $1 AND s.last_seen_at < $2
		  AND NOT EXISTS (SELECT 1 FROM analytics_page_views pv WHERE pv.session_id = s.id)`,
		projectID, cutoff)
	if err != nil {
		return result, fmt.Errorf("delete sessions: %w", err)
	}
	result.sessions = sessions.RowsAffected()

	visitors, err := pool.Exec(ctx,
		`DELETE FROM analytics_visitors v
		  WHERE v.project_id = $1 AND v.last_seen_at < $2
		  AND NOT EXISTS (SELECT 1 FROM analytics_sessions s WHERE s.visitor_id = v.id)
		  AND NOT EXISTS (SELECT 1 FROM analytics_page_views pv WHERE pv.visitor_id = v.id)`,
		projectID, cutoff)
	if err != nil {
		return result, fmt.Errorf("delete visitors: %w", err)
	}
	result.visitors = visitors.RowsAffected()

	return result, nil
}
