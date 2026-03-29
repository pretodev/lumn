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
		printMainHelp(stderr)
		return int(errkind.ErrGeneric)
	}

	if isHelpToken(args[0]) {
		printMainHelp(stdout)
		return int(errkind.OK)
	}

	if args[0] == "help" {
		return runHelpCommand(args[1:], stdout, stderr)
	}

	switch args[0] {
	case "validate":
		return runValidateCommand(args[1:], stdout, stderr)
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
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printMainHelp(stderr)
		return int(errkind.ErrGeneric)
	}
}

func runHelpCommand(args []string, stdout, stderr io.Writer) int {
	topic, err := parseHelpTopic(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return int(errkind.ErrGeneric)
	}
	if topic == "" {
		printMainHelp(stdout)
		return int(errkind.OK)
	}
	if !printHelpTopic(stdout, topic) {
		fmt.Fprintf(stderr, "unknown help topic %q\n\n", topic)
		printMainHelp(stderr)
		return int(errkind.ErrGeneric)
	}
	return int(errkind.OK)
}

func runValidateCommand(args []string, stdout, stderr io.Writer) int {
	if hasHelpFlag(args) {
		printValidateHelp(stdout)
		return int(errkind.OK)
	}

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
	if hasHelpFlag(args) {
		printRunHelp(stdout)
		return int(errkind.OK)
	}

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
	if hasHelpFlag(args) {
		printStartHelp(stdout)
		return int(errkind.OK)
	}

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
	if hasHelpFlag(args) {
		printSelectorCommandHelp(stdout, command)
		return int(errkind.OK)
	}

	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(stderr, usageError(command, "expected exactly one workflow selector"))
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
	if hasHelpFlag(args) {
		printListHelp(stdout)
		return int(errkind.OK)
	}

	if len(args) != 0 {
		fmt.Fprintln(stderr, usageError("list", "list does not accept positional arguments"))
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
	if hasHelpFlag(args) {
		printWatchHelp(stdout)
		return int(errkind.OK)
	}

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
	if hasHelpFlag(args) {
		printLogsHelp(stdout)
		return int(errkind.OK)
	}

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
		printDaemonHelp(stderr)
		return int(errkind.ErrGeneric)
	}

	if isHelpToken(args[0]) {
		printDaemonHelp(stdout)
		return int(errkind.OK)
	}

	switch args[0] {
	case "start":
		if hasHelpFlag(args[1:]) {
			printDaemonStartHelp(stdout)
			return int(errkind.OK)
		}
		if len(args) != 1 {
			fmt.Fprintln(stderr, usageError("daemon start", "daemon start does not accept additional arguments"))
			return int(errkind.ErrGeneric)
		}
		if err := startDaemonProcess(); err != nil {
			fmt.Fprintln(stderr, err)
			return int(errkind.ErrGeneric)
		}
		fmt.Fprintln(stdout, "daemon started")
		return int(errkind.OK)
	case "stop":
		if hasHelpFlag(args[1:]) {
			printDaemonStopHelp(stdout)
			return int(errkind.OK)
		}
		if len(args) != 1 {
			fmt.Fprintln(stderr, usageError("daemon stop", "daemon stop does not accept additional arguments"))
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
		if hasHelpFlag(args[1:]) {
			printDaemonStatusHelp(stdout)
			return int(errkind.OK)
		}
		if len(args) != 1 {
			fmt.Fprintln(stderr, usageError("daemon status", "daemon status does not accept additional arguments"))
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
		fmt.Fprintf(stderr, "unknown daemon command %q\n\n", args[0])
		printDaemonHelp(stderr)
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

func parseValidateArgs(args []string) (string, error) {
	target, positionals, err := extractTargetFlag(args)
	if err != nil {
		return "", usageError("validate", err.Error())
	}
	if len(positionals) != 0 {
		return "", usageError("validate", "validate accepts no positional arguments")
	}
	return target, nil
}

func parseRunArgs(args []string) (string, string, error) {
	target, positionals, err := extractTargetFlag(args)
	if err != nil {
		return "", "", usageError("run", err.Error())
	}
	if len(positionals) > 1 {
		return "", "", usageError("run", "run accepts at most one workflow selector")
	}
	if target != "" && len(positionals) > 0 {
		return "", "", usageError("run", "selector and -f cannot be used together")
	}
	if len(positionals) == 1 {
		return positionals[0], target, nil
	}
	return "", target, nil
}

func parseStartArgs(args []string) (string, string, error) {
	target, positionals, err := extractTargetFlag(args)
	if err != nil {
		return "", "", usageError("start", err.Error())
	}
	if len(positionals) > 1 {
		return "", "", usageError("start", "start accepts at most one optional name[:tag] argument")
	}
	name := ""
	if len(positionals) == 1 {
		name = positionals[0]
	}
	return name, target, nil
}

func parseOptionalSelector(args []string, command string) (string, error) {
	if len(args) > 1 {
		return "", usageError(command, fmt.Sprintf("%s accepts at most one workflow selector", command))
	}
	if len(args) == 1 && strings.HasPrefix(args[0], "-") {
		return "", usageError(command, "selectors cannot start with '-'")
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
				return "", usageError("logs", fmt.Sprintf("flag %s requires a value", arg))
			}
			idx++
		default:
			if strings.HasPrefix(arg, "--") {
				return "", usageError("logs", fmt.Sprintf("unknown flag %s", arg))
			}
			if selector != "" {
				return "", usageError("logs", "logs accepts at most one workflow selector")
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

func isHelpToken(value string) bool {
	return value == "-h" || value == "--help"
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if isHelpToken(arg) {
			return true
		}
	}
	return false
}

func usageError(command, message string) error {
	return fmt.Errorf("%s\n\nSee 'lumn %s --help' for usage details.", message, command)
}

func parseHelpTopic(args []string) (string, error) {
	switch len(args) {
	case 0:
		return "", nil
	case 1:
		return args[0], nil
	case 2:
		if args[0] == "daemon" {
			return "daemon " + args[1], nil
		}
	default:
	}
	return "", errors.New("usage: lumn help [command]")
}

func printHelpTopic(w io.Writer, topic string) bool {
	switch topic {
	case "validate":
		printValidateHelp(w)
	case "run":
		printRunHelp(w)
	case "start":
		printStartHelp(w)
	case "stop":
		printStopHelp(w)
	case "delete":
		printDeleteHelp(w)
	case "restart":
		printRestartHelp(w)
	case "list":
		printListHelp(w)
	case "watch":
		printWatchHelp(w)
	case "logs":
		printLogsHelp(w)
	case "daemon":
		printDaemonHelp(w)
	case "daemon start":
		printDaemonStartHelp(w)
	case "daemon stop":
		printDaemonStopHelp(w)
	case "daemon status":
		printDaemonStatusHelp(w)
	default:
		return false
	}
	return true
}

func printMainHelp(w io.Writer) {
	fmt.Fprint(w, `lumn
Developer-first workflow runtime for Lua workflows.

Usage:
  lumn <command> [arguments] [flags]
  lumn help [command]

Workflow commands:
  validate    Validate a local workflow file or directory without executing it.
  run         Execute a workflow locally or trigger a daemon-managed workflow.
  start       Register a workflow in the daemon.
  stop        Stop a daemon-managed workflow.
  delete      Remove a workflow from the daemon permanently.
  restart     Reload a daemon-managed workflow.
  list        List workflows registered in the daemon.
  watch       Open the live workflow TUI (currently a placeholder).
  logs        Stream workflow logs (currently a placeholder).

Daemon commands:
  daemon start    Launch the daemon in the background.
  daemon stop     Shut the daemon down gracefully.
  daemon status   Show daemon health information.

Selectors:
  Commands that accept <id|name> support:
    - full workflow IDs
    - unique workflow ID prefixes
    - workflow names
  If a name matches multiple versions, the workflow tagged "latest" wins.

Entrypoint resolution:
  lumn run / lumn validate
      Load ./lumn.lua from the current directory.
  lumn run -f <directory>
      Load <directory>/init.lua, then <directory>/lumn.lua.
  lumn run <selector>
      Try the daemon first, then a local directory, then <selector>.lua.

Use "lumn <command> --help" for command-specific guidance.
`)
}

func printValidateHelp(w io.Writer) {
	fmt.Fprint(w, `lumn validate
Validate a workflow file or directory without running it.

Usage:
  lumn validate
  lumn validate -f <file|directory>

Resolution:
  Without -f, validate loads ./lumn.lua in the current directory.
  With -f <directory>, validate resolves <directory>/init.lua first,
  then <directory>/lumn.lua.
  With -f <file>, validate uses the exact file you provided.

Examples:
  lumn validate
  lumn validate -f ./lumn.lua
  lumn validate -f ./workflows/order_cancel
`)
}

func printRunHelp(w io.Writer) {
	fmt.Fprint(w, `lumn run
Execute a workflow locally or trigger a daemon-managed workflow.

Usage:
  lumn run
  lumn run <id|name>
  lumn run -f <file|directory>

Behavior:
  Without arguments, run loads ./lumn.lua and executes it locally.
  With -f, run executes the given local file or directory and skips the daemon.
  With <id|name>, run tries the daemon first. If the daemon is unavailable or
  the workflow is not registered there, run falls back to local resolution:
    1. local directory
    2. local .lua file (<selector>.lua)

Selectors:
  <id|name> accepts a full workflow ID, a unique ID prefix, or a workflow name.

Examples:
  lumn run
  lumn run sales-report
  lumn run a1b2
  lumn run -f ./workflows/order_cancel
  lumn run -f ./cancelamentos.lua
`)
}

func printStartHelp(w io.Writer) {
	fmt.Fprint(w, `lumn start
Register a workflow in the daemon.

Usage:
  lumn start
  lumn start [name[:tag]]
  lumn start [name[:tag]] -f <file|directory>

Behavior:
  start always resolves a local workflow target and sends it to the daemon.
  If name is omitted, lumn infers it from the resolved target.
  If tag is omitted, lumn uses "latest".
  Starting the same name:tag again updates the existing workflow in place.

Resolution:
  Without -f, start resolves ./lumn.lua in the current directory.
  With -f <directory>, start resolves <directory>/init.lua first,
  then <directory>/lumn.lua.
  With -f <file>, start uses the exact file you provided.

Examples:
  lumn start
  lumn start cancelamentos
  lumn start cancelamentos:1.2
  lumn start pedidos -f ./workflows/order_cancel
  lumn start finance-sync:2026-03 -f ./finance.lua
`)
}

func printSelectorCommandHelp(w io.Writer, command string) {
	switch command {
	case "stop":
		printStopHelp(w)
	case "delete":
		printDeleteHelp(w)
	case "restart":
		printRestartHelp(w)
	}
}

func printStopHelp(w io.Writer) {
	fmt.Fprint(w, `lumn stop
Stop a daemon-managed workflow and disable its triggers.

Usage:
  lumn stop <id|name>

Selectors:
  <id|name> accepts a full workflow ID, a unique ID prefix, or a workflow name.

Examples:
  lumn stop cancelamentos
  lumn stop a1b2
`)
}

func printDeleteHelp(w io.Writer) {
	fmt.Fprint(w, `lumn delete
Remove a workflow from the daemon permanently.

Usage:
  lumn delete <id|name>

Behavior:
  delete stops the workflow if it is active, clears queued work, removes
  triggers, and deletes the workflow record from the daemon store.

Selectors:
  <id|name> accepts a full workflow ID, a unique ID prefix, or a workflow name.

Examples:
  lumn delete cancelamentos
  lumn delete a1b2
`)
}

func printRestartHelp(w io.Writer) {
	fmt.Fprint(w, `lumn restart
Reload a daemon-managed workflow from its registered target.

Usage:
  lumn restart <id|name>

Selectors:
  <id|name> accepts a full workflow ID, a unique ID prefix, or a workflow name.

Examples:
  lumn restart cancelamentos
  lumn restart a1b2
`)
}

func printListHelp(w io.Writer) {
	fmt.Fprint(w, `lumn list
List workflows currently registered in the daemon.

Usage:
  lumn list

Columns:
  ID         Generated workflow ID.
  NAME       Runtime workflow name.
  VERSION    Runtime version tag, or "latest".
  STATUS     Current daemon status.
  LAST RUN   Timestamp of the most recent execution.
  FAILS      Number of failed executions since the last workflow update.
  NEXT RUN   Next scheduled execution time, when available.

Example:
  lumn list
`)
}

func printWatchHelp(w io.Writer) {
	fmt.Fprint(w, `lumn watch
Open the live workflow TUI.

Usage:
  lumn watch
  lumn watch <id|name>

Selectors:
  <id|name> accepts a full workflow ID, a unique ID prefix, or a workflow name.

Status:
  The command is registered and validates daemon connectivity, but the
  interactive TUI is still a placeholder in the current implementation.
`)
}

func printLogsHelp(w io.Writer) {
	fmt.Fprint(w, `lumn logs
Stream workflow logs from the daemon.

Usage:
  lumn logs
  lumn logs <id|name>
  lumn logs [<id|name>] [--lines <n>] [--no-follow] [--since <duration>] [--level <level>] [--step <name>]

Selectors:
  <id|name> accepts a full workflow ID, a unique ID prefix, or a workflow name.

Flags:
  --lines <n>       Show the last n lines before following.
  --no-follow       Print historical logs only.
  --since <dur>     Filter logs newer than the given duration.
  --level <level>   Filter by log level.
  --step <name>     Filter to a specific workflow step.

Status:
  The command is registered and validates daemon connectivity, but log streaming
  is still a placeholder in the current implementation.
`)
}

func printDaemonHelp(w io.Writer) {
	fmt.Fprint(w, `lumn daemon
Manage the local lumn daemon process.

Usage:
  lumn daemon <subcommand>

Subcommands:
  start     Launch the daemon in the background.
  stop      Stop the daemon gracefully.
  status    Show daemon health information.

Examples:
  lumn daemon start
  lumn daemon stop
  lumn daemon status
`)
}

func printDaemonStartHelp(w io.Writer) {
	fmt.Fprint(w, `lumn daemon start
Launch the daemon in the background.

Usage:
  lumn daemon start

Behavior:
  start writes daemon logs to the configured state directory and waits until the
  daemon health endpoint becomes reachable.
`)
}

func printDaemonStopHelp(w io.Writer) {
	fmt.Fprint(w, `lumn daemon stop
Stop the daemon gracefully.

Usage:
  lumn daemon stop

Behavior:
  stop asks the running daemon to shut down cleanly, stopping triggers and
  waiting for in-flight executions to finish within the shutdown timeout.
`)
}

func printDaemonStatusHelp(w io.Writer) {
	fmt.Fprint(w, `lumn daemon status
Show daemon health information.

Usage:
  lumn daemon status

Output:
  running          Whether the daemon responds to health checks.
  transport        Socket or pipe used by the CLI.
  webhook_port     Local HTTP port for webhook triggers.
  active_workflows Number of active workflows currently loaded.
  uptime_seconds   Daemon uptime in seconds.
`)
}
