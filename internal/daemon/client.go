package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pretodev/lumn/internal/daemonapi"
)

var ErrDaemonNotRunning = errors.New("daemon is not running")

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func IsDaemonNotRunning(err error) bool {
	return errors.Is(err, ErrDaemonNotRunning)
}

type Client struct {
	paths   Paths
	http    *http.Client
	baseURL string
}

func NewClient(paths Paths) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialInternal(ctx, paths)
		},
	}

	return &Client{
		paths: paths,
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		baseURL: paths.InternalURL(),
	}
}

func (c *Client) Health(ctx context.Context) (daemonapi.HealthResponse, error) {
	var out daemonapi.HealthResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/health", nil, &out)
	return out, err
}

func (c *Client) StartWorkflow(ctx context.Context, req daemonapi.StartWorkflowRequest) (daemonapi.WorkflowMutationResponse, error) {
	var out daemonapi.WorkflowMutationResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/workflows", req, &out)
	return out, err
}

func (c *Client) StopWorkflow(ctx context.Context, workflowID string) (daemonapi.WorkflowMutationResponse, error) {
	var out daemonapi.WorkflowMutationResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/workflows/"+url.PathEscape(workflowID)+"/stop", nil, &out)
	return out, err
}

func (c *Client) DeleteWorkflow(ctx context.Context, workflowID string) (daemonapi.WorkflowMutationResponse, error) {
	var out daemonapi.WorkflowMutationResponse
	err := c.doJSON(ctx, http.MethodDelete, "/api/v1/workflows/"+url.PathEscape(workflowID), nil, &out)
	return out, err
}

func (c *Client) RestartWorkflow(ctx context.Context, workflowID string) (daemonapi.WorkflowMutationResponse, error) {
	var out daemonapi.WorkflowMutationResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/workflows/"+url.PathEscape(workflowID)+"/restart", nil, &out)
	return out, err
}

func (c *Client) ListWorkflows(ctx context.Context) (daemonapi.WorkflowsListResponse, error) {
	var out daemonapi.WorkflowsListResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/workflows", nil, &out)
	return out, err
}

func (c *Client) WorkflowStatus(ctx context.Context, workflowID string) (daemonapi.WorkflowDetailResponse, error) {
	var out daemonapi.WorkflowDetailResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/workflows/"+url.PathEscape(workflowID)+"/status", nil, &out)
	return out, err
}

func (c *Client) ExecWorkflow(ctx context.Context, workflowID string) (daemonapi.ExecWorkflowResponse, error) {
	var out daemonapi.ExecWorkflowResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/workflows/"+url.PathEscape(workflowID)+"/exec", nil, &out)
	return out, err
}

func (c *Client) Shutdown(ctx context.Context) error {
	return c.doJSON(ctx, http.MethodPost, "/api/v1/daemon/shutdown", nil, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var payload *bytes.Reader
	if body == nil {
		payload = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return c.wrapTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var failure daemonapi.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&failure); err == nil && failure.Error != "" {
			return &APIError{
				StatusCode: resp.StatusCode,
				Message:    failure.Error,
			}
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("daemon request failed with status %s", resp.Status),
		}
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

func (c *Client) wrapTransportError(err error) error {
	var netErr *net.OpError
	if errors.As(err, &netErr) || strings.Contains(err.Error(), "connect: no such file") || strings.Contains(err.Error(), "connect: connection refused") {
		return fmt.Errorf("%w - start it with 'lumn daemon start'", ErrDaemonNotRunning)
	}
	return err
}
