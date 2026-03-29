package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pretodev/lumn/internal/daemonapi"
)

func TestDaemonHealthManualExecAndReactivate(t *testing.T) {
	t.Parallel()

	_, client, cfg := newTestDaemon(t)

	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if !health.Running || health.WebhookPort != cfg.WebhookPort {
		t.Fatalf("unexpected health: %+v", health)
	}

	workflowDir := filepath.Join(t.TempDir(), "manual")
	writeWorkflow(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.test_source({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
    filter {
      condition = function(item)
        return lumn.trigger_data().type == "manual"
      end,
    },
  }
}
`)

	startResp, err := client.StartWorkflow(context.Background(), daemonapi.StartWorkflowRequest{
		Target: workflowDir,
	})
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}

	execResp, err := client.ExecWorkflow(context.Background(), "manual")
	if err != nil {
		t.Fatalf("exec workflow: %v", err)
	}
	if execResp.Report.Status != "ok" || execResp.Report.ItemsOut != 1 {
		t.Fatalf("unexpected exec report: %+v", execResp.Report)
	}
	if execResp.Report.Workflow != "manual" || execResp.Report.Version != "latest" {
		t.Fatalf("unexpected exec metadata: %+v", execResp.Report)
	}

	if _, err := client.StopWorkflow(context.Background(), "manual"); err != nil {
		t.Fatalf("stop workflow: %v", err)
	}

	status, err := client.WorkflowStatus(context.Background(), "manual")
	if err != nil {
		t.Fatalf("workflow status after stop: %v", err)
	}
	if status.Workflow.Status != "stopped" {
		t.Fatalf("expected stopped status, got %+v", status.Workflow)
	}

	restartResp, err := client.StartWorkflow(context.Background(), daemonapi.StartWorkflowRequest{
		Target: workflowDir,
	})
	if err != nil {
		t.Fatalf("reactivate workflow: %v", err)
	}
	if restartResp.WorkflowID != startResp.WorkflowID {
		t.Fatalf("workflow id changed: %s != %s", restartResp.WorkflowID, startResp.WorkflowID)
	}
}

func TestDaemonStartReusesIDForSameNameVersion(t *testing.T) {
	t.Parallel()

	_, client, _ := newTestDaemon(t)

	workflowDir := filepath.Join(t.TempDir(), "versioned")
	writeWorkflow(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.test_source({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
  }
}
`)

	first, err := client.StartWorkflow(context.Background(), daemonapi.StartWorkflowRequest{
		Name:    "cancelamentos",
		Version: "1.2",
		Target:  workflowDir,
	})
	if err != nil {
		t.Fatalf("start first workflow: %v", err)
	}

	second, err := client.StartWorkflow(context.Background(), daemonapi.StartWorkflowRequest{
		Name:    "cancelamentos",
		Version: "1.2",
		Target:  workflowDir,
	})
	if err != nil {
		t.Fatalf("start replacement workflow: %v", err)
	}
	if first.WorkflowID != second.WorkflowID {
		t.Fatalf("workflow id changed: %s != %s", first.WorkflowID, second.WorkflowID)
	}

	status, err := client.WorkflowStatus(context.Background(), "cancelamentos")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Workflow.Name != "cancelamentos" || status.Workflow.Version != "1.2" {
		t.Fatalf("unexpected workflow status: %+v", status.Workflow)
	}
}

func TestDaemonDeleteRemovesWorkflow(t *testing.T) {
	t.Parallel()

	_, client, _ := newTestDaemon(t)

	workflowDir := filepath.Join(t.TempDir(), "cleanup")
	writeWorkflow(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.test_source({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
  }
}
`)

	if _, err := client.StartWorkflow(context.Background(), daemonapi.StartWorkflowRequest{
		Target: workflowDir,
	}); err != nil {
		t.Fatalf("start workflow: %v", err)
	}

	if _, err := client.DeleteWorkflow(context.Background(), "cleanup"); err != nil {
		t.Fatalf("delete workflow: %v", err)
	}

	if _, err := client.WorkflowStatus(context.Background(), "cleanup"); err == nil {
		t.Fatalf("expected workflow to be deleted")
	}
}

func TestDaemonListIncludesNameVersionAndFails(t *testing.T) {
	t.Parallel()

	_, client, _ := newTestDaemon(t)

	workflowDir := filepath.Join(t.TempDir(), "failing")
	writeWorkflow(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.test_source({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
    set {
      to = function(item)
        return nil
      end,
    },
  }
}
`)

	if _, err := client.StartWorkflow(context.Background(), daemonapi.StartWorkflowRequest{
		Name:    "cancelamentos",
		Version: "1.2",
		Target:  workflowDir,
	}); err != nil {
		t.Fatalf("start workflow: %v", err)
	}

	resp, err := client.ExecWorkflow(context.Background(), "cancelamentos")
	if err != nil {
		t.Fatalf("exec workflow: %v", err)
	}
	if resp.Report.Status != "error" {
		t.Fatalf("expected error report, got %+v", resp.Report)
	}

	list, err := client.ListWorkflows(context.Background())
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	if len(list.Workflows) != 1 {
		t.Fatalf("unexpected workflows: %+v", list.Workflows)
	}
	workflow := list.Workflows[0]
	if workflow.Name != "cancelamentos" || workflow.Version != "1.2" || workflow.Fails != 1 {
		t.Fatalf("unexpected list row: %+v", workflow)
	}
}

func TestDaemonWebhookTriggerAndConflict(t *testing.T) {
	t.Parallel()

	_, client, cfg := newTestDaemon(t)

	firstDir := filepath.Join(t.TempDir(), "webhook-one")
	writeWorkflow(t, firstDir, "lumn.lua", `
return {
  triggers = {
    lumn.triggers.webhook {
      path = "/hooks/test",
      method = "POST",
    },
  },
  flow = {
    call {
      exec = lumn.test_source({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
    filter {
      condition = function(item)
        local trigger = lumn.trigger_data()
        return trigger.type == "webhook" and trigger.path == "/hooks/test"
      end,
    },
  }
}
`)
	if _, err := client.StartWorkflow(context.Background(), daemonapi.StartWorkflowRequest{
		Target: firstDir,
	}); err != nil {
		t.Fatalf("start first webhook workflow: %v", err)
	}

	secondDir := filepath.Join(t.TempDir(), "webhook-two")
	writeWorkflow(t, secondDir, "lumn.lua", `
return {
  triggers = {
    lumn.triggers.webhook {
      path = "/hooks/test",
      method = "POST",
    },
  },
  flow = {
    call {
      exec = lumn.test_source({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
  }
}
`)
	if _, err := client.StartWorkflow(context.Background(), daemonapi.StartWorkflowRequest{
		Target: secondDir,
	}); err == nil {
		t.Fatalf("expected webhook conflict")
	}

	body := bytes.NewBufferString(`{"ok":true}`)
	resp, err := http.Post("http://127.0.0.1:"+itoa(cfg.WebhookPort)+"/hooks/test", "application/json", body)
	if err != nil {
		t.Fatalf("post webhook: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected webhook status: %d", resp.StatusCode)
	}

	waitForWorkflowStatus(t, client, "webhook-one", "ok")
}

func TestDaemonFileWatcherTrigger(t *testing.T) {
	t.Parallel()

	_, client, _ := newTestDaemon(t)

	watchDir := filepath.Join(t.TempDir(), "watch")
	if err := os.MkdirAll(watchDir, 0o755); err != nil {
		t.Fatalf("mkdir watch dir: %v", err)
	}
	workflowDir := filepath.Join(t.TempDir(), "watcher")
	writeWorkflow(t, workflowDir, "lumn.lua", `
return {
  triggers = {
    lumn.triggers.file_watcher {
      path = "`+strings.ReplaceAll(watchDir, `\`, `\\`)+`",
      pattern = "*.csv",
      event = "create",
      debounce = "500ms",
    },
  },
  flow = {
    call {
      exec = lumn.test_source({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
    filter {
      condition = function(item)
        local trigger = lumn.trigger_data()
        return trigger.type == "file_watcher" and trigger.event == "create"
      end,
    },
  }
}
`)

	if _, err := client.StartWorkflow(context.Background(), daemonapi.StartWorkflowRequest{
		Target: workflowDir,
	}); err != nil {
		t.Fatalf("start watcher workflow: %v", err)
	}

	if err := os.WriteFile(filepath.Join(watchDir, "data.csv"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write watched file: %v", err)
	}

	waitForWorkflowStatus(t, client, "watcher", "ok")
}

func TestDaemonSchedulerTrigger(t *testing.T) {
	t.Parallel()

	_, client, _ := newTestDaemon(t)

	workflowDir := filepath.Join(t.TempDir(), "scheduler")
	writeWorkflow(t, workflowDir, "lumn.lua", `
return {
  triggers = {
    lumn.triggers.scheduler {
      interval = "1s",
    },
  },
  flow = {
    call {
      exec = lumn.test_source({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
    filter {
      condition = function(item)
        return lumn.trigger_data().type == "scheduler"
      end,
    },
  }
}
`)

	if _, err := client.StartWorkflow(context.Background(), daemonapi.StartWorkflowRequest{
		Target: workflowDir,
	}); err != nil {
		t.Fatalf("start scheduler workflow: %v", err)
	}

	waitForWorkflowStatus(t, client, "scheduler", "ok")
}

func newTestDaemon(t *testing.T) (*Daemon, *Client, Config) {
	t.Helper()

	stateDir := t.TempDir()
	paths := Paths{
		StateDir:   stateDir,
		DBPath:     filepath.Join(stateDir, "lumnd.db"),
		ConfigPath: filepath.Join(stateDir, "lumnd.conf"),
		SocketPath: filepath.Join(stateDir, "lumnd.sock"),
		PIDPath:    filepath.Join(stateDir, "lumnd.pid"),
		LogPath:    filepath.Join(stateDir, "lumnd.log"),
	}
	cfg := DefaultConfig(paths)
	cfg.WebhookPort = freePort(t)

	server, err := New(cfg, io.Discard)
	if err != nil {
		t.Fatalf("new daemon: %v", err)
	}
	if err := server.Run(); err != nil {
		t.Fatalf("run daemon: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Shutdown(2 * time.Second)
		_ = server.Close()
	})
	return server, NewClient(paths), cfg
}

func writeWorkflow(t *testing.T, dir, fileName, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
}

func waitForWorkflowStatus(t *testing.T, client *Client, workflowID, expected string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := client.WorkflowStatus(context.Background(), workflowID)
		if err == nil && status.Workflow.LastStatus == expected {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	status, err := client.WorkflowStatus(context.Background(), workflowID)
	if err != nil {
		t.Fatalf("workflow status: %v", err)
	}
	raw, _ := json.Marshal(status)
	t.Fatalf("workflow %s did not reach status %s: %s", workflowID, expected, raw)
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func itoa(value int) string {
	return fmt.Sprintf("%d", value)
}
