-- DevPulse retention cleanup observability.
-- Each worker pass records per-run totals so cleanup is observable from
-- the database (and later from the dashboard).

CREATE TABLE IF NOT EXISTS retention_runs (
    id BIGSERIAL PRIMARY KEY,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    projects_processed INTEGER NOT NULL DEFAULT 0,
    page_views_deleted BIGINT NOT NULL DEFAULT 0,
    sessions_deleted BIGINT NOT NULL DEFAULT 0,
    visitors_deleted BIGINT NOT NULL DEFAULT 0,
    -- Login-session purges are counted separately from analytics rows.
    -- Existing databases gain this via 006_session_cleanup.sql.
    auth_sessions_deleted BIGINT NOT NULL DEFAULT 0,
    error TEXT
);
