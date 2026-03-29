-- name: CountQueueByWorkflow :one
SELECT COUNT(*)
FROM queue
WHERE workflow_id = sqlc.arg(workflow_id);

-- name: CreateQueueItem :one
INSERT INTO queue (execution_id, workflow_id, trigger_type, trigger_context, queued_at)
VALUES (
  sqlc.arg(execution_id),
  sqlc.arg(workflow_id),
  sqlc.arg(trigger_type),
  sqlc.arg(trigger_context),
  sqlc.arg(queued_at)
)
RETURNING id, execution_id, workflow_id, trigger_type, trigger_context, queued_at;

-- name: GetNextQueueItemByWorkflow :one
SELECT id, execution_id, workflow_id, trigger_type, trigger_context, queued_at
FROM queue
WHERE workflow_id = sqlc.arg(workflow_id)
ORDER BY queued_at, id
LIMIT 1;

-- name: DeleteQueueItem :exec
DELETE FROM queue
WHERE id = sqlc.arg(id);

-- name: DeleteQueueByWorkflow :exec
DELETE FROM queue
WHERE workflow_id = sqlc.arg(workflow_id);
