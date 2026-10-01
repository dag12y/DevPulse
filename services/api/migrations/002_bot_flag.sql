-- DevPulse bot-flag enrichment columns.
-- Bot page views are stored but excluded from visitor metrics.

ALTER TABLE analytics_page_views
    ADD COLUMN IF NOT EXISTS is_bot BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE analytics_sessions
    ADD COLUMN IF NOT EXISTS is_bot BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_analytics_page_views_project_bot_occurred
    ON analytics_page_views(project_id, is_bot, occurred_at DESC);
