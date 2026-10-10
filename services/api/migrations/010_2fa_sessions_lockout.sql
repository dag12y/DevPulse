-- Phase 4: two-factor auth, session transparency, account lockout.

-- users: TOTP enrollment (secret pending until verified; enabled_at
-- flips only after the user proves the authenticator works) and
-- per-account failed-login counters. The per-IP limiter remains the
-- first line; this backs it up against distributed password guessing.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS totp_secret TEXT,
    ADD COLUMN IF NOT EXISTS totp_enabled_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS failed_login_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;

-- user_sessions: enough metadata for a "where am I signed in" list.
-- ip is the rate-limiting client address (never analytics data);
-- user_agent is truncated display text, not a fingerprint.
ALTER TABLE user_sessions
    ADD COLUMN IF NOT EXISTS ip TEXT,
    ADD COLUMN IF NOT EXISTS user_agent TEXT,
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;
