package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pretodev/lumn/internal/executor"
	_ "modernc.org/sqlite"
)

const (
	StatusActive   = "active"
	StatusStopped  = "stopped"
	StatusQueued   = "queued"
	StatusRunning  = "running"
	StatusOK       = "ok"
	StatusError    = "error"
	StatusEmpty    = "empty"
	TriggerActive  = "active"
	TriggerStopped = "stopped"
)

type Store struct {
	db *sql.DB
}

type Workflow struct {
	ID        string
	Version   string
	Path      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Trigger struct {
	ID         int64
	WorkflowID string
	Type       string
	Config     map[string]any
	NextRunAt  *time.Time
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Execution struct {
	ID          int64
	WorkflowID  string
	TriggerType string
	Status      string
	QueuedAt    time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time
	Report      *executor.Report
}

type QueueItem struct {
	ID             int64
	ExecutionID    int64
	WorkflowID     string
	TriggerType    string
	TriggerContext map[string]any
	QueuedAt       time.Time
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) UpsertWorkflow(workflow Workflow) error {
	now := utcNow()
	if workflow.CreatedAt.IsZero() {
		workflow.CreatedAt = now
	}
	if workflow.UpdatedAt.IsZero() {
		workflow.UpdatedAt = now
	}

	_, err := s.db.Exec(`
INSERT INTO workflows (id, version, path, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  version = excluded.version,
  path = excluded.path,
  status = excluded.status,
  updated_at = excluded.updated_at
`, workflow.ID, workflow.Version, workflow.Path, workflow.Status, formatTime(workflow.CreatedAt), formatTime(workflow.UpdatedAt))
	return err
}

func (s *Store) DeleteWorkflow(id string) error {
	_, err := s.db.Exec(`DELETE FROM workflows WHERE id = ?`, id)
	return err
}

func (s *Store) GetWorkflow(id string) (Workflow, bool, error) {
	row := s.db.QueryRow(`SELECT id, version, path, status, created_at, updated_at FROM workflows WHERE id = ?`, id)
	workflow, found, err := scanWorkflow(row)
	return workflow, found, err
}

func (s *Store) ListWorkflows() ([]Workflow, error) {
	rows, err := s.db.Query(`SELECT id, version, path, status, created_at, updated_at FROM workflows ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workflows := []Workflow{}
	for rows.Next() {
		workflow, err := scanWorkflowFromRows(rows)
		if err != nil {
			return nil, err
		}
		workflows = append(workflows, workflow)
	}
	return workflows, rows.Err()
}

func (s *Store) ListActiveWorkflows() ([]Workflow, error) {
	rows, err := s.db.Query(`SELECT id, version, path, status, created_at, updated_at FROM workflows WHERE status = ? ORDER BY id`, StatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workflows := []Workflow{}
	for rows.Next() {
		workflow, err := scanWorkflowFromRows(rows)
		if err != nil {
			return nil, err
		}
		workflows = append(workflows, workflow)
	}
	return workflows, rows.Err()
}

func (s *Store) SetWorkflowStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE workflows SET status = ?, updated_at = ? WHERE id = ?`, status, formatTime(utcNow()), id)
	return err
}

func (s *Store) ReplaceWorkflowTriggers(workflowID string, triggers []Trigger) ([]Trigger, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)

	if _, err := tx.Exec(`DELETE FROM triggers WHERE workflow_id = ?`, workflowID); err != nil {
		return nil, err
	}

	now := utcNow()
	stored := make([]Trigger, 0, len(triggers))
	for _, trigger := range triggers {
		payload, err := json.Marshal(trigger.Config)
		if err != nil {
			return nil, err
		}
		result, err := tx.Exec(`
INSERT INTO triggers (workflow_id, type, config, next_run_at, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, workflowID, trigger.Type, string(payload), nullableTime(trigger.NextRunAt), trigger.Status, formatTime(now), formatTime(now))
		if err != nil {
			return nil, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return nil, err
		}
		trigger.ID = id
		trigger.WorkflowID = workflowID
		trigger.CreatedAt = now
		trigger.UpdatedAt = now
		stored = append(stored, trigger)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *Store) ListTriggers(workflowID string) ([]Trigger, error) {
	rows, err := s.db.Query(`
SELECT id, workflow_id, type, config, next_run_at, status, created_at, updated_at
FROM triggers
WHERE workflow_id = ?
ORDER BY id
`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	triggers := []Trigger{}
	for rows.Next() {
		trigger, err := scanTrigger(rows)
		if err != nil {
			return nil, err
		}
		triggers = append(triggers, trigger)
	}
	return triggers, rows.Err()
}

func (s *Store) SetTriggerRuntime(triggerID int64, status string, nextRunAt *time.Time) error {
	_, err := s.db.Exec(`
UPDATE triggers
SET status = ?, next_run_at = ?, updated_at = ?
WHERE id = ?
`, status, nullableTime(nextRunAt), formatTime(utcNow()), triggerID)
	return err
}

func (s *Store) QueueCount(workflowID string) (int, error) {
	row := s.db.QueryRow(`SELECT COUNT(*) FROM queue WHERE workflow_id = ?`, workflowID)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) CreateQueuedExecution(workflowID, triggerType string, triggerContext map[string]any) (Execution, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return Execution{}, err
	}
	defer rollback(tx)

	now := utcNow()
	result, err := tx.Exec(`
INSERT INTO executions (workflow_id, trigger_type, status, queued_at, started_at, finished_at, report)
VALUES (?, ?, ?, ?, NULL, NULL, NULL)
`, workflowID, triggerType, StatusQueued, formatTime(now))
	if err != nil {
		return Execution{}, err
	}
	executionID, err := result.LastInsertId()
	if err != nil {
		return Execution{}, err
	}

	payload, err := json.Marshal(triggerContext)
	if err != nil {
		return Execution{}, err
	}
	if _, err := tx.Exec(`
INSERT INTO queue (execution_id, workflow_id, trigger_type, trigger_context, queued_at)
VALUES (?, ?, ?, ?, ?)
`, executionID, workflowID, triggerType, string(payload), formatTime(now)); err != nil {
		return Execution{}, err
	}

	if err := tx.Commit(); err != nil {
		return Execution{}, err
	}

	return Execution{
		ID:          executionID,
		WorkflowID:  workflowID,
		TriggerType: triggerType,
		Status:      StatusQueued,
		QueuedAt:    now,
	}, nil
}

func (s *Store) ClaimNextQueueItem(workflowID string) (QueueItem, bool, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return QueueItem{}, false, err
	}
	defer rollback(tx)

	row := tx.QueryRow(`
SELECT id, execution_id, workflow_id, trigger_type, trigger_context, queued_at
FROM queue
WHERE workflow_id = ?
ORDER BY queued_at, id
LIMIT 1
`, workflowID)

	item, found, err := scanQueueItem(row)
	if err != nil || !found {
		return item, found, err
	}

	now := utcNow()
	if _, err := tx.Exec(`DELETE FROM queue WHERE id = ?`, item.ID); err != nil {
		return QueueItem{}, false, err
	}
	if _, err := tx.Exec(`
UPDATE executions
SET status = ?, started_at = ?, finished_at = NULL
WHERE id = ?
`, StatusRunning, formatTime(now), item.ExecutionID); err != nil {
		return QueueItem{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return QueueItem{}, false, err
	}

	item.ID = item.ID
	return item, true, nil
}

func (s *Store) CompleteExecution(executionID int64, status string, report executor.Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
UPDATE executions
SET status = ?, finished_at = ?, report = ?
WHERE id = ?
`, status, formatTime(utcNow()), string(payload), executionID)
	return err
}

func (s *Store) CancelQueuedExecutions(workflowID, message string) ([]int64, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)

	rows, err := tx.Query(`
SELECT q.execution_id, w.version
FROM queue q
JOIN workflows w ON w.id = q.workflow_id
WHERE q.workflow_id = ?
`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type canceled struct {
		executionID int64
		version     string
	}
	items := []canceled{}
	for rows.Next() {
		var item canceled
		if err := rows.Scan(&item.executionID, &item.version); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(`DELETE FROM queue WHERE workflow_id = ?`, workflowID); err != nil {
		return nil, err
	}

	now := formatTime(utcNow())
	for _, item := range items {
		reportPayload, err := json.Marshal(executor.Report{
			Workflow: workflowID,
			Version:  item.version,
			Status:   StatusError,
			Errors: []executor.ReportError{{
				Type:    "generic",
				Message: message,
			}},
		})
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`
UPDATE executions
SET status = ?, finished_at = ?, report = ?
WHERE id = ?
`, StatusError, now, string(reportPayload), item.executionID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	executionIDs := make([]int64, 0, len(items))
	for _, item := range items {
		executionIDs = append(executionIDs, item.executionID)
	}
	return executionIDs, nil
}

func (s *Store) MarkStaleRunningExecutions(message string) error {
	rows, err := s.db.Query(`
SELECT e.id, e.workflow_id, w.version
FROM executions e
JOIN workflows w ON w.id = e.workflow_id
WHERE e.status = ?
`, StatusRunning)
	if err != nil {
		return err
	}
	defer rows.Close()

	type stale struct {
		executionID int64
		workflowID  string
		version     string
	}
	staleExecutions := []stale{}
	for rows.Next() {
		var item stale
		if err := rows.Scan(&item.executionID, &item.workflowID, &item.version); err != nil {
			return err
		}
		staleExecutions = append(staleExecutions, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range staleExecutions {
		report := executor.Report{
			Workflow: item.workflowID,
			Version:  item.version,
			Status:   StatusError,
			Errors: []executor.ReportError{{
				Type:    "runtime",
				Message: message,
			}},
		}
		if err := s.CompleteExecution(item.executionID, StatusError, report); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) GetExecution(id int64) (Execution, bool, error) {
	row := s.db.QueryRow(`
SELECT id, workflow_id, trigger_type, status, queued_at, started_at, finished_at, report
FROM executions
WHERE id = ?
`, id)
	return scanExecution(row)
}

func (s *Store) LatestExecution(workflowID string) (Execution, bool, error) {
	row := s.db.QueryRow(`
SELECT id, workflow_id, trigger_type, status, queued_at, started_at, finished_at, report
FROM executions
WHERE workflow_id = ?
ORDER BY COALESCE(finished_at, started_at, queued_at) DESC, id DESC
LIMIT 1
`, workflowID)
	return scanExecution(row)
}

func (s *Store) CountActiveWorkflows() (int, error) {
	row := s.db.QueryRow(`SELECT COUNT(*) FROM workflows WHERE status = ?`, StatusActive)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) ApplyRetention(maxExecutions, maxDays int) error {
	if maxDays > 0 {
		cutoff := utcNow().Add(-time.Duration(maxDays) * 24 * time.Hour)
		if _, err := s.db.Exec(`
DELETE FROM executions
WHERE status NOT IN (?, ?)
  AND finished_at IS NOT NULL
  AND finished_at < ?
`, StatusQueued, StatusRunning, formatTime(cutoff)); err != nil {
			return err
		}
	}

	if maxExecutions > 0 {
		workflows, err := s.ListWorkflows()
		if err != nil {
			return err
		}
		for _, workflow := range workflows {
			if _, err := s.db.Exec(`
DELETE FROM executions
WHERE id IN (
  SELECT id
  FROM executions
  WHERE workflow_id = ?
    AND status NOT IN (?, ?)
  ORDER BY COALESCE(finished_at, started_at, queued_at) DESC, id DESC
  LIMIT -1 OFFSET ?
)
`, workflow.ID, StatusQueued, StatusRunning, maxExecutions); err != nil {
				return err
			}
		}
	}

	return nil
}

func scanWorkflow(row *sql.Row) (Workflow, bool, error) {
	var workflow Workflow
	var createdAt string
	var updatedAt string
	if err := row.Scan(&workflow.ID, &workflow.Version, &workflow.Path, &workflow.Status, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workflow{}, false, nil
		}
		return Workflow{}, false, err
	}

	var err error
	workflow.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Workflow{}, false, err
	}
	workflow.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Workflow{}, false, err
	}
	return workflow, true, nil
}

func scanWorkflowFromRows(rows *sql.Rows) (Workflow, error) {
	var workflow Workflow
	var createdAt string
	var updatedAt string
	if err := rows.Scan(&workflow.ID, &workflow.Version, &workflow.Path, &workflow.Status, &createdAt, &updatedAt); err != nil {
		return Workflow{}, err
	}
	var err error
	workflow.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Workflow{}, err
	}
	workflow.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Workflow{}, err
	}
	return workflow, nil
}

func scanTrigger(scanner interface{ Scan(dest ...any) error }) (Trigger, error) {
	var trigger Trigger
	var config string
	var nextRunAt sql.NullString
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(&trigger.ID, &trigger.WorkflowID, &trigger.Type, &config, &nextRunAt, &trigger.Status, &createdAt, &updatedAt); err != nil {
		return Trigger{}, err
	}
	if err := json.Unmarshal([]byte(config), &trigger.Config); err != nil {
		return Trigger{}, err
	}
	var err error
	trigger.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Trigger{}, err
	}
	trigger.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Trigger{}, err
	}
	trigger.NextRunAt, err = parseNullTime(nextRunAt)
	if err != nil {
		return Trigger{}, err
	}
	return trigger, nil
}

func scanQueueItem(row *sql.Row) (QueueItem, bool, error) {
	var item QueueItem
	var contextPayload string
	var queuedAt string
	if err := row.Scan(&item.ID, &item.ExecutionID, &item.WorkflowID, &item.TriggerType, &contextPayload, &queuedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return QueueItem{}, false, nil
		}
		return QueueItem{}, false, err
	}
	if contextPayload != "" {
		if err := json.Unmarshal([]byte(contextPayload), &item.TriggerContext); err != nil {
			return QueueItem{}, false, err
		}
	}
	if item.TriggerContext == nil {
		item.TriggerContext = map[string]any{}
	}
	timeValue, err := parseTime(queuedAt)
	if err != nil {
		return QueueItem{}, false, err
	}
	item.QueuedAt = timeValue
	return item, true, nil
}

func scanExecution(row *sql.Row) (Execution, bool, error) {
	var execution Execution
	var queuedAt string
	var startedAt sql.NullString
	var finishedAt sql.NullString
	var report sql.NullString
	if err := row.Scan(&execution.ID, &execution.WorkflowID, &execution.TriggerType, &execution.Status, &queuedAt, &startedAt, &finishedAt, &report); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Execution{}, false, nil
		}
		return Execution{}, false, err
	}
	var err error
	execution.QueuedAt, err = parseTime(queuedAt)
	if err != nil {
		return Execution{}, false, err
	}
	execution.StartedAt, err = parseNullTime(startedAt)
	if err != nil {
		return Execution{}, false, err
	}
	execution.FinishedAt, err = parseNullTime(finishedAt)
	if err != nil {
		return Execution{}, false, err
	}
	if report.Valid && report.String != "" {
		var parsed executor.Report
		if err := json.Unmarshal([]byte(report.String), &parsed); err != nil {
			return Execution{}, false, err
		}
		execution.Report = &parsed
	}
	return execution, true, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(value.UTC())
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func parseNullTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func utcNow() time.Time {
	return time.Now().UTC()
}

func rollback(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}

func (s *Store) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS workflows (
			id TEXT PRIMARY KEY,
			version TEXT NOT NULL,
			path TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS triggers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workflow_id TEXT NOT NULL,
			type TEXT NOT NULL,
			config TEXT NOT NULL,
			next_run_at TEXT,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS executions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workflow_id TEXT NOT NULL,
			trigger_type TEXT NOT NULL,
			status TEXT NOT NULL,
			queued_at TEXT NOT NULL,
			started_at TEXT,
			finished_at TEXT,
			report TEXT,
			FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			execution_id INTEGER NOT NULL,
			workflow_id TEXT NOT NULL,
			trigger_type TEXT NOT NULL,
			trigger_context TEXT NOT NULL,
			queued_at TEXT NOT NULL,
			FOREIGN KEY(execution_id) REFERENCES executions(id) ON DELETE CASCADE,
			FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_triggers_workflow_id ON triggers(workflow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_executions_workflow_id ON executions(workflow_id, queued_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_queue_workflow_id ON queue(workflow_id, queued_at ASC)`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("migrate sqlite schema: %w", err)
		}
	}
	return nil
}
