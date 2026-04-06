CREATE TABLE IF NOT EXISTS podcast_script_pages (
    id BIGSERIAL PRIMARY KEY,
    slug VARCHAR(160) NOT NULL UNIQUE,
    project_id VARCHAR(120),
    language VARCHAR(16) NOT NULL,
    audience_language VARCHAR(16),
    title TEXT NOT NULL,
    subtitle TEXT,
    summary TEXT,
    cover_image_url TEXT,
    seo_title TEXT,
    seo_description TEXT,
    seo_keywords TEXT[] NOT NULL DEFAULT '{}',
    canonical_url TEXT,
    script_json JSONB NOT NULL DEFAULT '{"intro":"","sections":[]}'::jsonb,
    vocabulary_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    grammar_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    downloads_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    sidebar_json JSONB NOT NULL DEFAULT '{"products":[]}'::jsonb,
    rendered_html TEXT,
    html_path TEXT,
    pdf_path TEXT,
    pdf_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT podcast_script_pages_status_check CHECK (status IN ('draft', 'published', 'archived')),
    CONSTRAINT podcast_script_pages_pdf_status_check CHECK (pdf_status IN ('pending', 'processing', 'ready', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_podcast_script_pages_status_published_at
    ON podcast_script_pages (status, published_at DESC);

CREATE INDEX IF NOT EXISTS idx_podcast_script_pages_project_id
    ON podcast_script_pages (project_id);

CREATE INDEX IF NOT EXISTS idx_podcast_script_pages_language
    ON podcast_script_pages (language);

CREATE INDEX IF NOT EXISTS idx_podcast_script_pages_script_json_gin
    ON podcast_script_pages
    USING GIN (script_json);

CREATE INDEX IF NOT EXISTS idx_podcast_script_pages_vocabulary_json_gin
    ON podcast_script_pages
    USING GIN (vocabulary_json);

CREATE INDEX IF NOT EXISTS idx_podcast_script_pages_grammar_json_gin
    ON podcast_script_pages
    USING GIN (grammar_json);

CREATE TABLE IF NOT EXISTS merchandise_products (
    id BIGSERIAL PRIMARY KEY,
    slug VARCHAR(160) NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT,
    image_url TEXT,
    price_label VARCHAR(64),
    buy_url TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT merchandise_products_status_check CHECK (status IN ('draft', 'active', 'archived'))
);

CREATE INDEX IF NOT EXISTS idx_merchandise_products_status_sort
    ON merchandise_products (status, sort_order, id);

CREATE TABLE IF NOT EXISTS podcast_script_page_products (
    page_id BIGINT NOT NULL REFERENCES podcast_script_pages(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES merchandise_products(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (page_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_podcast_script_page_products_page_sort
    ON podcast_script_page_products (page_id, sort_order, product_id);

CREATE OR REPLACE FUNCTION touch_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_podcast_script_pages_touch_updated_at ON podcast_script_pages;
CREATE TRIGGER trg_podcast_script_pages_touch_updated_at
BEFORE UPDATE ON podcast_script_pages
FOR EACH ROW
EXECUTE FUNCTION touch_updated_at_column();

DROP TRIGGER IF EXISTS trg_merchandise_products_touch_updated_at ON merchandise_products;
CREATE TRIGGER trg_merchandise_products_touch_updated_at
BEFORE UPDATE ON merchandise_products
FOR EACH ROW
EXECUTE FUNCTION touch_updated_at_column();

COMMENT ON TABLE podcast_script_pages IS 'SEO-friendly podcast script content pages with structured script, vocabulary, grammar, and download metadata.';
COMMENT ON COLUMN podcast_script_pages.script_json IS 'Structured script body, recommended shape: {"intro":"","sections":[{"heading":"","body":"","lines":[...]}]}';
COMMENT ON COLUMN podcast_script_pages.vocabulary_json IS 'Vocabulary list JSON array.';
COMMENT ON COLUMN podcast_script_pages.grammar_json IS 'Grammar summary JSON array.';
COMMENT ON COLUMN podcast_script_pages.downloads_json IS 'Download asset list JSON array.';
COMMENT ON COLUMN podcast_script_pages.sidebar_json IS 'Sidebar payload, recommended shape: {"products":[]}';
