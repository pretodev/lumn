package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pretodev/lumn/internal/daemonapi"
	"github.com/pretodev/lumn/internal/store"
)

func (d *Daemon) internalHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", d.handleHealth)
	mux.HandleFunc("/api/v1/workflows", d.handleWorkflows)
	mux.HandleFunc("/api/v1/workflows/", d.handleWorkflowByID)
	mux.HandleFunc("/api/v1/daemon/shutdown", d.handleShutdown)
	return mux
}

func (d *Daemon) handleHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	activeCount, err := d.store.CountActiveWorkflows()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, daemonapi.HealthResponse{
		Running:         true,
		Transport:       d.config.Paths.TransportDescription(),
		WebhookPort:     d.config.WebhookPort,
		ActiveWorkflows: activeCount,
		PID:             osGetpid(),
		StartedAt:       d.startTime.Format(time.RFC3339Nano),
		UptimeSeconds:   int64(time.Since(d.startTime).Seconds()),
	})
}

func (d *Daemon) handleWorkflows(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		bundles, err := d.ListWorkflowResponses()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		resp := daemonapi.WorkflowsListResponse{Workflows: make([]daemonapi.WorkflowResponse, 0, len(bundles))}
		for _, bundle := range bundles {
			resp.Workflows = append(resp.Workflows, summarizeWorkflow(bundle))
		}
		writeJSON(w, http.StatusOK, resp)
	case http.MethodPost:
		var payload daemonapi.StartWorkflowRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		workflow, err := d.StartWorkflow(payload.Name, payload.Version, payload.Target)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, daemonapi.WorkflowMutationResponse{
			WorkflowID: workflow.ID,
			Message:    fmt.Sprintf("workflow %s started", workflow.ID),
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (d *Daemon) handleWorkflowByID(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/api/v1/workflows/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	workflowID := parts[0]

	if len(parts) == 1 {
		if req.Method == http.MethodDelete {
			if err := d.DeleteWorkflow(workflowID); err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, daemonapi.WorkflowMutationResponse{
				WorkflowID: workflowID,
				Message:    fmt.Sprintf("workflow %s deleted", workflowID),
			})
			return
		}
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	switch parts[1] {
	case "stop":
		if req.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := d.StopWorkflow(workflowID); err != nil {
			writeWorkflowError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, daemonapi.WorkflowMutationResponse{
			WorkflowID: workflowID,
			Message:    fmt.Sprintf("workflow %s stopped", workflowID),
		})
	case "restart":
		if req.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := d.RestartWorkflow(workflowID); err != nil {
			writeWorkflowError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, daemonapi.WorkflowMutationResponse{
			WorkflowID: workflowID,
			Message:    fmt.Sprintf("workflow %s restarted", workflowID),
		})
	case "exec":
		if req.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		executionID, report, err := d.ExecWorkflow(workflowID)
		if err != nil {
			writeWorkflowError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, daemonapi.ExecWorkflowResponse{
			ExecutionID: executionID,
			Report:      report,
		})
	case "status":
		if req.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		bundle, err := d.WorkflowDetails(workflowID)
		if err != nil {
			writeWorkflowError(w, err)
			return
		}
		resp := daemonapi.WorkflowDetailResponse{
			Workflow: summarizeWorkflow(bundle),
			Triggers: summarizeTriggers(bundle.Triggers),
		}
		writeJSON(w, http.StatusOK, resp)
	default:
		writeError(w, http.StatusNotFound, "workflow endpoint not found")
	}
}

func (d *Daemon) handleShutdown(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := d.Shutdown(d.config.ShutdownTimeout); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, daemonapi.WorkflowMutationResponse{
		Message: "daemon stopped",
	})
}

func summarizeWorkflow(bundle workflowBundle) daemonapi.WorkflowResponse {
	triggers := make([]string, 0, len(bundle.Triggers))
	var nextRun string
	for _, trigger := range bundle.Triggers {
		triggers = append(triggers, trigger.Type)
		if trigger.NextRunAt != nil {
			candidate := trigger.NextRunAt.Format(time.RFC3339Nano)
			if nextRun == "" || candidate < nextRun {
				nextRun = candidate
			}
		}
	}
	sort.Strings(triggers)

	response := daemonapi.WorkflowResponse{
		ID:       bundle.Workflow.ID,
		Name:     bundle.Workflow.Name,
		Version:  bundle.Workflow.Version,
		Path:     bundle.Workflow.Path,
		Status:   bundle.Workflow.Status,
		Triggers: triggers,
		Fails:    bundle.Fails,
		NextRun:  nextRun,
	}
	if bundle.Latest != nil {
		response.LastStatus = bundle.Latest.Status
		if bundle.Latest.FinishedAt != nil {
			response.LastRun = bundle.Latest.FinishedAt.Format(time.RFC3339Nano)
		} else if bundle.Latest.StartedAt != nil {
			response.LastRun = bundle.Latest.StartedAt.Format(time.RFC3339Nano)
		} else {
			response.LastRun = bundle.Latest.QueuedAt.Format(time.RFC3339Nano)
		}
	}
	return response
}

func summarizeTriggers(triggers []store.Trigger) []daemonapi.TriggerResponse {
	resp := make([]daemonapi.TriggerResponse, 0, len(triggers))
	for _, trigger := range triggers {
		item := daemonapi.TriggerResponse{
			Type:   trigger.Type,
			Status: trigger.Status,
			Config: trigger.Config,
		}
		if trigger.NextRunAt != nil {
			item.NextRun = trigger.NextRunAt.Format(time.RFC3339Nano)
		}
		resp = append(resp, item)
	}
	return resp
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, daemonapi.ErrorResponse{Error: message})
}

func writeWorkflowError(w http.ResponseWriter, err error) {
	if strings.Contains(err.Error(), "not found") {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func osGetpid() int {
	return os.Getpid()
}
