package daemonapi

import "github.com/pretodev/lumn/internal/executor"

type ErrorResponse struct {
	Error string `json:"error"`
}

type HealthResponse struct {
	Running         bool   `json:"running"`
	Transport       string `json:"transport"`
	WebhookPort     int    `json:"webhook_port"`
	ActiveWorkflows int    `json:"active_workflows"`
	PID             int    `json:"pid"`
	StartedAt       string `json:"started_at"`
	UptimeSeconds   int64  `json:"uptime_seconds"`
}

type StartWorkflowRequest struct {
	Path string `json:"path"`
}

type WorkflowResponse struct {
	ID         string   `json:"id"`
	Version    string   `json:"version"`
	Path       string   `json:"path"`
	Status     string   `json:"status"`
	Triggers   []string `json:"triggers"`
	NextRun    string   `json:"next_run,omitempty"`
	LastRun    string   `json:"last_run,omitempty"`
	LastStatus string   `json:"last_status,omitempty"`
}

type TriggerResponse struct {
	Type    string         `json:"type"`
	Status  string         `json:"status"`
	NextRun string         `json:"next_run,omitempty"`
	Config  map[string]any `json:"config"`
}

type WorkflowsListResponse struct {
	Workflows []WorkflowResponse `json:"workflows"`
}

type WorkflowDetailResponse struct {
	Workflow WorkflowResponse  `json:"workflow"`
	Triggers []TriggerResponse `json:"triggers"`
}

type WorkflowMutationResponse struct {
	WorkflowID string `json:"workflow_id"`
	Message    string `json:"message"`
}

type AsyncAcceptedResponse struct {
	ExecutionID int64 `json:"execution_id"`
}

type ExecWorkflowResponse struct {
	ExecutionID int64           `json:"execution_id"`
	Report      executor.Report `json:"report"`
}
