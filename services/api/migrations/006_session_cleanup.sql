-- DevPulse login-session hygiene.
--
-- Session expiry is already enforced at read time (FindSession rejects rows
-- past expires_at), so this migration adds no new behaviour for users. It
-- makes the cleanup observable and keeps the table from growing without
-- bound: without a sweep, every login leaves a row forever.
--
-- The sessions_deleted / visitors_deleted counters in retention_runs refer to
-- analytics_sessions and analytics_visitors. Login rows are a separate
-- lifecycle, so they get their own counter rather than being folded in.

ALTER TABLE retention_runs
    ADD COLUMN IF NOT EXISTS auth_sessions_deleted BIGINT NOT NULL DEFAULT 0;

-- Cleanup scans for expired or revoked rows on every pass.
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at
    ON user_sessions(expires_at);

-- The sweep also filters on revoked_at; a partial index keeps
-- recent-logout lookups cheap without indexing live rows.
CREATE INDEX IF NOT EXISTS idx_user_sessions_revoked_at
    ON user_sessions(revoked_at) WHERE revoked_at IS NOT NULL;
