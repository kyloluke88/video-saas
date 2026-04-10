CREATE TABLE IF NOT EXISTS podcast_script_pages (
    id BIGSERIAL PRIMARY KEY,
    slug VARCHAR(160) NOT NULL UNIQUE,
    project_id VARCHAR(120) NOT NULL UNIQUE
        REFERENCES projects(project_id) ON DELETE CASCADE,
    language VARCHAR(16) NOT NULL,
    audience_language VARCHAR(16),
    title TEXT NOT NULL,
    subtitle TEXT,
    summary TEXT,
    cover_image_url TEXT,
    video_url TEXT,
    youtube_video_id VARCHAR(32),
    youtube_video_url TEXT,
    seo_title TEXT,
    seo_description TEXT,
    seo_keywords TEXT[] NOT NULL DEFAULT '{}',
    canonical_url TEXT,
    script_json JSONB NOT NULL DEFAULT '{"sections":[]}'::jsonb,
    vocabulary_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    grammar_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    downloads_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT podcast_script_pages_status_check CHECK (status IN ('draft', 'published', 'archived'))
);

CREATE INDEX IF NOT EXISTS idx_podcast_script_pages_status_published_at
    ON podcast_script_pages (status, published_at DESC);
