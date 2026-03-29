package lua

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	golua "github.com/speedata/go-lua"
)

func TestCloneRefPreservesNestedArrays(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflow")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}

	rt, err := NewRuntime(workflowDir, root, io.Discard)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	defer rt.Close()

	if err := golua.DoString(rt.State, `
value = {
  tags = { "a", "b", "c" },
  nested = {
    numbers = { 1, 2, 3 },
  },
}
`); err != nil {
		t.Fatalf("define value: %v", err)
	}

	rt.State.Global("value")
	valueRef := rt.StoreRef(-1)
	rt.State.Pop(1)

	clonedRef, err := rt.CloneRef(valueRef)
	if err != nil {
		t.Fatalf("clone ref: %v", err)
	}

	rt.PushRef(clonedRef)
	rt.State.SetGlobal("cloned")

	if err := golua.DoString(rt.State, `
assert(#cloned.tags == 3)
assert(cloned.tags[1] == "a")
assert(cloned.tags[2] == "b")
assert(cloned.tags[3] == "c")
assert(#cloned.nested.numbers == 3)
assert(cloned.nested.numbers[1] == 1)
assert(cloned.nested.numbers[2] == 2)
assert(cloned.nested.numbers[3] == 3)
`); err != nil {
		t.Fatalf("assert cloned arrays: %v", err)
	}
}

func TestTriggerDataOutsideExecutionReturnsNoneAndIsIndependent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflow")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}

	rt, err := NewRuntime(workflowDir, root, io.Discard)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	defer rt.Close()

	if err := golua.DoString(rt.State, `
first = lumn.trigger_data()
first.type = "mutated"
second = lumn.trigger_data()
assert(first.type == "mutated")
assert(second.type == "none")
`); err != nil {
		t.Fatalf("assert trigger data default: %v", err)
	}
}
