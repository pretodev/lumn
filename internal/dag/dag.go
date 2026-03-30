package dag

import (
	"fmt"

	luaenv "github.com/pretodev/lumn/internal/lua"
	"github.com/pretodev/lumn/pkg/errkind"
	"github.com/pretodev/lumn/pkg/primitive"
	golua "github.com/speedata/go-lua"
)

type Workflow struct {
	Nodes   []Node
	Runtime *luaenv.Runtime
}

type Node struct {
	Kind         primitive.Kind
	Position     int
	Ref          string
	FnRef        string
	OnDataRef    string
	CallableRef  string
	CallableName string
}

func Build(rt *luaenv.Runtime, workflowRef string) (*Workflow, error) {
	flowRef, ok := rt.TableRefFieldRef(workflowRef, "flow")
	if !ok || rt.RefType(flowRef) != golua.TypeTable {
		return nil, errkind.New(errkind.ErrStructure, errkind.TypeStructure, `workflow field "flow" must be a table`)
	}

	flowLen := rt.TableLen(flowRef)
	nodes := make([]Node, 0, flowLen)
	for idx := 1; idx <= flowLen; idx++ {
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
		case primitive.Call:
			if err := bindCallable(rt, &node, "exec"); err != nil {
				return nil, err
			}
			onDataRef, ok := rt.TableRefFieldRef(nodeRef, "on_data")
			if ok && rt.RefType(onDataRef) != golua.TypeFunction {
				return nil, &errkind.Error{
					Code:      errkind.ErrInvalidSignature,
					Type:      errkind.TypeInvalidSignature,
					Message:   `call expects a function in field "on_data"`,
					Primitive: string(node.Kind),
					Position:  position,
				}
			}
			if ok {
				node.OnDataRef = onDataRef
			}
		case primitive.Tap:
			if err := bindCallable(rt, &node, "exec"); err != nil {
				return nil, err
			}
		case primitive.Set:
			fnRef, ok := rt.TableRefFieldRef(nodeRef, "to")
			if !ok || rt.RefType(fnRef) != golua.TypeFunction {
				return nil, &errkind.Error{
					Code:      errkind.ErrInvalidSignature,
					Type:      errkind.TypeInvalidSignature,
					Message:   `set expects a function in field "to"`,
					Primitive: string(node.Kind),
					Position:  position,
				}
			}
			node.FnRef = fnRef
		case primitive.Filter:
			fnRef, ok := rt.TableRefFieldRef(nodeRef, "condition")
			if !ok || rt.RefType(fnRef) != golua.TypeFunction {
				return nil, &errkind.Error{
					Code:      errkind.ErrInvalidSignature,
					Type:      errkind.TypeInvalidSignature,
					Message:   `filter expects a function in field "condition"`,
					Primitive: string(node.Kind),
					Position:  position,
				}
			}
			node.FnRef = fnRef
		}

		nodes = append(nodes, node)
	}

	return &Workflow{
		Nodes:   nodes,
		Runtime: rt,
	}, nil
}

func bindCallable(rt *luaenv.Runtime, node *Node, field string) error {
	callableRef, ok := rt.TableRefFieldRef(node.Ref, field)
	if !ok {
		return &errkind.Error{
			Code:      errkind.ErrInvalidSignature,
			Type:      errkind.TypeInvalidSignature,
			Message:   fmt.Sprintf(`%s expects a callable in field %q`, node.Kind, field),
			Primitive: string(node.Kind),
			Position:  node.Position,
		}
	}
	if rt.IsMissingSymbolRef(callableRef) {
		name, _ := rt.TableStringFieldRef(callableRef, "name")
		return &errkind.Error{
			Code:      errkind.ErrCallableNotFound,
			Type:      errkind.TypeCallableNotFound,
			Message:   fmt.Sprintf("callable %q could not be resolved", name),
			Primitive: string(node.Kind),
			Position:  node.Position,
			Callable:  name,
		}
	}
	if rt.RefType(callableRef) != golua.TypeTable {
		return &errkind.Error{
			Code:      errkind.ErrInvalidSignature,
			Type:      errkind.TypeInvalidSignature,
			Message:   fmt.Sprintf(`%s expects a callable table in field %q`, node.Kind, field),
			Primitive: string(node.Kind),
			Position:  node.Position,
		}
	}

	callableName, ok := rt.TableStringFieldRef(callableRef, "name")
	if !ok || callableName == "" {
		return &errkind.Error{
			Code:      errkind.ErrInvalidSignature,
			Type:      errkind.TypeInvalidSignature,
			Message:   "callable must define a non-empty name",
			Primitive: string(node.Kind),
			Position:  node.Position,
		}
	}
	if rt.TableTypeFieldRef(callableRef, "run") != golua.TypeFunction {
		return &errkind.Error{
			Code:      errkind.ErrInvalidSignature,
			Type:      errkind.TypeInvalidSignature,
			Message:   "callable must define a run function",
			Primitive: string(node.Kind),
			Position:  node.Position,
			Callable:  callableName,
		}
	}

	node.CallableRef = callableRef
	node.CallableName = callableName
	return nil
}
