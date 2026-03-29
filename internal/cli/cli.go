package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/pretodev/lumn/internal/daemon"
	"github.com/pretodev/lumn/internal/daemonapi"
	"github.com/pretodev/lumn/internal/engine"
	"github.com/pretodev/lumn/internal/executor"
	"github.com/pretodev/lumn/pkg/errkind"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return int(errkind.ErrGeneric)
	}

	switch args[0] {
	case "init":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: lumn init <name>")
			return int(errkind.ErrGeneric)
		}
		if err := initWorkflow(args[1]); err != nil {
			fmt.Fprintln(stderr, errkind.Format(err))
			return errkind.ExitStatus(err)
		}
		fmt.Fprintf(stdout, "created %s\n", filepath.Join(args[1], "init.lua"))
		return int(errkind.OK)
	case "validate":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: lumn validate <workflow|init.lua>")
			return int(errkind.ErrGeneric)
		}
		if err := engine.ValidateTarget(args[1], stderr); err != nil {
			fmt.Fprintln(stderr, errkind.Format(err))
			return errkind.ExitStatus(err)
		}
		return int(errkind.OK)
	case "run":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: lumn run <workflow|init.lua>")
			return int(errkind.ErrGeneric)
		}
		report, code := engine.RunTarget(args[1], stderr)
		writeReport(stdout, report)
		return code
	case "start":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: lumn start <workflow>")
			return int(errkind.ErrGeneric)
		}
		client, err := newDaemonClient()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		resp, err := client.StartWorkflow(context.Background(), args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		fmt.Fprintln(stdout, resp.Message)
		return int(errkind.OK)
	case "stop":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: lumn stop <workflow-id>")
			return int(errkind.ErrGeneric)
		}
		client, err := newDaemonClient()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		resp, err := client.StopWorkflow(context.Background(), args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		fmt.Fprintln(stdout, resp.Message)
		return int(errkind.OK)
	case "restart":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: lumn restart <workflow-id>")
			return int(errkind.ErrGeneric)
		}
		client, err := newDaemonClient()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		resp, err := client.RestartWorkflow(context.Background(), args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		fmt.Fprintln(stdout, resp.Message)
		return int(errkind.OK)
	case "status":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: lumn status")
			return int(errkind.ErrGeneric)
		}
		client, err := newDaemonClient()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		resp, err := client.ListWorkflows(context.Background())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		renderWorkflowStatus(stdout, resp)
		return int(errkind.OK)
	case "exec":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: lumn exec <workflow-id>")
			return int(errkind.ErrGeneric)
		}
		client, err := newDaemonClient()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		resp, err := client.ExecWorkflow(context.Background(), args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		writeReport(stdout, resp.Report)
		return int(errkind.OK)
	case "daemon":
		return runDaemonCommand(args[1:], stdout, stderr)
	default:
		printUsage(stderr)
		return int(errkind.ErrGeneric)
	}
}

func runDaemonCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: lumn daemon <start|stop|status>")
		return int(errkind.ErrGeneric)
	}

	switch args[0] {
	case "start":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: lumn daemon start")
			return int(errkind.ErrGeneric)
		}
		if err := startDaemonProcess(); err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		fmt.Fprintln(stdout, "daemon started")
		return int(errkind.OK)
	case "stop":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: lumn daemon stop")
			return int(errkind.ErrGeneric)
		}
		client, err := newDaemonClient()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		if err := client.Shutdown(context.Background()); err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		fmt.Fprintln(stdout, "daemon stopped")
		return int(errkind.OK)
	case "status":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: lumn daemon status")
			return int(errkind.ErrGeneric)
		}
		client, err := newDaemonClient()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		health, err := client.Health(context.Background())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		renderDaemonStatus(stdout, health)
		return int(errkind.OK)
	default:
		fmt.Fprintln(stderr, "usage: lumn daemon <start|stop|status>")
		return int(errkind.ErrGeneric)
	}
}

type scaffoldData struct {
	ID string
}

func newDaemonClient() (*daemon.Client, error) {
	paths, err := daemon.DefaultPaths()
	if err != nil {
		return nil, err
	}
	return daemon.NewClient(paths), nil
}

func startDaemonProcess() error {
	paths, err := daemon.DefaultPaths()
	if err != nil {
		return err
	}
	if err := paths.EnsureStateDir(); err != nil {
		return err
	}

	client := daemon.NewClient(paths)
	if _, err := client.Health(context.Background()); err == nil {
		return errors.New("daemon is already running")
	}

	logFile, err := os.OpenFile(paths.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	commandPath, err := resolveLumndBinary()
	if err != nil {
		return err
	}

	cmd := exec.Command(commandPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.Env = os.Environ()
	applyDaemonProcessAttrs(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if _, err := client.Health(context.Background()); err == nil {
			_ = cmd.Process.Release()
			return nil
		}
	}

	_ = cmd.Process.Release()
	return errors.New("daemon did not become ready in time")
}

func resolveLumndBinary() (string, error) {
	if value := os.Getenv("LUMN_LUMND_BIN"); value != "" {
		return value, nil
	}

	current, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(current), daemonBinaryName())
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}
	return exec.LookPath(daemonBinaryName())
}

func daemonBinaryName() string {
	if runtime.GOOS == "windows" {
		return "lumnd.exe"
	}
	return "lumnd"
}

func initWorkflow(name string) error {
	if name == "" {
		return errkind.New(errkind.ErrGeneric, errkind.TypeGeneric, "workflow name is required")
	}

	if _, err := os.Stat(name); err == nil {
		return errkind.New(errkind.ErrGeneric, errkind.TypeGeneric, "target already exists")
	}

	if err := os.MkdirAll(name, 0o755); err != nil {
		return errkind.Wrap(errkind.ErrGeneric, errkind.TypeGeneric, err.Error(), err)
	}

	initPath := filepath.Join(name, "init.lua")
	file, err := os.Create(initPath)
	if err != nil {
		return errkind.Wrap(errkind.ErrGeneric, errkind.TypeGeneric, err.Error(), err)
	}
	defer file.Close()

	tpl := template.Must(template.New("init").Parse(initTemplate))
	if err := tpl.Execute(file, scaffoldData{ID: filepath.Base(name)}); err != nil {
		return errkind.Wrap(errkind.ErrGeneric, errkind.TypeGeneric, err.Error(), err)
	}

	return nil
}

func writeReport(stdout io.Writer, report executor.Report) {
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(report)
}

func renderWorkflowStatus(stdout io.Writer, resp daemonapi.WorkflowsListResponse) {
	tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tVersion\tStatus\tTrigger\tNext Run\tLast Run\tLast Status")
	for _, workflow := range resp.Workflows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			workflow.ID,
			workflow.Version,
			workflow.Status,
			strings.Join(workflow.Triggers, ","),
			orDash(workflow.NextRun),
			orDash(workflow.LastRun),
			orDash(workflow.LastStatus),
		)
	}
	_ = tw.Flush()
}

func renderDaemonStatus(stdout io.Writer, health daemonapi.HealthResponse) {
	fmt.Fprintf(stdout, "running: %t\n", health.Running)
	fmt.Fprintf(stdout, "transport: %s\n", health.Transport)
	fmt.Fprintf(stdout, "webhook_port: %d\n", health.WebhookPort)
	fmt.Fprintf(stdout, "active_workflows: %d\n", health.ActiveWorkflows)
	fmt.Fprintf(stdout, "uptime_seconds: %d\n", health.UptimeSeconds)
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: lumn <init|validate|run|start|stop|restart|status|exec|daemon>")
}

const initTemplate = `local items = {
  { id = 1, nome = "Item A", valor = 100 },
  { id = 2, nome = "Item B", valor = 50 },
  { id = 3, nome = "Item C", valor = 200 },
}

local log_item = {
  name = "log_item",
  run = function(input)
    print(input.nome .. " aprovado")
  end
}

return {
  id = "{{ .ID }}",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source(items),
      on_data = function(result)
        return result
      end,
    },
    set {
      to = function(item)
        lumn.set("ultimo_item_id", item.id)
        item.ultimo_item_id = lumn.get("ultimo_item_id")
        item.processado = true
        return item
      end,
    },
    filter {
      condition = function(item)
        return item.valor > 80
      end,
    },
    tap {
      exec = log_item,
    },
  }
}
`
