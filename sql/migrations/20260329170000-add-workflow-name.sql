-- +migrate Up
ALTER TABLE workflows ADD COLUMN name TEXT;

UPDATE workflows
SET name = id
WHERE name IS NULL OR name = '';

CREATE UNIQUE INDEX idx_workflows_name_version ON workflows(name, version);

-- +migrate Down
DROP INDEX IF EXISTS idx_workflows_name_version;
