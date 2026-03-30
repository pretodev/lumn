-- name: UpsertWorkflow :exec
INSERT INTO workflows (id, name, version, path, status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(version), sqlc.arg(path), sqlc.arg(status), sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT(id) DO UPDATE SET
  name = excluded.name,
  version = excluded.version,
  path = excluded.path,
  status = excluded.status,
  updated_at = excluded.updated_at;

-- name: DeleteWorkflow :exec
DELETE FROM workflows
WHERE id = sqlc.arg(id);

-- name: GetWorkflow :one
SELECT id, name, version, path, status, created_at, updated_at
FROM workflows
WHERE id = sqlc.arg(id);

-- name: ListWorkflows :many
SELECT id, name, version, path, status, created_at, updated_at
FROM workflows
ORDER BY name, version, id;

-- name: ListActiveWorkflows :many
SELECT id, name, version, path, status, created_at, updated_at
FROM workflows
WHERE status = sqlc.arg(status)
ORDER BY name, version, id;

-- name: SetWorkflowStatus :exec
UPDATE workflows
SET status = sqlc.arg(status), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: CountActiveWorkflows :one
SELECT COUNT(*)
FROM workflows
WHERE status = sqlc.arg(status);
