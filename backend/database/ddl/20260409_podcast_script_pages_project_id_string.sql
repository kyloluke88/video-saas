DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'podcast_script_pages'
          AND column_name = 'project_id'
          AND data_type = 'bigint'
    ) THEN
        IF EXISTS (
            SELECT 1
            FROM podcast_script_pages psp
            LEFT JOIN projects p ON p.id = psp.project_id
            WHERE psp.project_id IS NOT NULL
              AND p.project_id IS NULL
        ) THEN
            RAISE EXCEPTION 'podcast_script_pages.project_id contains values that cannot be mapped to projects.project_id';
        END IF;

        ALTER TABLE podcast_script_pages
            DROP CONSTRAINT IF EXISTS podcast_script_pages_project_id_fkey;

        ALTER TABLE podcast_script_pages
            ADD COLUMN project_id_v2 VARCHAR(120);

        UPDATE podcast_script_pages psp
        SET project_id_v2 = p.project_id
        FROM projects p
        WHERE p.id = psp.project_id;

        ALTER TABLE podcast_script_pages
            DROP COLUMN project_id;

        ALTER TABLE podcast_script_pages
            RENAME COLUMN project_id_v2 TO project_id;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM (
            SELECT project_id
            FROM podcast_script_pages
            GROUP BY project_id
            HAVING COUNT(*) > 1
        ) duplicated
    ) THEN
        RAISE EXCEPTION 'podcast_script_pages.project_id contains duplicate values';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM podcast_script_pages psp
        LEFT JOIN projects p ON p.project_id = psp.project_id
        WHERE p.project_id IS NULL
    ) THEN
        RAISE EXCEPTION 'podcast_script_pages.project_id contains values that do not exist in projects.project_id';
    END IF;
END $$;

ALTER TABLE podcast_script_pages
    ALTER COLUMN project_id TYPE VARCHAR(120),
    ALTER COLUMN project_id SET NOT NULL;

DROP INDEX IF EXISTS idx_podcast_script_pages_project_id;

ALTER TABLE podcast_script_pages
    DROP CONSTRAINT IF EXISTS podcast_script_pages_project_id_key,
    DROP CONSTRAINT IF EXISTS podcast_script_pages_project_id_fkey;

ALTER TABLE podcast_script_pages
    ADD CONSTRAINT podcast_script_pages_project_id_key UNIQUE (project_id);

ALTER TABLE podcast_script_pages
    ADD CONSTRAINT podcast_script_pages_project_id_fkey
    FOREIGN KEY (project_id)
    REFERENCES projects(project_id)
    ON DELETE CASCADE;
