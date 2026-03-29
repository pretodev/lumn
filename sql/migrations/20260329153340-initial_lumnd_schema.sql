
-- +migrate Up
CREATE TABLE workflows (
  id TEXT PRIMARY KEY,
  version TEXT NOT NULL,
  path TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE TABLE triggers (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  workflow_id TEXT NOT NULL,
  type TEXT NOT NULL,
  config TEXT NOT NULL,
  next_run_at DATETIME,
  status TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

CREATE TABLE executions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  workflow_id TEXT NOT NULL,
  trigger_type TEXT NOT NULL,
  status TEXT NOT NULL,
  queued_at DATETIME NOT NULL,
  started_at DATETIME,
  finished_at DATETIME,
  report TEXT,
  FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

CREATE TABLE queue (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  execution_id INTEGER NOT NULL,
  workflow_id TEXT NOT NULL,
  trigger_type TEXT NOT NULL,
  trigger_context TEXT NOT NULL,
  queued_at DATETIME NOT NULL,
  FOREIGN KEY(execution_id) REFERENCES executions(id) ON DELETE CASCADE,
  FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

CREATE INDEX idx_triggers_workflow_id ON triggers(workflow_id);
CREATE INDEX idx_executions_workflow_id ON executions(workflow_id, queued_at DESC);
CREATE INDEX idx_queue_workflow_id ON queue(workflow_id, queued_at ASC);

-- +migrate Down
DROP INDEX IF EXISTS idx_queue_workflow_id;
DROP INDEX IF EXISTS idx_executions_workflow_id;
DROP INDEX IF EXISTS idx_triggers_workflow_id;

DROP TABLE IF EXISTS queue;
DROP TABLE IF EXISTS executions;
DROP TABLE IF EXISTS triggers;
DROP TABLE IF EXISTS workflows;
