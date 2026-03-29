package daemon

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pretodev/lumn/internal/engine"
	"github.com/pretodev/lumn/internal/executor"
	"github.com/pretodev/lumn/internal/store"
	"github.com/robfig/cron/v3"
)

type Daemon struct {
	config    Config
	store     *store.Store
	logger    *log.Logger
	startTime time.Time

	internalListener net.Listener
	internalServer   *http.Server
	webhookListener  net.Listener
	webhookServer    *http.Server
	webhooks         *webhookRegistry

	mu           sync.Mutex
	workflows    map[string]*managedWorkflow
	waiters      map[int64]chan executor.Report
	shuttingDown bool
}

type managedWorkflow struct {
	workflow store.Workflow
	triggers []store.Trigger
	handles  []triggerHandle
	notifyCh chan struct{}
	stopCh   chan struct{}

	mu        sync.Mutex
	accepting bool
	running   bool
	manual    bool
}

type triggerHandle interface {
	Stop() error
}

type stopFunc func() error

func (fn stopFunc) Stop() error {
	return fn()
}

func New(config Config, stderr io.Writer) (*Daemon, error) {
	if err := config.Paths.EnsureStateDir(); err != nil {
		return nil, err
	}

	storeDB, err := store.Open(config.Paths.DBPath)
	if err != nil {
		return nil, err
	}

	logger := log.New(stderr, "lumnd: ", log.LstdFlags)
	d := &Daemon{
		config:    config,
		store:     storeDB,
		logger:    logger,
		startTime: time.Now().UTC(),
		workflows: map[string]*managedWorkflow{},
		waiters:   map[int64]chan executor.Report{},
	}
	d.webhooks = newWebhookRegistry(d)
	return d, nil
}

func (d *Daemon) Close() error {
	if d == nil {
		return nil
	}
	if d.internalServer != nil {
		_ = d.internalServer.Close()
	}
	if d.webhookServer != nil {
		_ = d.webhookServer.Close()
	}
	if d.internalListener != nil {
		_ = d.internalListener.Close()
	}
	if d.webhookListener != nil {
		_ = d.webhookListener.Close()
	}
	_ = cleanupInternal(d.config.Paths)
	_ = os.Remove(d.config.Paths.PIDPath)
	return d.store.Close()
}

func (d *Daemon) Run() error {
	if err := d.config.Paths.EnsureStateDir(); err != nil {
		return err
	}
	if err := cleanupInternal(d.config.Paths); err != nil {
		return err
	}

	internalListener, err := listenInternal(d.config.Paths)
	if err != nil {
		return err
	}
	d.internalListener = internalListener

	webhookListener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", d.config.WebhookPort))
	if err != nil {
		_ = internalListener.Close()
		return err
	}
	d.webhookListener = webhookListener

	if err := os.WriteFile(d.config.Paths.PIDPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644); err != nil {
		return err
	}

	if err := d.store.MarkStaleRunningExecutions("daemon restarted while execution was running"); err != nil {
		return err
	}
	if err := d.store.ApplyRetention(d.config.RetentionMaxRuns, d.config.RetentionMaxDays); err != nil {
		return err
	}

	d.internalServer = &http.Server{Handler: d.internalHandler()}
	d.webhookServer = &http.Server{Handler: d.webhooks}

	go func() {
		if err := d.internalServer.Serve(internalListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			d.logger.Printf("internal server stopped with error: %v", err)
		}
	}()
	go func() {
		if err := d.webhookServer.Serve(webhookListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			d.logger.Printf("webhook server stopped with error: %v", err)
		}
	}()

	if err := d.restoreActiveWorkflows(); err != nil {
		return err
	}

	go d.retentionLoop()
	return nil
}

func (d *Daemon) Shutdown(timeout time.Duration) error {
	d.mu.Lock()
	if d.shuttingDown {
		d.mu.Unlock()
		return errors.New("shutdown already in progress")
	}
	d.shuttingDown = true
	workflows := make([]*managedWorkflow, 0, len(d.workflows))
	for _, workflow := range d.workflows {
		workflows = append(workflows, workflow)
	}
	d.mu.Unlock()

	for _, workflow := range workflows {
		workflow.setAccepting(false)
	}

	deadline := time.Now().Add(timeout)
	for _, workflow := range workflows {
		if err := workflow.waitIdle(time.Until(deadline)); err != nil {
			for _, item := range workflows {
				item.setAccepting(true)
				item.notify()
			}
			d.mu.Lock()
			d.shuttingDown = false
			d.mu.Unlock()
			return err
		}
	}

	for _, workflow := range workflows {
		if err := d.finalizeWorkflowStop(workflow, store.StatusStopped, "daemon stopped before queued execution ran"); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if d.internalServer != nil {
		_ = d.internalServer.Shutdown(ctx)
	}
	if d.webhookServer != nil {
		_ = d.webhookServer.Shutdown(ctx)
	}
	if d.internalListener != nil {
		_ = d.internalListener.Close()
	}
	if d.webhookListener != nil {
		_ = d.webhookListener.Close()
	}
	_ = cleanupInternal(d.config.Paths)
	_ = os.Remove(d.config.Paths.PIDPath)
	return nil
}

func (d *Daemon) restoreActiveWorkflows() error {
	workflows, err := d.store.ListActiveWorkflows()
	if err != nil {
		return err
	}
	for _, workflow := range workflows {
		triggers, err := d.store.ListTriggers(workflow.ID)
		if err != nil {
			return err
		}
		if err := d.activateWorkflow(workflow, triggers, true); err != nil {
			d.logger.Printf("restore workflow %s failed: %v", workflow.ID, err)
			_ = d.store.SetWorkflowStatus(workflow.ID, store.StatusStopped)
			for _, trigger := range triggers {
				_ = d.store.SetTriggerRuntime(trigger.ID, store.TriggerStopped, nil)
			}
		}
	}
	return nil
}

func (d *Daemon) retentionLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		if err := d.store.ApplyRetention(d.config.RetentionMaxRuns, d.config.RetentionMaxDays); err != nil {
			d.logger.Printf("retention cleanup failed: %v", err)
		}
	}
}

func (d *Daemon) StartWorkflow(name, version, targetPath string) (store.Workflow, error) {
	definition, target, err := engine.LoadDefinition(targetPath, d.logger.Writer())
	if err != nil {
		return store.Workflow{}, err
	}
	if name == "" {
		name = target.Name
	}
	if version == "" {
		version = "latest"
	}

	resolvedTarget := target.TargetPath
	triggers, err := validateTriggers(definition.Triggers)
	if err != nil {
		return store.Workflow{}, err
	}

	existing, found, err := d.store.GetWorkflowByNameVersion(name, version)
	if err != nil {
		return store.Workflow{}, err
	}
	if found && existing.Status == store.StatusActive {
		managed, err := d.getManagedWorkflowByID(existing.ID)
		if err == nil {
			managed.setAccepting(false)
			if err := managed.waitIdle(d.config.ShutdownTimeout); err != nil {
				managed.setAccepting(true)
				managed.notify()
				return store.Workflow{}, err
			}
			if err := d.finalizeWorkflowStop(managed, store.StatusStopped, "workflow restarted before queued execution ran"); err != nil {
				return store.Workflow{}, err
			}
		} else {
			if err := d.store.SetWorkflowStatus(existing.ID, store.StatusStopped); err != nil {
				return store.Workflow{}, err
			}
		}
	}

	workflow := store.Workflow{
		ID:      workflowIDFor(name, version),
		Name:    name,
		Version: version,
		Path:    resolvedTarget,
		Status:  store.StatusStopped,
	}
	if found {
		workflow.ID = existing.ID
		workflow.CreatedAt = existing.CreatedAt
	}
	if err := d.store.UpsertWorkflow(workflow); err != nil {
		return store.Workflow{}, err
	}

	storeTriggers := make([]store.Trigger, 0, len(triggers))
	for _, trigger := range triggers {
		storeTriggers = append(storeTriggers, store.Trigger{
			Type:   trigger.Type,
			Config: trigger.Config,
			Status: store.TriggerStopped,
		})
	}
	insertedTriggers, err := d.store.ReplaceWorkflowTriggers(workflow.ID, storeTriggers)
	if err != nil {
		if !found {
			_ = d.store.DeleteWorkflow(workflow.ID)
		}
		return store.Workflow{}, err
	}

	if err := d.activateWorkflow(workflow, insertedTriggers, false); err != nil {
		if !found {
			_ = d.store.DeleteWorkflow(workflow.ID)
		}
		return store.Workflow{}, err
	}

	activeWorkflow, _, err := d.store.GetWorkflow(workflow.ID)
	if err != nil {
		return workflow, err
	}
	return activeWorkflow, nil
}

func (d *Daemon) StopWorkflow(selector string) error {
	workflow, err := d.resolveWorkflow(selector)
	if err != nil {
		return err
	}
	if workflow.Status != store.StatusActive {
		return nil
	}

	managed, err := d.getManagedWorkflowByID(workflow.ID)
	if err != nil {
		return err
	}

	managed.setAccepting(false)
	if err := managed.waitIdle(d.config.ShutdownTimeout); err != nil {
		managed.setAccepting(true)
		managed.notify()
		return err
	}

	return d.finalizeWorkflowStop(managed, store.StatusStopped, "workflow stopped before queued execution ran")
}

func (d *Daemon) DeleteWorkflow(selector string) error {
	workflow, err := d.resolveWorkflow(selector)
	if err != nil {
		return err
	}

	if workflow.Status == store.StatusActive {
		managed, err := d.getManagedWorkflowByID(workflow.ID)
		if err != nil {
			return err
		}

		managed.setAccepting(false)
		if err := managed.waitIdle(d.config.ShutdownTimeout); err != nil {
			managed.setAccepting(true)
			managed.notify()
			return err
		}

		if err := d.finalizeWorkflowStop(managed, store.StatusStopped, "workflow deleted before queued execution ran"); err != nil {
			return err
		}
	}

	return d.store.DeleteWorkflow(workflow.ID)
}

func (d *Daemon) RestartWorkflow(selector string) error {
	workflow, err := d.resolveWorkflow(selector)
	if err != nil {
		return err
	}

	if workflow.Status == store.StatusActive {
		managed, err := d.getManagedWorkflowByID(workflow.ID)
		if err != nil {
			return err
		}

		managed.setAccepting(false)
		if err := managed.waitIdle(d.config.ShutdownTimeout); err != nil {
			managed.setAccepting(true)
			managed.notify()
			return err
		}

		if err := d.finalizeWorkflowStop(managed, store.StatusStopped, "workflow restarted before queued execution ran"); err != nil {
			return err
		}
	}

	if _, err := d.StartWorkflow(workflow.Name, workflow.Version, workflow.Path); err != nil {
		return err
	}
	return nil
}

func (d *Daemon) ListWorkflowResponses() ([]workflowBundle, error) {
	workflows, err := d.store.ListWorkflows()
	if err != nil {
		return nil, err
	}

	bundles := make([]workflowBundle, 0, len(workflows))
	for _, workflow := range workflows {
		triggers, err := d.store.ListTriggers(workflow.ID)
		if err != nil {
			return nil, err
		}
		lastExecution, found, err := d.store.LatestExecution(workflow.ID)
		if err != nil {
			return nil, err
		}
		var latest *store.Execution
		if found {
			latest = &lastExecution
		}
		fails, err := d.store.CountFailedExecutionsSince(workflow.ID, workflow.UpdatedAt)
		if err != nil {
			return nil, err
		}
		bundles = append(bundles, workflowBundle{
			Workflow: workflow,
			Triggers: triggers,
			Latest:   latest,
			Fails:    fails,
		})
	}
	return bundles, nil
}

func (d *Daemon) WorkflowDetails(selector string) (workflowBundle, error) {
	workflow, err := d.resolveWorkflow(selector)
	if err != nil {
		return workflowBundle{}, err
	}
	triggers, err := d.store.ListTriggers(workflow.ID)
	if err != nil {
		return workflowBundle{}, err
	}
	lastExecution, found, err := d.store.LatestExecution(workflow.ID)
	if err != nil {
		return workflowBundle{}, err
	}
	fails, err := d.store.CountFailedExecutionsSince(workflow.ID, workflow.UpdatedAt)
	if err != nil {
		return workflowBundle{}, err
	}
	bundle := workflowBundle{Workflow: workflow, Triggers: triggers, Fails: fails}
	if found {
		bundle.Latest = &lastExecution
	}
	return bundle, nil
}

func (d *Daemon) ExecWorkflow(selector string) (int64, executor.Report, error) {
	workflow, err := d.resolveWorkflow(selector)
	if err != nil {
		return 0, executor.Report{}, err
	}
	managed, err := d.getManagedWorkflowByID(workflow.ID)
	if err != nil {
		return 0, executor.Report{}, err
	}
	if !managed.manual {
		return 0, executor.Report{}, fmt.Errorf("workflow %q does not accept manual execution", workflow.Name)
	}

	execution, err := d.enqueueExecution(workflow.ID, "manual", map[string]any{"type": "manual"})
	if err != nil {
		return 0, executor.Report{}, err
	}

	waiter := make(chan executor.Report, 1)
	d.mu.Lock()
	d.waiters[execution.ID] = waiter
	d.mu.Unlock()
	defer d.deleteWaiter(execution.ID)

	managed.notify()

	select {
	case report := <-waiter:
		return execution.ID, report, nil
	case <-time.After(d.config.ShutdownTimeout):
		return execution.ID, executor.Report{}, fmt.Errorf("execution %d timed out waiting for completion", execution.ID)
	}
}

func (d *Daemon) enqueueExecution(workflowID, triggerType string, triggerContext map[string]any) (store.Execution, error) {
	managed, err := d.getManagedWorkflow(workflowID)
	if err != nil {
		return store.Execution{}, err
	}
	if !managed.isAccepting() {
		return store.Execution{}, fmt.Errorf("workflow %q is stopping", workflowID)
	}

	count, err := d.store.QueueCount(workflowID)
	if err != nil {
		return store.Execution{}, err
	}
	if count >= d.config.QueueLimit {
		return store.Execution{}, fmt.Errorf("workflow %q queue is full", workflowID)
	}

	execution, err := d.store.CreateQueuedExecution(workflowID, triggerType, triggerContext)
	if err != nil {
		return store.Execution{}, err
	}
	managed.notify()
	return execution, nil
}

func (d *Daemon) enqueueAsync(workflowID, triggerType string, triggerContext map[string]any) error {
	execution, err := d.enqueueExecution(workflowID, triggerType, triggerContext)
	if err != nil {
		d.logger.Printf("enqueue %s for %s failed: %v", triggerType, workflowID, err)
		return err
	}
	d.logger.Printf("queued execution %d for workflow %s via %s", execution.ID, workflowID, triggerType)
	return nil
}

func (d *Daemon) activateWorkflow(workflow store.Workflow, triggers []store.Trigger, restoring bool) error {
	managed := &managedWorkflow{
		workflow:  workflow,
		triggers:  triggers,
		notifyCh:  make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		accepting: true,
	}

	for idx := range triggers {
		trigger := triggers[idx]
		handle, nextRun, err := d.startTrigger(managed, trigger, restoring)
		if err != nil {
			for _, existing := range managed.handles {
				_ = existing.Stop()
			}
			return err
		}
		if handle != nil {
			managed.handles = append(managed.handles, handle)
		}
		if trigger.Type == "manual" {
			managed.manual = true
		}
		status := store.TriggerActive
		if err := d.store.SetTriggerRuntime(trigger.ID, status, nextRun); err != nil {
			return err
		}
		triggers[idx].Status = status
		triggers[idx].NextRunAt = nextRun
	}

	managed.triggers = triggers
	go d.workerLoop(managed)

	d.mu.Lock()
	d.workflows[workflow.ID] = managed
	d.mu.Unlock()

	if err := d.store.UpsertWorkflow(store.Workflow{
		ID:        workflow.ID,
		Name:      workflow.Name,
		Version:   workflow.Version,
		Path:      workflow.Path,
		Status:    store.StatusActive,
		CreatedAt: workflow.CreatedAt,
		UpdatedAt: workflow.UpdatedAt,
	}); err != nil {
		return err
	}

	managed.notify()
	return nil
}

func (d *Daemon) finalizeWorkflowStop(managed *managedWorkflow, status string, cancelMessage string) error {
	canceled, err := d.store.CancelQueuedExecutions(managed.workflow.ID, cancelMessage)
	if err != nil {
		return err
	}
	for _, executionID := range canceled {
		execution, found, err := d.store.GetExecution(executionID)
		if err == nil && found && execution.Report != nil {
			d.publishWaiter(executionID, *execution.Report)
		}
	}

	for _, handle := range managed.handles {
		if err := handle.Stop(); err != nil {
			d.logger.Printf("stop trigger for %s failed: %v", managed.workflow.ID, err)
		}
	}
	close(managed.stopCh)

	for _, trigger := range managed.triggers {
		if err := d.store.SetTriggerRuntime(trigger.ID, store.TriggerStopped, nil); err != nil {
			return err
		}
	}
	if err := d.store.SetWorkflowStatus(managed.workflow.ID, status); err != nil {
		return err
	}

	d.webhooks.removeWorkflow(managed.workflow.ID)

	d.mu.Lock()
	delete(d.workflows, managed.workflow.ID)
	d.mu.Unlock()
	return nil
}

func (d *Daemon) startTrigger(managed *managedWorkflow, trigger store.Trigger, restoring bool) (triggerHandle, *time.Time, error) {
	switch trigger.Type {
	case "manual":
		return nil, nil, nil
	case "scheduler":
		return d.startSchedulerTrigger(managed, trigger, restoring)
	case "webhook":
		return d.startWebhookTrigger(managed, trigger)
	case "file_watcher":
		return d.startFileWatcherTrigger(managed, trigger)
	default:
		return nil, nil, fmt.Errorf("unsupported trigger type %q", trigger.Type)
	}
}

func (d *Daemon) startSchedulerTrigger(managed *managedWorkflow, trigger store.Trigger, restoring bool) (triggerHandle, *time.Time, error) {
	schedule, err := parseSchedulerConfig(trigger.Config)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().UTC()
	nextRun := trigger.NextRunAt
	if nextRun == nil {
		value := schedule.Next(now)
		nextRun = &value
	}
	if restoring && nextRun.Before(now) {
		_ = d.enqueueAsync(managed.workflow.ID, "scheduler", map[string]any{
			"type":         "scheduler",
			"scheduled_at": nextRun.Format(time.RFC3339Nano),
			"fired_at":     now.Format(time.RFC3339Nano),
		})
		value := schedule.Next(now)
		nextRun = &value
	}

	stopCh := make(chan struct{})
	go func(current time.Time) {
		next := current
		for {
			delay := time.Until(next)
			if delay < 0 {
				delay = 0
			}
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
				firedAt := time.Now().UTC()
				_ = d.enqueueAsync(managed.workflow.ID, "scheduler", map[string]any{
					"type":         "scheduler",
					"scheduled_at": next.Format(time.RFC3339Nano),
					"fired_at":     firedAt.Format(time.RFC3339Nano),
				})
				next = schedule.Next(firedAt)
				_ = d.store.SetTriggerRuntime(trigger.ID, store.TriggerActive, &next)
			case <-stopCh:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			}
		}
	}(*nextRun)

	return stopFunc(func() error {
		close(stopCh)
		return nil
	}), nextRun, nil
}

func (d *Daemon) startWebhookTrigger(managed *managedWorkflow, trigger store.Trigger) (triggerHandle, *time.Time, error) {
	cfg, err := parseWebhookConfig(trigger.Config)
	if err != nil {
		return nil, nil, err
	}
	if err := d.webhooks.register(managed.workflow.ID, cfg.Method, cfg.Path); err != nil {
		return nil, nil, err
	}

	return stopFunc(func() error {
		d.webhooks.unregister(managed.workflow.ID, cfg.Method, cfg.Path)
		return nil
	}), nil, nil
}

func (d *Daemon) startFileWatcherTrigger(managed *managedWorkflow, trigger store.Trigger) (triggerHandle, *time.Time, error) {
	cfg, err := parseFileWatcherConfig(trigger.Config)
	if err != nil {
		return nil, nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, err
	}
	if err := watcher.Add(cfg.Path); err != nil {
		_ = watcher.Close()
		return nil, nil, err
	}

	stopCh := make(chan struct{})
	var timersMu sync.Mutex
	timers := map[string]*time.Timer{}
	eventKinds := map[string]string{}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				eventKind, matched := cfg.Match(event)
				if !matched {
					continue
				}
				filePath := event.Name
				timersMu.Lock()
				eventKinds[filePath] = eventKind
				if timer, ok := timers[filePath]; ok {
					timer.Reset(cfg.Debounce)
				} else {
					timer := time.AfterFunc(cfg.Debounce, func() {
						timersMu.Lock()
						delete(timers, filePath)
						eventKind := eventKinds[filePath]
						delete(eventKinds, filePath)
						timersMu.Unlock()

						_ = d.enqueueAsync(managed.workflow.ID, "file_watcher", map[string]any{
							"type":  "file_watcher",
							"file":  filepath.Base(filePath),
							"event": eventKind,
							"path":  cfg.Path,
						})
					})
					timers[filePath] = timer
				}
				timersMu.Unlock()
			case err := <-watcher.Errors:
				if err != nil {
					d.logger.Printf("file watcher for %s failed: %v", managed.workflow.ID, err)
				}
			case <-stopCh:
				timersMu.Lock()
				for _, timer := range timers {
					timer.Stop()
				}
				timersMu.Unlock()
				return
			}
		}
	}()

	return stopFunc(func() error {
		close(stopCh)
		return nil
	}), nil, nil
}

func (d *Daemon) workerLoop(managed *managedWorkflow) {
	for {
		select {
		case <-managed.notifyCh:
		case <-managed.stopCh:
			return
		}

		for managed.isAccepting() {
			item, found, err := d.store.ClaimNextQueueItem(managed.workflow.ID)
			if err != nil {
				d.logger.Printf("claim queue item for %s failed: %v", managed.workflow.ID, err)
				break
			}
			if !found {
				break
			}

			managed.setRunning(true)
			report, _ := engine.RunTargetWithOptions(managed.workflow.Path, d.logger.Writer(), engine.RunOptions{
				WorkflowName: managed.workflow.Name,
				Version:      managed.workflow.Version,
				TriggerData:  item.TriggerContext,
			})
			if err := d.store.CompleteExecution(item.ExecutionID, report.Status, report); err != nil {
				d.logger.Printf("complete execution %d failed: %v", item.ExecutionID, err)
			}
			d.publishWaiter(item.ExecutionID, report)
			managed.setRunning(false)
		}
	}
}

func (d *Daemon) getManagedWorkflow(selector string) (*managedWorkflow, error) {
	workflow, err := d.resolveWorkflow(selector)
	if err != nil {
		return nil, err
	}
	return d.getManagedWorkflowByID(workflow.ID)
}

func (d *Daemon) getManagedWorkflowByID(workflowID string) (*managedWorkflow, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	managed, ok := d.workflows[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow %q is not active", workflowID)
	}
	return managed, nil
}

func (d *Daemon) publishWaiter(executionID int64, report executor.Report) {
	d.mu.Lock()
	waiter, ok := d.waiters[executionID]
	if ok {
		delete(d.waiters, executionID)
	}
	d.mu.Unlock()
	if ok {
		waiter <- report
		close(waiter)
	}
}

func (d *Daemon) deleteWaiter(executionID int64) {
	d.mu.Lock()
	delete(d.waiters, executionID)
	d.mu.Unlock()
}

type workflowBundle struct {
	Workflow store.Workflow
	Triggers []store.Trigger
	Latest   *store.Execution
	Fails    int
}

func (w *managedWorkflow) notify() {
	select {
	case w.notifyCh <- struct{}{}:
	default:
	}
}

func (w *managedWorkflow) setAccepting(value bool) {
	w.mu.Lock()
	w.accepting = value
	w.mu.Unlock()
}

func (w *managedWorkflow) isAccepting() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.accepting
}

func (w *managedWorkflow) setRunning(value bool) {
	w.mu.Lock()
	w.running = value
	w.mu.Unlock()
}

func (w *managedWorkflow) waitIdle(timeout time.Duration) error {
	if timeout <= 0 {
		timeout = time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		w.mu.Lock()
		running := w.running
		w.mu.Unlock()
		if !running {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("workflow %q did not finish execution before timeout", w.workflow.ID)
}

func (d *Daemon) resolveWorkflow(selector string) (store.Workflow, error) {
	workflow, found, err := d.store.ResolveWorkflowSelector(selector)
	if err != nil {
		return store.Workflow{}, err
	}
	if !found {
		return store.Workflow{}, fmt.Errorf("workflow %q not found", selector)
	}
	return workflow, nil
}

func workflowIDFor(name, version string) string {
	sum := sha1.Sum([]byte(name + ":" + version))
	return hex.EncodeToString(sum[:])[:8]
}

type schedulerPlan struct {
	next func(time.Time) time.Time
}

func (s schedulerPlan) Next(after time.Time) time.Time {
	return s.next(after.UTC())
}

func parseSchedulerConfig(config map[string]any) (schedulerPlan, error) {
	interval, _ := config["interval"].(string)
	cronExpr, _ := config["cron"].(string)
	timezone, _ := config["timezone"].(string)
	if (interval == "" && cronExpr == "") || (interval != "" && cronExpr != "") {
		return schedulerPlan{}, errors.New("scheduler trigger requires exactly one of interval or cron")
	}

	if interval != "" {
		duration, err := parseStrictDuration(interval, false)
		if err != nil {
			return schedulerPlan{}, err
		}
		return schedulerPlan{
			next: func(after time.Time) time.Time {
				return after.Add(duration)
			},
		}, nil
	}

	location := time.UTC
	if timezone != "" {
		loaded, err := time.LoadLocation(timezone)
		if err != nil {
			return schedulerPlan{}, fmt.Errorf("invalid scheduler timezone %q", timezone)
		}
		location = loaded
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		return schedulerPlan{}, fmt.Errorf("invalid scheduler cron: %w", err)
	}
	return schedulerPlan{
		next: func(after time.Time) time.Time {
			return schedule.Next(after.In(location)).UTC()
		},
	}, nil
}

type webhookConfig struct {
	Path   string
	Method string
}

func parseWebhookConfig(config map[string]any) (webhookConfig, error) {
	path, _ := config["path"].(string)
	if path == "" || !strings.HasPrefix(path, "/hooks/") {
		return webhookConfig{}, errors.New(`webhook trigger path must start with "/hooks/"`)
	}
	method, _ := config["method"].(string)
	if method == "" {
		method = http.MethodPost
	}
	return webhookConfig{
		Path:   path,
		Method: strings.ToUpper(method),
	}, nil
}

type fileWatcherConfig struct {
	Path     string
	Pattern  string
	Event    string
	Debounce time.Duration
}

func parseFileWatcherConfig(config map[string]any) (fileWatcherConfig, error) {
	path, _ := config["path"].(string)
	if path == "" || !filepath.IsAbs(path) {
		return fileWatcherConfig{}, errors.New("file_watcher trigger path must be an absolute path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fileWatcherConfig{}, fmt.Errorf("file_watcher path %q does not exist", path)
	}
	if !info.IsDir() {
		return fileWatcherConfig{}, fmt.Errorf("file_watcher path %q must be a directory", path)
	}
	pattern, _ := config["pattern"].(string)
	event, _ := config["event"].(string)
	if event == "" {
		event = "any"
	}
	switch event {
	case "create", "modify", "delete", "any":
	default:
		return fileWatcherConfig{}, fmt.Errorf("unsupported file_watcher event %q", event)
	}

	debounceValue := "500ms"
	if raw, ok := config["debounce"].(string); ok && raw != "" {
		debounceValue = raw
	}
	debounce, err := parseStrictDuration(debounceValue, true)
	if err != nil {
		return fileWatcherConfig{}, err
	}

	return fileWatcherConfig{
		Path:     path,
		Pattern:  pattern,
		Event:    event,
		Debounce: debounce,
	}, nil
}

func (c fileWatcherConfig) Match(event fsnotify.Event) (string, bool) {
	if c.Pattern != "" {
		matched, err := filepath.Match(c.Pattern, filepath.Base(event.Name))
		if err != nil || !matched {
			return "", false
		}
	}

	switch {
	case event.Has(fsnotify.Create):
		return c.matchEvent("create")
	case event.Has(fsnotify.Write):
		return c.matchEvent("modify")
	case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
		return c.matchEvent("delete")
	default:
		return "", false
	}
}

func (c fileWatcherConfig) matchEvent(event string) (string, bool) {
	if c.Event == "any" || c.Event == event {
		return event, true
	}
	return "", false
}

func parseStrictDuration(raw string, allowMilliseconds bool) (time.Duration, error) {
	if raw == "" {
		return 0, errors.New("duration cannot be empty")
	}
	if strings.HasSuffix(raw, "ms") {
		if !allowMilliseconds {
			return 0, fmt.Errorf("duration %q does not support milliseconds", raw)
		}
	} else if !(strings.HasSuffix(raw, "s") || strings.HasSuffix(raw, "m") || strings.HasSuffix(raw, "h")) {
		return 0, fmt.Errorf("unsupported duration %q", raw)
	}

	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", raw)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("duration %q must be positive", raw)
	}
	return duration, nil
}

type validatedTrigger struct {
	Type   string
	Config map[string]any
}

func validateTriggers(triggers []engine.TriggerSpec) ([]validatedTrigger, error) {
	if len(triggers) == 0 {
		return []validatedTrigger{{Type: "manual", Config: map[string]any{}}}, nil
	}

	validated := make([]validatedTrigger, 0, len(triggers))
	for _, trigger := range triggers {
		switch trigger.Type {
		case "manual":
			validated = append(validated, validatedTrigger{Type: trigger.Type, Config: map[string]any{}})
		case "scheduler":
			if _, err := parseSchedulerConfig(trigger.Config); err != nil {
				return nil, err
			}
			validated = append(validated, validatedTrigger{Type: trigger.Type, Config: trigger.Config})
		case "webhook":
			if _, err := parseWebhookConfig(trigger.Config); err != nil {
				return nil, err
			}
			validated = append(validated, validatedTrigger{Type: trigger.Type, Config: trigger.Config})
		case "file_watcher":
			if _, err := parseFileWatcherConfig(trigger.Config); err != nil {
				return nil, err
			}
			validated = append(validated, validatedTrigger{Type: trigger.Type, Config: trigger.Config})
		default:
			return nil, fmt.Errorf("unsupported trigger type %q", trigger.Type)
		}
	}
	return validated, nil
}

type webhookRegistry struct {
	daemon *Daemon
	mu     sync.RWMutex
	routes map[string]webhookRoute
}

type webhookRoute struct {
	WorkflowID string
	Method     string
	Path       string
}

func newWebhookRegistry(daemon *Daemon) *webhookRegistry {
	return &webhookRegistry{
		daemon: daemon,
		routes: map[string]webhookRoute{},
	}
}

func (r *webhookRegistry) key(method, path string) string {
	return method + " " + path
}

func (r *webhookRegistry) register(workflowID, method, path string) error {
	key := r.key(method, path)
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.routes[key]; ok && existing.WorkflowID != workflowID {
		return fmt.Errorf("webhook path %s %s is already registered", method, path)
	}
	r.routes[key] = webhookRoute{
		WorkflowID: workflowID,
		Method:     method,
		Path:       path,
	}
	return nil
}

func (r *webhookRegistry) unregister(workflowID, method, path string) {
	key := r.key(method, path)
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.routes[key]; ok && existing.WorkflowID == workflowID {
		delete(r.routes, key)
	}
}

func (r *webhookRegistry) removeWorkflow(workflowID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, route := range r.routes {
		if route.WorkflowID == workflowID {
			delete(r.routes, key)
		}
	}
}

func (r *webhookRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	route, ok := r.routes[r.key(req.Method, req.URL.Path)]
	r.mu.RUnlock()
	if !ok {
		http.NotFound(w, req)
		return
	}

	var body any
	if req.Body != nil {
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "webhook body must be valid JSON"})
			return
		}
	}
	if body == nil {
		body = map[string]any{}
	}

	headers := map[string]any{}
	for key, values := range req.Header {
		if len(values) == 1 {
			headers[strings.ToLower(key)] = values[0]
			continue
		}
		items := make([]any, 0, len(values))
		for _, value := range values {
			items = append(items, value)
		}
		headers[strings.ToLower(key)] = items
	}

	execution, err := r.daemon.enqueueExecution(route.WorkflowID, "webhook", map[string]any{
		"type":    "webhook",
		"body":    body,
		"headers": headers,
		"method":  req.Method,
		"path":    req.URL.Path,
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{"execution_id": execution.ID})
}
