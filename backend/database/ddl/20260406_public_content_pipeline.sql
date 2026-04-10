CREATE TABLE IF NOT EXISTS projects (
    id BIGSERIAL PRIMARY KEY,
    project_id VARCHAR(120) NOT NULL,
    content_type VARCHAR(32) NOT NULL,
    run_mode INTEGER NOT NULL DEFAULT 0,
    retry_num INTEGER NOT NULL DEFAULT 0,
    current_stage VARCHAR(64),
    current_task_type VARCHAR(120),
    terminated_task_type VARCHAR(120),
    -- status: 0=queued, 1=running, 2=retrying, 3=finished, 4=error, 5=cancelling, 6=cancelled
    status SMALLINT NOT NULL DEFAULT 0,
    last_error TEXT,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    cancel_requested_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    cancel_source VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT projects_status_check CHECK (status IN (0, 1, 2, 3, 4, 5, 6))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_project_id
    ON projects (project_id);

CREATE INDEX IF NOT EXISTS idx_projects_status
    ON projects (status);
