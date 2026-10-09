-- Email verification and password reset.
--
-- Accounts created before verification existed are backfilled as
-- verified, so the new login gate never locks out existing users.
-- The backfill runs once (schema_migrations tracks this file); new
-- signups start unverified and flip to verified on a one-time link.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;

UPDATE users SET email_verified_at = NOW() WHERE email_verified_at IS NULL;

-- Single-purpose, single-use tokens (email verification, password
-- reset). Only the SHA-256 hash is stored — the same rule as sessions
-- and API keys: a database leak must not yield working links. Issuing
-- a new token for (user, kind) deletes the previous one, so at most
-- one live link per purpose exists.
CREATE TABLE IF NOT EXISTS auth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT auth_tokens_kind_check CHECK (kind IN ('verify_email', 'password_reset'))
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_kind ON auth_tokens(user_id, kind);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_expires_at ON auth_tokens(expires_at);
