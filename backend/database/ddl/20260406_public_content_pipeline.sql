ALTER TABLE podcast_script_pages
    ADD COLUMN IF NOT EXISTS video_url TEXT,
    ADD COLUMN IF NOT EXISTS youtube_video_id VARCHAR(32),
    ADD COLUMN IF NOT EXISTS youtube_video_url TEXT;

CREATE TABLE IF NOT EXISTS video_projects (
    id BIGSERIAL PRIMARY KEY,
    project_id VARCHAR(120) NOT NULL UNIQUE,
    content_type VARCHAR(32) NOT NULL,
    language VARCHAR(16),
    title TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    current_stage VARCHAR(64),
    current_task_type VARCHAR(120),
    last_error TEXT,
    request_payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    video_url TEXT,
    youtube_video_id VARCHAR(32),
    youtube_video_url TEXT,
    script_page_id BIGINT REFERENCES podcast_script_pages(id) ON DELETE SET NULL,
    script_page_slug VARCHAR(160),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT video_projects_status_check CHECK (status IN ('queued', 'running', 'retrying', 'succeeded', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_video_projects_content_type_status
    ON video_projects (content_type, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_video_projects_current_stage
    ON video_projects (current_stage, updated_at DESC);

CREATE TABLE IF NOT EXISTS video_project_task_runs (
    id BIGSERIAL PRIMARY KEY,
    project_id VARCHAR(120) NOT NULL REFERENCES video_projects(project_id) ON DELETE CASCADE,
    task_id VARCHAR(160) NOT NULL,
    task_type VARCHAR(120) NOT NULL,
    stage VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    retry_count INTEGER NOT NULL DEFAULT 0,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT video_project_task_runs_status_check CHECK (status IN ('running', 'retrying', 'succeeded', 'failed')),
    CONSTRAINT video_project_task_runs_task_retry_unique UNIQUE (task_id, retry_count)
);

CREATE INDEX IF NOT EXISTS idx_video_project_task_runs_project_stage
    ON video_project_task_runs (project_id, stage, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_video_project_task_runs_status_started_at
    ON video_project_task_runs (status, started_at DESC);

DROP TRIGGER IF EXISTS trg_video_projects_touch_updated_at ON video_projects;
CREATE TRIGGER trg_video_projects_touch_updated_at
BEFORE UPDATE ON video_projects
FOR EACH ROW
EXECUTE FUNCTION touch_updated_at_column();

DROP TRIGGER IF EXISTS trg_video_project_task_runs_touch_updated_at ON video_project_task_runs;
CREATE TRIGGER trg_video_project_task_runs_touch_updated_at
BEFORE UPDATE ON video_project_task_runs
FOR EACH ROW
EXECUTE FUNCTION touch_updated_at_column();

COMMENT ON TABLE video_projects IS 'High-level state for generated video projects across podcast and other content pipelines.';
COMMENT ON TABLE video_project_task_runs IS 'Per-task execution state for each project, including retries and latest error.';
