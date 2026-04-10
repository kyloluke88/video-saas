ALTER TABLE projects
    DROP CONSTRAINT IF EXISTS projects_status_check;

ALTER TABLE projects
    ADD CONSTRAINT projects_status_check
    CHECK (status IN (0, 1, 2, 3, 4, 5, 6));
