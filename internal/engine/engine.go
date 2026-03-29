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
	golua "github.com/speedata/go-lua"
)

type Target struct {
	Input        string
	InitPath     string
	WorkspaceDir string
	WorkflowDir  string
	Name         string
}

type Definition struct {
	ID       string
	Version  string
	Triggers []TriggerSpec
}

type TriggerSpec struct {
	Type   string
	Config map[string]any
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
	return RunTargetWithOptions(input, stderr, executor.RunOptions{})
}

func RunTargetWithOptions(input string, stderr io.Writer, opts executor.RunOptions) (executor.Report, int) {
	start := time.Now()
	workflow, target, err := LoadTarget(input, stderr)
	if err != nil {
		report := errorReport(fallbackName(input, target.Name), "", err)
		report.DurationMS = time.Since(start).Milliseconds()
		return report, errkind.ExitStatus(err)
	}
	defer workflow.Runtime.Close()

	report, runErr := executor.Run(workflow, opts)
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

func LoadDefinition(input string, stderr io.Writer) (Definition, Target, error) {
	target, err := ResolveTarget(input)
	if err != nil {
		return Definition{}, target, err
	}

	rt, err := luaenv.NewRuntime(target.WorkflowDir, target.WorkspaceDir, stderr)
	if err != nil {
		return Definition{}, target, err
	}
	defer rt.Close()

	workflowRef, err := rt.LoadWorkflow(target.InitPath)
	if err != nil {
		return Definition{}, target, err
	}
	defer rt.DeleteRef(workflowRef)

	workflow, err := dag.Build(rt, workflowRef)
	if err != nil {
		return Definition{}, target, err
	}
	if workflow.Runtime != nil {
		workflow.Runtime = nil
	}

	definition := Definition{
		ID:      workflow.ID,
		Version: workflow.Version,
	}

	triggersRef, ok := rt.TableRefFieldRef(workflowRef, "triggers")
	if !ok {
		definition.Triggers = []TriggerSpec{{Type: "manual", Config: map[string]any{}}}
		return definition, target, nil
	}
	defer rt.DeleteRef(triggersRef)

	if rt.RefType(triggersRef) != golua.TypeTable {
		return Definition{}, target, errkind.New(errkind.ErrStructure, errkind.TypeStructure, `workflow field "triggers" must be a table-array`)
	}

	triggerLen := rt.TableLen(triggersRef)
	if triggerLen == 0 {
		definition.Triggers = []TriggerSpec{{Type: "manual", Config: map[string]any{}}}
		return definition, target, nil
	}

	definition.Triggers = make([]TriggerSpec, 0, triggerLen)
	for idx := 1; idx <= triggerLen; idx++ {
		triggerRef := rt.ArrayValueRef(triggersRef, idx)
		if rt.RefType(triggerRef) != golua.TypeTable {
			rt.DeleteRef(triggerRef)
			return Definition{}, target, errkind.New(errkind.ErrStructure, errkind.TypeStructure, "each trigger must be a table")
		}

		triggerType, ok := rt.TriggerKindRef(triggerRef)
		if !ok || triggerType == "" {
			rt.DeleteRef(triggerRef)
			return Definition{}, target, errkind.New(errkind.ErrStructure, errkind.TypeStructure, "triggers must be declared with lumn.triggers.*")
		}

		value, err := rt.RefToGoValue(triggerRef)
		rt.DeleteRef(triggerRef)
		if err != nil {
			return Definition{}, target, err
		}

		config, err := normalizeTriggerConfig(value)
		if err != nil {
			return Definition{}, target, err
		}
		delete(config, "__lumn_trigger_type")

		definition.Triggers = append(definition.Triggers, TriggerSpec{
			Type:   triggerType,
			Config: config,
		})
	}

	return definition, target, nil
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

func normalizeTriggerConfig(value any) (map[string]any, error) {
	raw, ok := value.(map[any]any)
	if !ok {
		return nil, errkind.New(errkind.ErrStructure, errkind.TypeStructure, "trigger config must be a table")
	}

	config := make(map[string]any, len(raw))
	for key, item := range raw {
		keyString, ok := key.(string)
		if !ok {
			return nil, errkind.New(errkind.ErrStructure, errkind.TypeStructure, "trigger config keys must be strings")
		}
		config[keyString] = normalizeGoValue(item)
	}
	return config, nil
}

func normalizeGoValue(value any) any {
	switch typed := value.(type) {
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			keyString, ok := key.(string)
			if !ok {
				continue
			}
			out[keyString] = normalizeGoValue(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeGoValue(item))
		}
		return out
	default:
		return typed
	}
}
