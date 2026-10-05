-- DevPulse workspace CRUD hardening.
--
-- Rename/delete ship in the API with no schema change to ownership, so
-- this migration only tightens the workspaces table itself:
--   - forbid empty/blank-adjacent names at the DB layer (API trims and
--     validates first; this is the backstop for direct SQL writes);
--   - cap length to match ValidateWorkspaceName (255 chars).
--
-- Deletion relies on existing ON DELETE CASCADE foreign keys:
-- analytics_projects, workspace_api_keys, and workspace_members all
-- cascade from workspaces(id), so DELETE FROM workspaces removes the
-- workspace and everything it owns in one statement.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'workspaces_name_not_empty'
    ) THEN
        ALTER TABLE workspaces
            ADD CONSTRAINT workspaces_name_not_empty CHECK (name <> '');
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'workspaces_name_max_length'
    ) THEN
        ALTER TABLE workspaces
            ADD CONSTRAINT workspaces_name_max_length CHECK (char_length(name) <= 255);
    END IF;
END
$$;
