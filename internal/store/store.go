package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/pretodev/lumn/internal/executor"
	dbsqlc "github.com/pretodev/lumn/internal/store/sqlc"
	sqldata "github.com/pretodev/lumn/sql"
	migrate "github.com/rubenv/sql-migrate"
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
	db      *sql.DB
	queries *dbsqlc.Queries
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

	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{
		db:      db,
		queries: dbsqlc.New(db),
	}, nil
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
	return s.queries.UpsertWorkflow(context.Background(), dbsqlc.UpsertWorkflowParams{
		ID:        workflow.ID,
		Version:   workflow.Version,
		Path:      workflow.Path,
		Status:    workflow.Status,
		CreatedAt: workflow.CreatedAt.UTC(),
		UpdatedAt: workflow.UpdatedAt.UTC(),
	})
}

func (s *Store) DeleteWorkflow(id string) error {
	return s.queries.DeleteWorkflow(context.Background(), id)
}

func (s *Store) GetWorkflow(id string) (Workflow, bool, error) {
	row, err := s.queries.GetWorkflow(context.Background(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return Workflow{}, false, nil
	}
	if err != nil {
		return Workflow{}, false, err
	}
	return fromWorkflowRow(row), true, nil
}

func (s *Store) ListWorkflows() ([]Workflow, error) {
	rows, err := s.queries.ListWorkflows(context.Background())
	if err != nil {
		return nil, err
	}
	items := make([]Workflow, 0, len(rows))
	for _, row := range rows {
		items = append(items, fromWorkflowRow(row))
	}
	return items, nil
}

func (s *Store) ListActiveWorkflows() ([]Workflow, error) {
	rows, err := s.queries.ListActiveWorkflows(context.Background(), StatusActive)
	if err != nil {
		return nil, err
	}
	items := make([]Workflow, 0, len(rows))
	for _, row := range rows {
		items = append(items, fromWorkflowRow(row))
	}
	return items, nil
}

func (s *Store) SetWorkflowStatus(id, status string) error {
	return s.queries.SetWorkflowStatus(context.Background(), dbsqlc.SetWorkflowStatusParams{
		Status:    status,
		UpdatedAt: utcNow(),
		ID:        id,
	})
}

func (s *Store) ReplaceWorkflowTriggers(workflowID string, triggers []Trigger) ([]Trigger, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)

	qtx := s.queries.WithTx(tx)
	if err := qtx.DeleteTriggersByWorkflow(context.Background(), workflowID); err != nil {
		return nil, err
	}

	now := utcNow()
	stored := make([]Trigger, 0, len(triggers))
	for _, trigger := range triggers {
		payload, err := json.Marshal(trigger.Config)
		if err != nil {
			return nil, err
		}
		row, err := qtx.CreateTrigger(context.Background(), dbsqlc.CreateTriggerParams{
			WorkflowID: workflowID,
			Type:       trigger.Type,
			Config:     string(payload),
			NextRunAt:  trigger.NextRunAt,
			Status:     trigger.Status,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
		if err != nil {
			return nil, err
		}
		item, err := fromTriggerRow(row)
		if err != nil {
			return nil, err
		}
		stored = append(stored, item)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *Store) ListTriggers(workflowID string) ([]Trigger, error) {
	rows, err := s.queries.ListTriggersByWorkflow(context.Background(), workflowID)
	if err != nil {
		return nil, err
	}
	items := make([]Trigger, 0, len(rows))
	for _, row := range rows {
		item, err := fromTriggerRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) SetTriggerRuntime(triggerID int64, status string, nextRunAt *time.Time) error {
	return s.queries.SetTriggerRuntime(context.Background(), dbsqlc.SetTriggerRuntimeParams{
		Status:    status,
		NextRunAt: nextRunAt,
		UpdatedAt: utcNow(),
		ID:        triggerID,
	})
}

func (s *Store) QueueCount(workflowID string) (int, error) {
	count, err := s.queries.CountQueueByWorkflow(context.Background(), workflowID)
	return int(count), err
}

func (s *Store) CreateQueuedExecution(workflowID, triggerType string, triggerContext map[string]any) (Execution, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return Execution{}, err
	}
	defer rollback(tx)

	qtx := s.queries.WithTx(tx)
	now := utcNow()
	executionRow, err := qtx.CreateExecution(context.Background(), dbsqlc.CreateExecutionParams{
		WorkflowID:  workflowID,
		TriggerType: triggerType,
		Status:      StatusQueued,
		QueuedAt:    now,
	})
	if err != nil {
		return Execution{}, err
	}

	payload, err := json.Marshal(triggerContext)
	if err != nil {
		return Execution{}, err
	}
	if _, err := qtx.CreateQueueItem(context.Background(), dbsqlc.CreateQueueItemParams{
		ExecutionID:    executionRow.ID,
		WorkflowID:     workflowID,
		TriggerType:    triggerType,
		TriggerContext: string(payload),
		QueuedAt:       now,
	}); err != nil {
		return Execution{}, err
	}

	if err := tx.Commit(); err != nil {
		return Execution{}, err
	}
	return fromExecutionRow(executionRow)
}

func (s *Store) ClaimNextQueueItem(workflowID string) (QueueItem, bool, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return QueueItem{}, false, err
	}
	defer rollback(tx)

	qtx := s.queries.WithTx(tx)
	queueRow, err := qtx.GetNextQueueItemByWorkflow(context.Background(), workflowID)
	if errors.Is(err, sql.ErrNoRows) {
		return QueueItem{}, false, nil
	}
	if err != nil {
		return QueueItem{}, false, err
	}
	if err := qtx.DeleteQueueItem(context.Background(), queueRow.ID); err != nil {
		return QueueItem{}, false, err
	}
	now := utcNow()
	if err := qtx.MarkExecutionRunning(context.Background(), dbsqlc.MarkExecutionRunningParams{
		Status:    StatusRunning,
		StartedAt: &now,
		ID:        queueRow.ExecutionID,
	}); err != nil {
		return QueueItem{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return QueueItem{}, false, err
	}
	item, err := fromQueueRow(queueRow)
	return item, err == nil, err
}

func (s *Store) CompleteExecution(executionID int64, status string, report executor.Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return err
	}
	finishedAt := utcNow()
	reportString := string(payload)
	return s.queries.CompleteExecution(context.Background(), dbsqlc.CompleteExecutionParams{
		Status:     status,
		FinishedAt: &finishedAt,
		Report:     &reportString,
		ID:         executionID,
	})
}

func (s *Store) CancelQueuedExecutions(workflowID, message string) ([]int64, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)

	qtx := s.queries.WithTx(tx)
	rows, err := qtx.ListQueuedExecutionVersionsByWorkflow(context.Background(), workflowID)
	if err != nil {
		return nil, err
	}
	if err := qtx.DeleteQueueByWorkflow(context.Background(), workflowID); err != nil {
		return nil, err
	}

	executionIDs := make([]int64, 0, len(rows))
	finishedAt := utcNow()
	for _, row := range rows {
		reportPayload, err := json.Marshal(executor.Report{
			Workflow: workflowID,
			Version:  row.Version,
			Status:   StatusError,
			Errors: []executor.ReportError{{
				Type:    "generic",
				Message: message,
			}},
		})
		if err != nil {
			return nil, err
		}
		reportString := string(reportPayload)
		if err := qtx.CompleteExecution(context.Background(), dbsqlc.CompleteExecutionParams{
			Status:     StatusError,
			FinishedAt: &finishedAt,
			Report:     &reportString,
			ID:         row.ExecutionID,
		}); err != nil {
			return nil, err
		}
		executionIDs = append(executionIDs, row.ExecutionID)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return executionIDs, nil
}

func (s *Store) MarkStaleRunningExecutions(message string) error {
	rows, err := s.queries.ListStaleRunningExecutions(context.Background(), StatusRunning)
	if err != nil {
		return err
	}
	for _, row := range rows {
		report := executor.Report{
			Workflow: row.WorkflowID,
			Version:  row.Version,
			Status:   StatusError,
			Errors: []executor.ReportError{{
				Type:    "runtime",
				Message: message,
			}},
		}
		if err := s.CompleteExecution(row.ID, StatusError, report); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetExecution(id int64) (Execution, bool, error) {
	row, err := s.queries.GetExecution(context.Background(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return Execution{}, false, nil
	}
	if err != nil {
		return Execution{}, false, err
	}
	item, err := fromExecutionRow(row)
	if err != nil {
		return Execution{}, false, err
	}
	return item, true, nil
}

func (s *Store) LatestExecution(workflowID string) (Execution, bool, error) {
	row, err := s.queries.LatestExecutionByWorkflow(context.Background(), workflowID)
	if errors.Is(err, sql.ErrNoRows) {
		return Execution{}, false, nil
	}
	if err != nil {
		return Execution{}, false, err
	}
	item, err := fromExecutionRow(row)
	if err != nil {
		return Execution{}, false, err
	}
	return item, true, nil
}

func (s *Store) CountActiveWorkflows() (int, error) {
	count, err := s.queries.CountActiveWorkflows(context.Background(), StatusActive)
	return int(count), err
}

func (s *Store) ApplyRetention(maxExecutions, maxDays int) error {
	ctx := context.Background()
	if maxDays > 0 {
		cutoff := utcNow().Add(-time.Duration(maxDays) * 24 * time.Hour)
		if err := s.queries.DeleteExecutionsOlderThan(ctx, dbsqlc.DeleteExecutionsOlderThanParams{
			QueuedStatus:  StatusQueued,
			RunningStatus: StatusRunning,
			Cutoff:        &cutoff,
		}); err != nil {
			return err
		}
	}
	if maxExecutions > 0 {
		workflows, err := s.ListWorkflows()
		if err != nil {
			return err
		}
		for _, workflow := range workflows {
			if err := s.queries.DeleteExecutionOverflowByWorkflow(ctx, dbsqlc.DeleteExecutionOverflowByWorkflowParams{
				WorkflowID:    workflow.ID,
				QueuedStatus:  StatusQueued,
				RunningStatus: StatusRunning,
				KeepLimit:     int64(maxExecutions),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func runMigrations(db *sql.DB) error {
	source := &migrate.EmbedFileSystemMigrationSource{
		FileSystem: sqldata.MigrationsFS,
		Root:       "migrations",
	}
	_, err := migrate.Exec(db, "sqlite3", source, migrate.Up)
	return err
}

func fromWorkflowRow(row dbsqlc.Workflow) Workflow {
	return Workflow{
		ID:        row.ID,
		Version:   row.Version,
		Path:      row.Path,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func fromTriggerRow(row dbsqlc.Trigger) (Trigger, error) {
	var config map[string]any
	if err := json.Unmarshal([]byte(row.Config), &config); err != nil {
		return Trigger{}, err
	}
	return Trigger{
		ID:         row.ID,
		WorkflowID: row.WorkflowID,
		Type:       row.Type,
		Config:     config,
		NextRunAt:  row.NextRunAt,
		Status:     row.Status,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}, nil
}

func fromExecutionRow(row dbsqlc.Execution) (Execution, error) {
	var report *executor.Report
	if row.Report != nil && *row.Report != "" {
		parsed := executor.Report{}
		if err := json.Unmarshal([]byte(*row.Report), &parsed); err != nil {
			return Execution{}, err
		}
		report = &parsed
	}
	return Execution{
		ID:          row.ID,
		WorkflowID:  row.WorkflowID,
		TriggerType: row.TriggerType,
		Status:      row.Status,
		QueuedAt:    row.QueuedAt,
		StartedAt:   row.StartedAt,
		FinishedAt:  row.FinishedAt,
		Report:      report,
	}, nil
}

func fromQueueRow(row dbsqlc.Queue) (QueueItem, error) {
	var triggerContext map[string]any
	if err := json.Unmarshal([]byte(row.TriggerContext), &triggerContext); err != nil {
		return QueueItem{}, err
	}
	return QueueItem{
		ID:             row.ID,
		ExecutionID:    row.ExecutionID,
		WorkflowID:     row.WorkflowID,
		TriggerType:    row.TriggerType,
		TriggerContext: triggerContext,
		QueuedAt:       row.QueuedAt,
	}, nil
}

func utcNow() time.Time {
	return time.Now().UTC()
}

func rollback(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}
