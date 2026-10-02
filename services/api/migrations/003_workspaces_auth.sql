-- DevPulse workspaces and API-key authentication.
-- Private APIs require a workspace-scoped Bearer key. Ingestion stays
-- public (tracking ID only) but reads/mutations are workspace-isolated.

CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workspace_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    key_prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL DEFAULT 'admin',
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT workspace_api_keys_role_check
        CHECK (role IN ('owner', 'admin', 'viewer'))
);

CREATE INDEX IF NOT EXISTS idx_workspace_api_keys_workspace
    ON workspace_api_keys(workspace_id);

CREATE INDEX IF NOT EXISTS idx_workspace_api_keys_prefix
    ON workspace_api_keys(key_prefix);

-- Backfill a default workspace for pre-workspace projects, then enforce
-- workspace ownership on all projects going forward.
DO $$
DECLARE
    default_workspace_id UUID;
BEGIN
    IF EXISTS (SELECT 1 FROM analytics_projects WHERE workspace_id IS NULL) THEN
        INSERT INTO workspaces (name)
        VALUES ('Default workspace')
        RETURNING id INTO default_workspace_id;

        UPDATE analytics_projects
        SET workspace_id = default_workspace_id
        WHERE workspace_id IS NULL;
    END IF;
END
$$;

-- Enforce the foreign key (idempotent) and require workspace ownership.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'analytics_projects_workspace_id_fkey'
    ) THEN
        ALTER TABLE analytics_projects
            ADD CONSTRAINT analytics_projects_workspace_id_fkey
            FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
    END IF;
END
$$;

ALTER TABLE analytics_projects
    ALTER COLUMN workspace_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_analytics_projects_workspace
    ON analytics_projects(workspace_id);
