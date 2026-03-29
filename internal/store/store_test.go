package store

import (
	"path/filepath"
	"testing"
)

func TestResolveWorkflowSelectorAcceptsUniqueIDPrefix(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	mustUpsertWorkflow(t, store, Workflow{
		ID:      "a1b2c3d4",
		Name:    "orders",
		Version: "latest",
		Path:    "/tmp/orders/lumn.lua",
		Status:  StatusActive,
	})

	workflow, found, err := store.ResolveWorkflowSelector("a1b2")
	if err != nil {
		t.Fatalf("resolve selector: %v", err)
	}
	if !found {
		t.Fatalf("expected workflow to be found")
	}
	if workflow.ID != "a1b2c3d4" {
		t.Fatalf("unexpected workflow: %+v", workflow)
	}
}

func TestResolveWorkflowSelectorRejectsAmbiguousIDPrefix(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	mustUpsertWorkflow(t, store, Workflow{
		ID:      "abc12345",
		Name:    "orders",
		Version: "latest",
		Path:    "/tmp/orders/lumn.lua",
		Status:  StatusActive,
	})
	mustUpsertWorkflow(t, store, Workflow{
		ID:      "abc67890",
		Name:    "billing",
		Version: "latest",
		Path:    "/tmp/billing/lumn.lua",
		Status:  StatusActive,
	})

	_, found, err := store.ResolveWorkflowSelector("abc")
	if err == nil {
		t.Fatalf("expected ambiguous prefix error")
	}
	if found {
		t.Fatalf("did not expect workflow match")
	}
}

func TestResolveWorkflowSelectorPrefersExactNameOverIDPrefix(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	mustUpsertWorkflow(t, store, Workflow{
		ID:      "abc12345",
		Name:    "abc",
		Version: "latest",
		Path:    "/tmp/abc/lumn.lua",
		Status:  StatusActive,
	})
	mustUpsertWorkflow(t, store, Workflow{
		ID:      "abc67890",
		Name:    "other",
		Version: "latest",
		Path:    "/tmp/other/lumn.lua",
		Status:  StatusActive,
	})

	workflow, found, err := store.ResolveWorkflowSelector("abc")
	if err != nil {
		t.Fatalf("resolve selector: %v", err)
	}
	if !found {
		t.Fatalf("expected workflow to be found")
	}
	if workflow.Name != "abc" {
		t.Fatalf("unexpected workflow: %+v", workflow)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := Open(filepath.Join(t.TempDir(), "lumn.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func mustUpsertWorkflow(t *testing.T, store *Store, workflow Workflow) {
	t.Helper()
	if err := store.UpsertWorkflow(workflow); err != nil {
		t.Fatalf("upsert workflow: %v", err)
	}
}
