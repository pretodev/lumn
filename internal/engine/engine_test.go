package engine

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateAllowsLocalRequire(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "require-local")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "helper.lua"), []byte(`
return {
  name = "helper",
  run = function(input, ctx)
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
    exec(helper),
  }
}
`), 0o644); err != nil {
		t.Fatalf("write init: %v", err)
	}

	if err := ValidateTarget(workflowDir, io.Discard); err != nil {
		t.Fatalf("validate target: %v", err)
	}
}

func TestRunTargetWithEmptySourceIsOK(t *testing.T) {
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
    exec(lumn.test_source({})),
  }
}
`), 0o644); err != nil {
		t.Fatalf("write init: %v", err)
	}

	report, code := RunTarget(workflowDir, io.Discard)
	if code != 0 {
		t.Fatalf("run code = %d, report = %+v", code, report)
	}
	if report.Status != "ok" || report.ItemsIn != 0 || report.ItemsOut != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}
