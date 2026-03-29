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
	case "validate":
		return runValidateCommand(args[1:], stderr)
	case "run":
		return runRunCommand(args[1:], stdout, stderr)
	case "start":
		return runStartCommand(args[1:], stdout, stderr)
	case "stop":
		return runSelectorMutation(args[1:], stdout, stderr, "stop", func(client *daemon.Client, selector string) (daemonapi.WorkflowMutationResponse, error) {
			return client.StopWorkflow(context.Background(), selector)
		})
	case "delete":
		return runSelectorMutation(args[1:], stdout, stderr, "delete", func(client *daemon.Client, selector string) (daemonapi.WorkflowMutationResponse, error) {
			return client.DeleteWorkflow(context.Background(), selector)
		})
	case "restart":
		return runSelectorMutation(args[1:], stdout, stderr, "restart", func(client *daemon.Client, selector string) (daemonapi.WorkflowMutationResponse, error) {
			return client.RestartWorkflow(context.Background(), selector)
		})
	case "list":
		return runListCommand(args[1:], stdout, stderr)
	case "watch":
		return runWatchCommand(args[1:], stdout, stderr)
	case "logs":
		return runLogsCommand(args[1:], stdout, stderr)
	case "daemon":
		return runDaemonCommand(args[1:], stdout, stderr)
	default:
		printUsage(stderr)
		return int(errkind.ErrGeneric)
	}
}

func runValidateCommand(args []string, stderr io.Writer) int {
	target, err := parseValidateArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}
	if err := engine.ValidateTarget(target, stderr); err != nil {
		fmt.Fprintln(stderr, errkind.Format(err))
		return errkind.ExitStatus(err)
	}
	return int(errkind.OK)
}

func runRunCommand(args []string, stdout, stderr io.Writer) int {
	selector, forcedTarget, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	if forcedTarget != "" {
		report, code := engine.RunTarget(forcedTarget, stderr)
		writeReport(stdout, report)
		return code
	}

	if selector == "" {
		report, code := engine.RunTarget("", stderr)
		writeReport(stdout, report)
		return code
	}

	client, err := newDaemonClient()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	resp, err := client.ExecWorkflow(context.Background(), selector)
	if err == nil {
		writeReport(stdout, resp.Report)
		return int(errkind.OK)
	}

	var apiErr *daemon.APIError
	if !daemon.IsDaemonNotRunning(err) && !(errors.As(err, &apiErr) && apiErr.StatusCode == 404) {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	target, resolveErr := engine.ResolveLocalSelector(selector)
	if resolveErr != nil {
		writeReport(stdout, executor.FailureReport(inferSelectorName(selector), "latest", resolveErr))
		return errkind.ExitStatus(resolveErr)
	}

	report, code := engine.RunTarget(target.TargetPath, stderr)
	writeReport(stdout, report)
	return code
}

func runStartCommand(args []string, stdout, stderr io.Writer) int {
	nameArg, targetArg, err := parseStartArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	target, err := engine.ResolveTarget(targetArg)
	if err != nil {
		fmt.Fprintln(stderr, errkind.Format(err))
		return errkind.ExitStatus(err)
	}

	name, version := splitNameVersion(nameArg)
	if name == "" {
		name = target.Name
	}
	if version == "" {
		version = "latest"
	}

	client, err := newDaemonClient()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	resp, err := client.StartWorkflow(context.Background(), daemonapi.StartWorkflowRequest{
		Name:    name,
		Version: version,
		Target:  target.TargetPath,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	fmt.Fprintln(stdout, resp.Message)
	return int(errkind.OK)
}

func runSelectorMutation(
	args []string,
	stdout, stderr io.Writer,
	command string,
	run func(client *daemon.Client, selector string) (daemonapi.WorkflowMutationResponse, error),
) int {
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(stderr, "usage: lumn %s <id|name>\n", command)
		return int(errkind.ErrGeneric)
	}

	client, err := newDaemonClient()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	resp, err := run(client, args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	fmt.Fprintln(stdout, resp.Message)
	return int(errkind.OK)
}

func runListCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: lumn list")
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

	renderWorkflowList(stdout, resp)
	return int(errkind.OK)
}

func runWatchCommand(args []string, stdout, stderr io.Writer) int {
	selector, err := parseOptionalSelector(args, "watch")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	client, err := newDaemonClient()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}
	if selector != "" {
		if _, err := client.WorkflowStatus(context.Background(), selector); err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
	} else if _, err := client.Health(context.Background()); err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	fmt.Fprintln(stdout, "watch is not implemented yet")
	return int(errkind.OK)
}

func runLogsCommand(args []string, stdout, stderr io.Writer) int {
	selector, err := parseLogsArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	client, err := newDaemonClient()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}
	if selector != "" {
		if _, err := client.WorkflowStatus(context.Background(), selector); err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
	} else if _, err := client.Health(context.Background()); err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}

	fmt.Fprintln(stdout, "logs is not implemented yet")
	return int(errkind.OK)
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

func writeReport(stdout io.Writer, report executor.Report) {
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(report)
}

func renderWorkflowList(stdout io.Writer, resp daemonapi.WorkflowsListResponse) {
	tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tVERSION\tSTATUS\tLAST RUN\tFAILS\tNEXT RUN")
	for _, workflow := range resp.Workflows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			workflow.ID,
			workflow.Name,
			workflow.Version,
			workflow.Status,
			orDash(workflow.LastRun),
			workflow.Fails,
			orDash(workflow.NextRun),
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
	fmt.Fprintln(w, "usage: lumn <validate|run|start|stop|delete|restart|list|watch|logs|daemon>")
}

func parseValidateArgs(args []string) (string, error) {
	target, positionals, err := extractTargetFlag(args)
	if err != nil {
		return "", err
	}
	if len(positionals) != 0 {
		return "", errors.New("usage: lumn validate [-f <arquivo|pasta>]")
	}
	return target, nil
}

func parseRunArgs(args []string) (string, string, error) {
	target, positionals, err := extractTargetFlag(args)
	if err != nil {
		return "", "", err
	}
	if len(positionals) > 1 {
		return "", "", errors.New("usage: lumn run [id|name] | lumn run -f <arquivo|pasta>")
	}
	if target != "" && len(positionals) > 0 {
		return "", "", errors.New("usage: lumn run [id|name] | lumn run -f <arquivo|pasta>")
	}
	if len(positionals) == 1 {
		return positionals[0], target, nil
	}
	return "", target, nil
}

func parseStartArgs(args []string) (string, string, error) {
	target, positionals, err := extractTargetFlag(args)
	if err != nil {
		return "", "", err
	}
	if len(positionals) > 1 {
		return "", "", errors.New("usage: lumn start [name[:tag]] [-f <arquivo|pasta>]")
	}
	name := ""
	if len(positionals) == 1 {
		name = positionals[0]
	}
	return name, target, nil
}

func parseOptionalSelector(args []string, command string) (string, error) {
	if len(args) > 1 {
		return "", fmt.Errorf("usage: lumn %s [id|name]", command)
	}
	if len(args) == 1 && strings.HasPrefix(args[0], "-") {
		return "", fmt.Errorf("usage: lumn %s [id|name]", command)
	}
	if len(args) == 0 {
		return "", nil
	}
	return args[0], nil
}

func parseLogsArgs(args []string) (string, error) {
	var selector string
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		switch arg {
		case "--no-follow":
		case "--lines", "--since", "--level", "--step":
			if idx+1 >= len(args) {
				return "", errors.New("usage: lumn logs [id|name] [--lines <n>] [--no-follow] [--since <duration>] [--level <level>] [--step <nome>]")
			}
			idx++
		default:
			if strings.HasPrefix(arg, "--") {
				return "", errors.New("usage: lumn logs [id|name] [--lines <n>] [--no-follow] [--since <duration>] [--level <level>] [--step <nome>]")
			}
			if selector != "" {
				return "", errors.New("usage: lumn logs [id|name] [--lines <n>] [--no-follow] [--since <duration>] [--level <level>] [--step <nome>]")
			}
			selector = arg
		}
	}
	return selector, nil
}

func extractTargetFlag(args []string) (string, []string, error) {
	target := ""
	positionals := make([]string, 0, len(args))
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		switch {
		case arg == "-f":
			if idx+1 >= len(args) {
				return "", nil, errors.New("missing value for -f")
			}
			target = args[idx+1]
			idx++
		case strings.HasPrefix(arg, "-f="):
			target = strings.TrimPrefix(arg, "-f=")
		case strings.HasPrefix(arg, "-"):
			return "", nil, fmt.Errorf("unknown flag %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	return target, positionals, nil
}

func splitNameVersion(raw string) (string, string) {
	if raw == "" {
		return "", ""
	}
	name, version, ok := strings.Cut(raw, ":")
	if !ok {
		return raw, ""
	}
	return name, version
}

func inferSelectorName(selector string) string {
	base := filepath.Base(filepath.Clean(selector))
	return strings.TrimSuffix(base, filepath.Ext(base))
}
