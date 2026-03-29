package executor

import (
	"fmt"

	"github.com/pretodev/lumn/internal/dag"
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

type itemState struct {
	ItemRef string
}

func Run(workflow *dag.Workflow) (Report, error) {
	report := Report{
		Workflow: workflow.ID,
		Version:  workflow.Version,
		Status:   "ok",
		Errors:   []ReportError{},
	}

	rt := workflow.Runtime
	stateRef := rt.NewTableRef()
	rt.SetExecutionState(stateRef)
	defer rt.SetExecutionState("")

	items := []itemState{}

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

			items = make([]itemState, 0, rt.TableLen(sourceRef))
			for i := 1; i <= rt.TableLen(sourceRef); i++ {
				resultRef := rt.ArrayValueRef(sourceRef, i)
				refs, err := rt.CallFunction(node.OnDataRef, 1, resultRef)
				if err != nil {
					return report, errkind.WithContext(err, string(node.Kind), node.Position, node.CallableName)
				}
				if len(refs) != 1 || rt.RefType(refs[0]) == golua.TypeNil {
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
					return report, errkind.WithContext(err, string(node.Kind), node.Position, "")
				}
				if len(refs) != 1 || rt.RefType(refs[0]) == golua.TypeNil {
					return report, &errkind.Error{
						Code:      errkind.ErrRuntime,
						Type:      errkind.TypeRuntime,
						Message:   "set.to must return item, got nil",
						Primitive: string(node.Kind),
						Position:  node.Position,
					}
				}
				items[i].ItemRef = refs[0]
			}
		case primitive.Filter:
			filtered := make([]itemState, 0, len(items))
			for i := range items {
				refs, err := rt.CallFunction(node.FnRef, 1, items[i].ItemRef)
				if err != nil {
					return report, errkind.WithContext(err, string(node.Kind), node.Position, "")
				}
				if len(refs) != 1 {
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
				}
			}
			items = filtered
		case primitive.Tap:
			for i := range items {
				clonedRef, err := rt.CloneRef(items[i].ItemRef)
				if err != nil {
					return report, errkind.WithContext(err, string(node.Kind), node.Position, node.CallableName)
				}
				if _, err := rt.CallCallable(node.CallableRef, clonedRef, stateRef); err != nil {
					return report, errkind.WithContext(err, string(node.Kind), node.Position, node.CallableName)
				}
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
