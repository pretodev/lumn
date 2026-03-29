package engine

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pretodev/lumn/internal/dag"
	"github.com/pretodev/lumn/internal/executor"
	luaenv "github.com/pretodev/lumn/internal/lua"
	"github.com/pretodev/lumn/pkg/errkind"
)

type Target struct {
	Input        string
	InitPath     string
	WorkspaceDir string
	WorkflowDir  string
	Name         string
}

func ResolveTarget(input string) (Target, error) {
	if input == "" {
		return Target{}, errkind.New(errkind.ErrGeneric, errkind.TypeGeneric, "workflow path is required")
	}

	absInput, err := filepath.Abs(input)
	if err != nil {
		return Target{}, errkind.Wrap(errkind.ErrGeneric, errkind.TypeGeneric, err.Error(), err)
	}

	info, err := os.Stat(absInput)
	if err != nil {
		return Target{}, errkind.New(errkind.ErrWorkflowNotFound, errkind.TypeWorkflowNotFound, "workflow not found")
	}

	target := Target{Input: input}
	if info.IsDir() {
		target.WorkflowDir = absInput
		target.InitPath = filepath.Join(absInput, "init.lua")
		if _, err := os.Stat(target.InitPath); err != nil {
			return Target{}, errkind.New(errkind.ErrWorkflowNotFound, errkind.TypeWorkflowNotFound, "workflow init.lua not found")
		}
	} else {
		target.InitPath = absInput
		target.WorkflowDir = filepath.Dir(absInput)
	}
	target.WorkspaceDir, err = inferWorkspaceDir(target.WorkflowDir)
	if err != nil {
		return Target{}, errkind.Wrap(errkind.ErrGeneric, errkind.TypeGeneric, err.Error(), err)
	}
	target.Name = filepath.Base(target.WorkflowDir)
	return target, nil
}

func LoadTarget(input string, stderr io.Writer) (*dag.Workflow, Target, error) {
	target, err := ResolveTarget(input)
	if err != nil {
		return nil, target, err
	}

	rt, err := luaenv.NewRuntime(target.WorkflowDir, target.WorkspaceDir, stderr)
	if err != nil {
		return nil, target, err
	}
	keepRuntime := false
	defer func() {
		if !keepRuntime {
			rt.Close()
		}
	}()

	workflowRef, err := rt.LoadWorkflow(target.InitPath)
	if err != nil {
		return nil, target, err
	}

	workflow, err := dag.Build(rt, workflowRef)
	if err != nil {
		return nil, target, err
	}
	keepRuntime = true
	return workflow, target, nil
}

func ValidateTarget(input string, stderr io.Writer) error {
	workflow, _, err := LoadTarget(input, stderr)
	if workflow != nil && workflow.Runtime != nil {
		defer workflow.Runtime.Close()
	}
	return err
}

func RunTarget(input string, stderr io.Writer) (executor.Report, int) {
	start := time.Now()
	workflow, target, err := LoadTarget(input, stderr)
	if err != nil {
		report := errorReport(fallbackName(input, target.Name), "", err)
		report.DurationMS = time.Since(start).Milliseconds()
		return report, errkind.ExitStatus(err)
	}
	defer workflow.Runtime.Close()

	report, runErr := executor.Run(workflow)
	if runErr != nil {
		report.Status = "error"
		report.ItemsOut = 0
		report.Errors = []executor.ReportError{reportError(runErr)}
		report.DurationMS = time.Since(start).Milliseconds()
		return report, errkind.ExitStatus(runErr)
	}

	report.DurationMS = time.Since(start).Milliseconds()
	return report, int(errkind.OK)
}

func errorReport(workflow, version string, err error) executor.Report {
	return executor.Report{
		Workflow:   workflow,
		Version:    version,
		Status:     "error",
		ItemsIn:    0,
		ItemsOut:   0,
		Errors:     []executor.ReportError{reportError(err)},
		DurationMS: 0,
	}
}

func reportError(err error) executor.ReportError {
	if typed := errkind.WithContext(err, "", 0, ""); typed != nil {
		return executor.ReportError{
			Type:      typed.Type,
			Primitive: typed.Primitive,
			Position:  typed.Position,
			Message:   typed.Message,
			Callable:  typed.Callable,
		}
	}
	return executor.ReportError{
		Type:    errkind.TypeGeneric,
		Message: err.Error(),
	}
}

func fallbackName(input, resolved string) string {
	if resolved != "" {
		return resolved
	}
	if input == "" {
		return "workflow"
	}
	return filepath.Base(filepath.Clean(input))
}

func inferWorkspaceDir(workflowDir string) (string, error) {
	current := workflowDir
	for {
		found, err := hasWorkspaceMarker(current)
		if err != nil {
			return "", err
		}
		if found {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	parent := filepath.Dir(workflowDir)
	if parent == "" {
		return workflowDir, nil
	}
	return parent, nil
}

func hasWorkspaceMarker(dir string) (bool, error) {
	for _, name := range []string{"lumn.lock", "lumn.config.lua"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true, nil
		} else if err != nil && !os.IsNotExist(err) {
			return false, err
		}
	}

	matches, err := filepath.Glob(filepath.Join(dir, "lumn.config.*.lua"))
	if err != nil {
		return false, err
	}
	if len(matches) > 0 {
		return true, nil
	}

	sharedPath := filepath.Join(dir, "_shared")
	info, err := os.Stat(sharedPath)
	if err == nil && info.IsDir() {
		return true, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	return false, nil
}
