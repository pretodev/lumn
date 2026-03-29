-- name: DeleteTriggersByWorkflow :exec
DELETE FROM triggers
WHERE workflow_id = sqlc.arg(workflow_id);

-- name: CreateTrigger :one
INSERT INTO triggers (workflow_id, type, config, next_run_at, status, created_at, updated_at)
VALUES (
  sqlc.arg(workflow_id),
  sqlc.arg(type),
  sqlc.arg(config),
  sqlc.narg(next_run_at),
  sqlc.arg(status),
  sqlc.arg(created_at),
  sqlc.arg(updated_at)
)
RETURNING id, workflow_id, type, config, next_run_at, status, created_at, updated_at;

-- name: ListTriggersByWorkflow :many
SELECT id, workflow_id, type, config, next_run_at, status, created_at, updated_at
FROM triggers
WHERE workflow_id = sqlc.arg(workflow_id)
ORDER BY id;

-- name: SetTriggerRuntime :exec
UPDATE triggers
SET status = sqlc.arg(status),
    next_run_at = sqlc.narg(next_run_at),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);
