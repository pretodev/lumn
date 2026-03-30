-- name: CreateExecution :one
INSERT INTO executions (workflow_id, trigger_type, status, queued_at, started_at, finished_at, report)
VALUES (
  sqlc.arg(workflow_id),
  sqlc.arg(trigger_type),
  sqlc.arg(status),
  sqlc.arg(queued_at),
  sqlc.narg(started_at),
  sqlc.narg(finished_at),
  sqlc.narg(report)
)
RETURNING id, workflow_id, trigger_type, status, queued_at, started_at, finished_at, report;

-- name: MarkExecutionRunning :exec
UPDATE executions
SET status = sqlc.arg(status),
    started_at = sqlc.arg(started_at),
    finished_at = NULL
WHERE id = sqlc.arg(id);

-- name: CompleteExecution :exec
UPDATE executions
SET status = sqlc.arg(status),
    finished_at = sqlc.arg(finished_at),
    report = sqlc.arg(report)
WHERE id = sqlc.arg(id);

-- name: GetExecution :one
SELECT id, workflow_id, trigger_type, status, queued_at, started_at, finished_at, report
FROM executions
WHERE id = sqlc.arg(id);

-- name: LatestExecutionByWorkflow :one
SELECT id, workflow_id, trigger_type, status, queued_at, started_at, finished_at, report
FROM executions
WHERE workflow_id = sqlc.arg(workflow_id)
ORDER BY COALESCE(finished_at, started_at, queued_at) DESC, id DESC
LIMIT 1;

-- name: ListQueuedExecutionVersionsByWorkflow :many
SELECT q.execution_id, w.name, w.version
FROM queue q
JOIN workflows w ON w.id = q.workflow_id
WHERE q.workflow_id = sqlc.arg(workflow_id);

-- name: ListStaleRunningExecutions :many
SELECT e.id, e.workflow_id, w.name, w.version
FROM executions e
JOIN workflows w ON w.id = e.workflow_id
WHERE e.status = sqlc.arg(status);

-- name: DeleteExecutionsOlderThan :exec
DELETE FROM executions
WHERE status NOT IN (sqlc.arg(queued_status), sqlc.arg(running_status))
  AND finished_at IS NOT NULL
  AND finished_at < sqlc.arg(cutoff);

-- name: DeleteExecutionOverflowByWorkflow :exec
DELETE FROM executions
WHERE id IN (
  SELECT stale.id
  FROM executions AS stale
  WHERE stale.workflow_id = sqlc.arg(workflow_id)
    AND stale.status NOT IN (sqlc.arg(queued_status), sqlc.arg(running_status))
  ORDER BY COALESCE(stale.finished_at, stale.started_at, stale.queued_at) DESC, stale.id DESC
  LIMIT -1 OFFSET sqlc.arg(keep_limit)
);
