package executor

import (
	"fmt"

	"github.com/pretodev/lumn/internal/dag"
	luaenv "github.com/pretodev/lumn/internal/lua"
	"github.com/pretodev/lumn/pkg/errkind"
	"github.com/pretodev/lumn/pkg/primitive"
	golua "github.com/speedata/go-lua"
)

type Report struct {
	Workflow   string        `json:"workflow"`
	Version    string        `json:"version"`
	Status     string        `json:"status"`
	ItemsIn    int           `json:"items_in"`
	ItemsOut   int           `json:"items_out"`
	Errors     []ReportError `json:"errors"`
	DurationMS int64         `json:"duration_ms"`
}

type ReportError struct {
	Type      string `json:"type"`
	Primitive string `json:"primitive,omitempty"`
	Position  int    `json:"position,omitempty"`
	Message   string `json:"message"`
	Callable  string `json:"callable,omitempty"`
}

type RunOptions struct {
	TriggerData map[string]any
}

type itemState struct {
	ItemRef string
}

func Run(workflow *dag.Workflow, opts RunOptions) (Report, error) {
	report := Report{
		Status: "ok",
		Errors: []ReportError{},
	}

	rt := workflow.Runtime
	stateRef := rt.NewTableRef()
	defer rt.DeleteRef(stateRef)
	if err := rt.SetExecutionValue(stateRef, "__lumn_trigger_data", normalizeTriggerData(opts.TriggerData)); err != nil {
		return report, err
	}
	rt.SetExecutionState(stateRef)
	defer rt.SetExecutionState("")

	items := []itemState{}
	defer deleteItemRefs(rt, items)

	for _, node := range workflow.Nodes {
		switch node.Kind {
		case primitive.Call:
			sourceRef, err := rt.CallCallable(node.CallableRef, "", stateRef)
			if err != nil {
				return report, errkind.WithContext(err, string(node.Kind), node.Position, node.CallableName)
			}
			if rt.RefType(sourceRef) != golua.TypeTable {
				return report, &errkind.Error{
					Code:      errkind.ErrRuntime,
					Type:      errkind.TypeRuntime,
					Message:   "call exec must return a table-array of results",
					Primitive: string(node.Kind),
					Position:  node.Position,
					Callable:  node.CallableName,
				}
			}
			defer rt.DeleteRef(sourceRef)

			sourceLen := rt.TableLen(sourceRef)
			deleteItemRefs(rt, items)
			items = make([]itemState, 0, sourceLen)
			for i := 1; i <= sourceLen; i++ {
				resultRef := rt.ArrayValueRef(sourceRef, i)
				refs, err := rt.CallFunction(node.OnDataRef, 1, resultRef)
				rt.DeleteRef(resultRef)
				if err != nil {
					deleteRefs(rt, refs...)
					return report, errkind.WithContext(err, string(node.Kind), node.Position, node.CallableName)
				}
				if len(refs) != 1 || rt.RefType(refs[0]) == golua.TypeNil {
					deleteRefs(rt, refs...)
					return report, &errkind.Error{
						Code:      errkind.ErrRuntime,
						Type:      errkind.TypeRuntime,
						Message:   "call on_data must return item, got nil",
						Primitive: string(node.Kind),
						Position:  node.Position,
						Callable:  node.CallableName,
					}
				}
				items = append(items, itemState{ItemRef: refs[0]})
			}
			report.ItemsIn = len(items)
		case primitive.Set:
			for i := range items {
				refs, err := rt.CallFunction(node.FnRef, 1, items[i].ItemRef)
				if err != nil {
					deleteRefs(rt, refs...)
					return report, errkind.WithContext(err, string(node.Kind), node.Position, "")
				}
				if len(refs) != 1 || rt.RefType(refs[0]) == golua.TypeNil {
					deleteRefs(rt, refs...)
					return report, &errkind.Error{
						Code:      errkind.ErrRuntime,
						Type:      errkind.TypeRuntime,
						Message:   "set.to must return item, got nil",
						Primitive: string(node.Kind),
						Position:  node.Position,
					}
				}
				rt.DeleteRef(items[i].ItemRef)
				items[i].ItemRef = refs[0]
			}
		case primitive.Filter:
			filtered := make([]itemState, 0, len(items))
			for i := range items {
				refs, err := rt.CallFunction(node.FnRef, 1, items[i].ItemRef)
				if err != nil {
					deleteRefs(rt, refs...)
					return report, errkind.WithContext(err, string(node.Kind), node.Position, "")
				}
				if len(refs) != 1 {
					deleteRefs(rt, refs...)
					return report, &errkind.Error{
						Code:      errkind.ErrRuntime,
						Type:      errkind.TypeRuntime,
						Message:   "filter.condition must return a boolean",
						Primitive: string(node.Kind),
						Position:  node.Position,
					}
				}
				rt.PushRef(refs[0])
				isBool := rt.State.IsBoolean(-1)
				keep := rt.State.ToBoolean(-1)
				rt.State.Pop(1)
				rt.DeleteRef(refs[0])
				if !isBool {
					return report, &errkind.Error{
						Code:      errkind.ErrRuntime,
						Type:      errkind.TypeRuntime,
						Message:   "filter.condition must return a boolean",
						Primitive: string(node.Kind),
						Position:  node.Position,
					}
				}
				if keep {
					filtered = append(filtered, items[i])
					continue
				}
				rt.DeleteRef(items[i].ItemRef)
			}
			items = filtered
		case primitive.Tap:
			for i := range items {
				clonedRef, err := rt.CloneRef(items[i].ItemRef)
				if err != nil {
					return report, errkind.WithContext(err, string(node.Kind), node.Position, node.CallableName)
				}
				resultRef, err := rt.CallCallable(node.CallableRef, clonedRef, stateRef)
				rt.DeleteRef(clonedRef)
				if err != nil {
					return report, errkind.WithContext(err, string(node.Kind), node.Position, node.CallableName)
				}
				rt.DeleteRef(resultRef)
			}
		default:
			return report, &errkind.Error{
				Code:      errkind.ErrUnknownPrimitive,
				Type:      errkind.TypeUnknownPrimitive,
				Message:   fmt.Sprintf("unknown primitive %q", node.Kind),
				Primitive: string(node.Kind),
				Position:  node.Position,
			}
		}

		if len(items) == 0 {
			report.Status = "empty"
			report.ItemsOut = 0
			return report, nil
		}
	}

	if len(workflow.Nodes) == 0 {
		report.ItemsOut = 0
		return report, nil
	}

	report.ItemsOut = len(items)
	return report, nil
}

func normalizeTriggerData(triggerData map[string]any) map[string]any {
	if len(triggerData) == 0 {
		return map[string]any{"type": "none"}
	}
	if _, ok := triggerData["type"]; !ok {
		cloned := make(map[string]any, len(triggerData)+1)
		for key, value := range triggerData {
			cloned[key] = value
		}
		cloned["type"] = "none"
		return cloned
	}

	cloned := make(map[string]any, len(triggerData))
	for key, value := range triggerData {
		cloned[key] = value
	}
	return cloned
}

func deleteRefs(rt *luaenv.Runtime, refs ...string) {
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		rt.DeleteRef(ref)
	}
}

func deleteItemRefs(rt *luaenv.Runtime, items []itemState) {
	for i := range items {
		if items[i].ItemRef == "" {
			continue
		}
		rt.DeleteRef(items[i].ItemRef)
		items[i].ItemRef = ""
	}
}

func FailureReport(workflow, version string, err error) Report {
	report := Report{
		Workflow: workflow,
		Version:  version,
		Status:   "error",
		ItemsIn:  0,
		ItemsOut: 0,
		Errors:   []ReportError{},
	}

	var typed *errkind.Error
	if ok := errkind.As(err, &typed); ok {
		report.Errors = append(report.Errors, ReportError{
			Type:      typed.Type,
			Primitive: typed.Primitive,
			Position:  typed.Position,
			Message:   typed.Message,
			Callable:  typed.Callable,
		})
		return report
	}

	report.Errors = append(report.Errors, ReportError{
		Type:    errkind.TypeGeneric,
		Message: err.Error(),
	})
	return report
}
