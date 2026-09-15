-- DevPulse initial analytics schema

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Projects
CREATE TABLE analytics_projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID,
    name VARCHAR(255) NOT NULL,
    tracking_id VARCHAR(64) NOT NULL UNIQUE,
    allowed_domains TEXT[] NOT NULL DEFAULT '{}',
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    retention_days INTEGER NOT NULL DEFAULT 90,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT analytics_projects_retention_days_check
        CHECK (retention_days IN (30, 90, 180, 365))
);

-- Visitors
CREATE TABLE analytics_visitors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES analytics_projects(id) ON DELETE CASCADE,
    visitor_key VARCHAR(255) NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    first_country VARCHAR(2),
    last_country VARCHAR(2),
    first_referrer VARCHAR(2048),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT analytics_visitors_project_visitor_unique
        UNIQUE (project_id, visitor_key)
);

-- Sessions
CREATE TABLE analytics_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES analytics_projects(id) ON DELETE CASCADE,
    visitor_id UUID NOT NULL REFERENCES analytics_visitors(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    landing_page VARCHAR(1024),
    exit_page VARCHAR(1024),
    page_views INTEGER NOT NULL DEFAULT 0,
    referrer VARCHAR(2048),
    country VARCHAR(2),
    device_type VARCHAR(32),
    browser VARCHAR(64),
    browser_version VARCHAR(32),
    os VARCHAR(64),
    os_version VARCHAR(32),
    is_bounce BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Page views
CREATE TABLE analytics_page_views (
    id BIGSERIAL PRIMARY KEY,
    event_id UUID NOT NULL UNIQUE,
    project_id UUID NOT NULL REFERENCES analytics_projects(id) ON DELETE CASCADE,
    visitor_id UUID NOT NULL REFERENCES analytics_visitors(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES analytics_sessions(id) ON DELETE CASCADE,

    path VARCHAR(1024) NOT NULL,
    title VARCHAR(512),
    referrer VARCHAR(2048),

    country VARCHAR(2),
    region VARCHAR(128),

    device_type VARCHAR(32),
    browser VARCHAR(64),
    browser_version VARCHAR(32),
    os VARCHAR(64),
    os_version VARCHAR(32),

    screen_width INTEGER,
    screen_height INTEGER,
    viewport_width INTEGER,
    viewport_height INTEGER,

    language VARCHAR(32),
    timezone VARCHAR(64),

    utm_source VARCHAR(256),
    utm_medium VARCHAR(256),
    utm_campaign VARCHAR(256),
    utm_term VARCHAR(256),
    utm_content VARCHAR(256),

    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Project page-view queries
CREATE INDEX idx_analytics_page_views_project_occurred
    ON analytics_page_views(project_id, occurred_at DESC);

CREATE INDEX idx_analytics_page_views_project_path
    ON analytics_page_views(project_id, path);

CREATE INDEX idx_analytics_page_views_session
    ON analytics_page_views(session_id);

-- Session queries
CREATE INDEX idx_analytics_sessions_project_started
    ON analytics_sessions(project_id, started_at DESC);

CREATE INDEX idx_analytics_sessions_visitor
    ON analytics_sessions(visitor_id);

-- Visitor queries
CREATE INDEX idx_analytics_visitors_project_last_seen
    ON analytics_visitors(project_id, last_seen_at DESC);
