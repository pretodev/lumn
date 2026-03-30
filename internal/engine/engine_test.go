package engine

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTargetUsesDefaultLumnLua(t *testing.T) {
	root := t.TempDir()
	writeWorkflowFile(t, root, "lumn.lua", `
return {
  flow = {}
}
`)

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})

	target, err := ResolveTarget("")
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if filepath.Base(target.EntryPath) != "lumn.lua" {
		t.Fatalf("entry path = %q", target.EntryPath)
	}
	if target.Name != filepath.Base(root) {
		t.Fatalf("name = %q, want %q", target.Name, filepath.Base(root))
	}
}

func TestResolveTargetPrefersInitOverLumn(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflow")
	writeWorkflowFile(t, workflowDir, "init.lua", `
return {
  flow = {}
}
`)
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {}
}
`)

	target, err := ResolveTarget(workflowDir)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if filepath.Base(target.EntryPath) != "init.lua" {
		t.Fatalf("entry path = %q, want init.lua", target.EntryPath)
	}
}

func TestResolveLocalSelectorFallsBackToLuaFile(t *testing.T) {
	root := t.TempDir()
	writeWorkflowFile(t, root, "orders.lua", `
return {
  flow = {}
}
`)

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})

	target, err := ResolveLocalSelector("orders")
	if err != nil {
		t.Fatalf("resolve local selector: %v", err)
	}
	if filepath.Base(target.EntryPath) != "orders.lua" {
		t.Fatalf("entry path = %q", target.EntryPath)
	}
	if target.Name != "orders" {
		t.Fatalf("name = %q", target.Name)
	}
}

func TestValidateAllowsLocalRequireStandalone(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "require-local")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "helper.lua"), []byte(`
return {
  name = "helper",
  run = function(input)
    return input
  end
}
`), 0o644); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
local helper = require("helper")

return {
  flow = {
    call {
      exec = helper,
      on_data = function(result)
        return result
      end,
    },
  }
}
`)

	target, err := ResolveTarget(workflowDir)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if target.WorkspaceDir != root {
		t.Fatalf("workspace dir = %q, want %q", target.WorkspaceDir, root)
	}

	if err := ValidateTarget(workflowDir, io.Discard); err != nil {
		t.Fatalf("validate target: %v", err)
	}
}

func TestValidateAllowsSharedRequireFromWorkspaceRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lumn.lock"), []byte("lock"), 0o644); err != nil {
		t.Fatalf("write lumn.lock: %v", err)
	}
	sharedDir := filepath.Join(root, "_shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("mkdir shared: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "helper.lua"), []byte(`
return {
  name = "helper",
  run = function(input)
    return input
  end
}
`), 0o644); err != nil {
		t.Fatalf("write helper: %v", err)
	}

	workflowDir := filepath.Join(root, "workflows", "require-shared")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
local helper = require("helper")

return {
  flow = {
    call {
      exec = helper,
      on_data = function(result)
        return result
      end,
    },
  }
}
`)

	target, err := ResolveTarget(workflowDir)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if target.WorkspaceDir != root {
		t.Fatalf("workspace dir = %q, want %q", target.WorkspaceDir, root)
	}

	if err := ValidateTarget(workflowDir, io.Discard); err != nil {
		t.Fatalf("validate target: %v", err)
	}
}

func TestResolveTargetInfersWorkspaceFromConfigFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lumn.config.staging.lua"), []byte("return {}"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	workflowDir := filepath.Join(root, "nested", "workflow")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {}
}
`)

	target, err := ResolveTarget(workflowDir)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if target.WorkspaceDir != root {
		t.Fatalf("workspace dir = %q, want %q", target.WorkspaceDir, root)
	}
}

func TestRunTargetWithEmptySourceIsEmpty(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "empty-source")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.from({}),
      on_data = function(result)
        return result
      end,
    },
  }
}
`)

	report, code := RunTarget(workflowDir, io.Discard)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v", code, report)
	}
	if report.Workflow != "empty-source" || report.Version != "latest" {
		t.Fatalf("unexpected metadata: %+v", report)
	}
	if report.Status != "empty" || report.ItemsIn != 0 || report.ItemsOut != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestRunTargetWithEmptyFlowIsOK(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "empty-flow")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {}
}
`)

	report, code := RunTarget(workflowDir, io.Discard)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v", code, report)
	}
	if report.Status != "ok" || report.ItemsIn != 0 || report.ItemsOut != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestValidateAndRunTapOnlyWorkflow(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "tap-only")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    tap {
      exec = {
        name = "hello-world",
        run = function(item)
          if item ~= nil then
            error("expected nil item")
          end
          print("hello world")
        end
      }
    },
  }
}
`)

	if err := ValidateTarget(workflowDir, io.Discard); err != nil {
		t.Fatalf("validate target: %v", err)
	}

	var stderr bytes.Buffer
	report, code := RunTarget(workflowDir, &stderr)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v, stderr = %q", code, report, stderr.String())
	}
	if report.Status != "empty" || report.ItemsIn != 0 || report.ItemsOut != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !strings.Contains(stderr.String(), "hello world") {
		t.Fatalf("expected tap output on stderr, got %q", stderr.String())
	}
}

func TestRunTargetCallWithoutOnDataUsesRawArrayItems(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "raw-array")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.from({
        { id = 1, active = true },
        { id = 2, active = false },
      }),
    },
    filter {
      condition = function(item)
        return item.active
      end,
    },
  }
}
`)

	report, code := RunTarget(workflowDir, io.Discard)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v", code, report)
	}
	if report.Status != "ok" || report.ItemsIn != 2 || report.ItemsOut != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestRunTargetCallWithoutOnDataAcceptsSingleObject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "single-object")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = {
        name = "single-object",
        run = function()
          return {
            id = 7,
            kind = "object",
          }
        end,
      },
    },
    filter {
      condition = function(item)
        return item.id == 7 and item.kind == "object"
      end,
    },
  }
}
`)

	report, code := RunTarget(workflowDir, io.Discard)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v", code, report)
	}
	if report.Status != "ok" || report.ItemsIn != 1 || report.ItemsOut != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestRunTargetCallWithoutOnDataAcceptsScalar(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "single-scalar")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = {
        name = "single-scalar",
        run = function()
          return "hello"
        end,
      },
    },
    tap {
      exec = {
        name = "print-item",
        run = function(item)
          print("item:" .. item)
        end,
      },
    },
  }
}
`)

	var stderr bytes.Buffer
	report, code := RunTarget(workflowDir, &stderr)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v, stderr = %q", code, report, stderr.String())
	}
	if report.Status != "ok" || report.ItemsIn != 1 || report.ItemsOut != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !strings.Contains(stderr.String(), "item:hello") {
		t.Fatalf("expected scalar item on stderr, got %q", stderr.String())
	}
}

func TestRunTargetTapThenCallThenTapUsesEmptyStateSemantics(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "tap-call-tap")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    tap {
      exec = {
        name = "before-call",
        run = function(item)
          if item == nil then
            print("before:nil")
            return
          end
          error("expected nil item before call")
        end,
      },
    },
    call {
      exec = lumn.from({
        { id = 1 },
        { id = 2 },
      }),
    },
    tap {
      exec = {
        name = "after-call",
        run = function(item)
          print("after:" .. item.id)
        end,
      },
    },
  }
}
`)

	var stderr bytes.Buffer
	report, code := RunTarget(workflowDir, &stderr)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v, stderr = %q", code, report, stderr.String())
	}
	if report.Status != "ok" || report.ItemsIn != 2 || report.ItemsOut != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !strings.Contains(stderr.String(), "before:nil") || !strings.Contains(stderr.String(), "after:1") || !strings.Contains(stderr.String(), "after:2") {
		t.Fatalf("unexpected tap output: %q", stderr.String())
	}
}

func TestRunTargetContinuesAfterEmptyBatchUntilNextCall(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "empty-then-call")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.from({}),
    },
    set {
      to = function(item)
        error("set should not run")
      end,
    },
    filter {
      condition = function(item)
        error("filter should not run")
      end,
    },
    call {
      exec = lumn.from({
        { id = 1 },
      }),
    },
  }
}
`)

	report, code := RunTarget(workflowDir, io.Discard)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v", code, report)
	}
	if report.Status != "ok" || report.ItemsIn != 1 || report.ItemsOut != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestRunTargetUsesLastCallBatchForItemsIn(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "last-call-wins")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.from({
        { id = 1 },
      }),
    },
    call {
      exec = lumn.from({
        { id = 2 },
        { id = 3 },
      }),
    },
    filter {
      condition = function(item)
        return item.id == 3
      end,
    },
  }
}
`)

	report, code := RunTarget(workflowDir, io.Discard)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v", code, report)
	}
	if report.Status != "ok" || report.ItemsIn != 2 || report.ItemsOut != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestRunTargetWithOptionsExposesTriggerData(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "trigger-data")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.from({
        { id = 1 },
      }),
      on_data = function(result)
        return result
      end,
    },
    tap {
      exec = {
        name = "print-trigger",
        run = function(item)
          local trigger = lumn.trigger_data()
          print(trigger.type .. ":" .. trigger.path)
        end
      }
    },
  }
}
`)

	var stderr bytes.Buffer
	report, code := RunTargetWithOptions(workflowDir, &stderr, RunOptions{
		WorkflowName: "custom-name",
		Version:      "1.2",
		TriggerData: map[string]any{
			"type": "webhook",
			"path": "/hooks/test",
		},
	})
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v, stderr = %q", code, report, stderr.String())
	}
	if report.Workflow != "custom-name" || report.Version != "1.2" {
		t.Fatalf("unexpected metadata: %+v", report)
	}
	if report.Status != "ok" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !strings.Contains(stderr.String(), "webhook:/hooks/test") {
		t.Fatalf("expected trigger data on stderr, got %q", stderr.String())
	}
}

func TestLegacyIDVersionFieldsAreIgnored(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "legacy")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  id = "legacy-id",
  version = "9.9.9",
  flow = {
    call {
      exec = lumn.from({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
  }
}
`)

	report, code := RunTarget(workflowDir, io.Discard)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v", code, report)
	}
	if report.Workflow != "legacy" || report.Version != "latest" {
		t.Fatalf("legacy fields leaked into report: %+v", report)
	}
}

func writeWorkflowFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
}
