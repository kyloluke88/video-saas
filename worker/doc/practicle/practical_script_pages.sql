-- public.practical_script_pages definition

CREATE TABLE public.practical_script_pages (
  id BIGSERIAL PRIMARY KEY,

  slug VARCHAR(160) NOT NULL,
  project_id VARCHAR(120) NOT NULL,

  language VARCHAR(16) NOT NULL,
  audience_language VARCHAR(16),
  difficulty_level VARCHAR(16),

  title TEXT NOT NULL,
  en_title TEXT,
  subtitle TEXT,
  summary TEXT,

  cover_image_url TEXT,
  video_url TEXT,
  youtube_video_id VARCHAR(32),
  youtube_video_url TEXT,

  translation_locales TEXT[] NOT NULL DEFAULT '{}'::TEXT[],

  seo_title TEXT,
  seo_description TEXT,
  seo_keywords TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
  canonical_url TEXT,

  script_json JSONB NOT NULL DEFAULT '{}'::JSONB,
  vocabulary_json JSONB NOT NULL DEFAULT '[]'::JSONB,
  grammar_json JSONB NOT NULL DEFAULT '[]'::JSONB,
  downloads_json JSONB NOT NULL DEFAULT '[]'::JSONB,

  status VARCHAR(24) NOT NULL DEFAULT 'published',
  published_at TIMESTAMPTZ,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_practical_script_pages_slug
  ON public.practical_script_pages (slug);

CREATE UNIQUE INDEX idx_practical_script_pages_project_id
  ON public.practical_script_pages (project_id);

CREATE INDEX idx_practical_script_pages_status
  ON public.practical_script_pages (status);
