package dag

import (
	"fmt"

	luaenv "github.com/pretodev/lumn/internal/lua"
	"github.com/pretodev/lumn/pkg/errkind"
	"github.com/pretodev/lumn/pkg/primitive"
	golua "github.com/speedata/go-lua"
)

type Workflow struct {
	ID      string
	Version string
	Nodes   []Node
	Runtime *luaenv.Runtime
}

type Node struct {
	Kind         primitive.Kind
	Position     int
	Ref          string
	FnRef        string
	CallableRef  string
	CallableName string
}

func Build(rt *luaenv.Runtime, workflowRef string) (*Workflow, error) {
	id, ok := rt.TableStringFieldRef(workflowRef, "id")
	if !ok || id == "" {
		return nil, errkind.New(errkind.ErrStructure, errkind.TypeStructure, `workflow field "id" must be a non-empty string`)
	}

	version, ok := rt.TableStringFieldRef(workflowRef, "version")
	if !ok || version == "" {
		return nil, errkind.New(errkind.ErrStructure, errkind.TypeStructure, `workflow field "version" must be a non-empty string`)
	}

	flowRef, ok := rt.TableRefFieldRef(workflowRef, "flow")
	if !ok {
		return nil, errkind.New(errkind.ErrStructure, errkind.TypeStructure, `workflow field "flow" must be a table`)
	}
	if rt.RefType(flowRef) != golua.TypeTable {
		return nil, errkind.New(errkind.ErrStructure, errkind.TypeStructure, `workflow field "flow" must be a table`)
	}

	nodes := make([]Node, 0, rt.TableLen(flowRef))
	for idx := 1; idx <= rt.TableLen(flowRef); idx++ {
		nodeRef := rt.ArrayValueRef(flowRef, idx)
		position := idx

		if rt.IsUnknownPrimitiveRef(nodeRef) {
			name, _ := rt.TableStringFieldRef(nodeRef, "name")
			return nil, &errkind.Error{
				Code:      errkind.ErrUnknownPrimitive,
				Type:      errkind.TypeUnknownPrimitive,
				Message:   fmt.Sprintf("unknown primitive %q in flow", name),
				Primitive: name,
				Position:  position,
			}
		}

		kindValue, ok := rt.TableKindRef(nodeRef)
		if !ok {
			return nil, &errkind.Error{
				Code:     errkind.ErrStructure,
				Type:     errkind.TypeStructure,
				Message:  fmt.Sprintf("flow position %d must be a primitive node", position),
				Position: position,
			}
		}

		node := Node{
			Kind:     primitive.Kind(kindValue),
			Position: position,
			Ref:      nodeRef,
		}

		if !node.Kind.Valid() {
			return nil, &errkind.Error{
				Code:      errkind.ErrUnknownPrimitive,
				Type:      errkind.TypeUnknownPrimitive,
				Message:   fmt.Sprintf("unknown primitive %q in flow", kindValue),
				Primitive: kindValue,
				Position:  position,
			}
		}

		switch node.Kind {
		case primitive.Exec:
			callableRef, ok := rt.TableRefFieldRef(nodeRef, "callable")
			if !ok {
				return nil, &errkind.Error{
					Code:      errkind.ErrCallableNotFound,
					Type:      errkind.TypeCallableNotFound,
					Message:   "exec requires a resolvable callable",
					Primitive: string(node.Kind),
					Position:  position,
				}
			}
			if rt.IsMissingSymbolRef(callableRef) {
				name, _ := rt.TableStringFieldRef(callableRef, "name")
				return nil, &errkind.Error{
					Code:      errkind.ErrCallableNotFound,
					Type:      errkind.TypeCallableNotFound,
					Message:   fmt.Sprintf("callable %q could not be resolved", name),
					Primitive: string(node.Kind),
					Position:  position,
					Callable:  name,
				}
			}
			if rt.RefType(callableRef) != golua.TypeTable {
				return nil, &errkind.Error{
					Code:      errkind.ErrInvalidSignature,
					Type:      errkind.TypeInvalidSignature,
					Message:   "exec expects a callable table",
					Primitive: string(node.Kind),
					Position:  position,
				}
			}
			callableName, ok := rt.TableStringFieldRef(callableRef, "name")
			if !ok || callableName == "" {
				return nil, &errkind.Error{
					Code:      errkind.ErrInvalidSignature,
					Type:      errkind.TypeInvalidSignature,
					Message:   "callable must define a non-empty name",
					Primitive: string(node.Kind),
					Position:  position,
				}
			}
			if rt.TableTypeFieldRef(callableRef, "run") != golua.TypeFunction {
				return nil, &errkind.Error{
					Code:      errkind.ErrInvalidSignature,
					Type:      errkind.TypeInvalidSignature,
					Message:   "callable must define a run function",
					Primitive: string(node.Kind),
					Position:  position,
					Callable:  callableName,
				}
			}
			node.CallableRef = callableRef
			node.CallableName = callableName
		case primitive.Set, primitive.Filter, primitive.Tap:
			fnRef, ok := rt.TableRefFieldRef(nodeRef, "fn")
			if !ok || rt.RefType(fnRef) != golua.TypeFunction {
				return nil, &errkind.Error{
					Code:      errkind.ErrInvalidSignature,
					Type:      errkind.TypeInvalidSignature,
					Message:   fmt.Sprintf("%s expects a function", node.Kind),
					Primitive: string(node.Kind),
					Position:  position,
				}
			}
			node.FnRef = fnRef
		}

		nodes = append(nodes, node)
	}

	return &Workflow{
		ID:      id,
		Version: version,
		Nodes:   nodes,
		Runtime: rt,
	}, nil
}
