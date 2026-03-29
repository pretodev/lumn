package engine

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pretodev/lumn/internal/executor"
)

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
	if err := os.WriteFile(filepath.Join(workflowDir, "init.lua"), []byte(`
local helper = require("helper")

return {
  id = "require-local",
  version = "1.0.0",
  flow = {
    call {
      exec = helper,
      on_data = function(result)
        return result
      end,
    },
  }
}
`), 0o644); err != nil {
		t.Fatalf("write init: %v", err)
	}

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
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "init.lua"), []byte(`
local helper = require("helper")

return {
  id = "require-shared",
  version = "1.0.0",
  flow = {
    call {
      exec = helper,
      on_data = function(result)
        return result
      end,
    },
  }
}
`), 0o644); err != nil {
		t.Fatalf("write init: %v", err)
	}

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
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "init.lua"), []byte(`
return {
  id = "nested-workflow",
  version = "1.0.0",
  flow = {}
}
`), 0o644); err != nil {
		t.Fatalf("write init: %v", err)
	}

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
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "init.lua"), []byte(`
return {
  id = "empty-source",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source({}),
      on_data = function(result)
        return result
      end,
    },
  }
}
`), 0o644); err != nil {
		t.Fatalf("write init: %v", err)
	}

	report, code := RunTarget(workflowDir, io.Discard)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v", code, report)
	}
	if report.Status != "empty" || report.ItemsIn != 0 || report.ItemsOut != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestRunTargetWithOptionsExposesTriggerData(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "trigger-data")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "init.lua"), []byte(`
return {
  id = "trigger-data",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source({
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
`), 0o644); err != nil {
		t.Fatalf("write init: %v", err)
	}

	var stderr bytes.Buffer
	report, code := RunTargetWithOptions(workflowDir, &stderr, executor.RunOptions{
		TriggerData: map[string]any{
			"type": "webhook",
			"path": "/hooks/test",
		},
	})
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v, stderr = %q", code, report, stderr.String())
	}
	if report.Status != "ok" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !strings.Contains(stderr.String(), "webhook:/hooks/test") {
		t.Fatalf("expected trigger data on stderr, got %q", stderr.String())
	}
}
