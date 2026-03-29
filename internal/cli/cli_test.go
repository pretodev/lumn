package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pretodev/lumn/internal/executor"
)

func TestInitAndValidateScaffold(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "meu-workflow")

	code, stdout, stderr := runCLI(t, "init", target)
	if code != 0 {
		t.Fatalf("init exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "init.lua") {
		t.Fatalf("expected init stdout to mention init.lua, got %q", stdout)
	}

	if _, err := os.Stat(filepath.Join(target, "init.lua")); err != nil {
		t.Fatalf("expected scaffolded init.lua: %v", err)
	}

	code, stdout, stderr = runCLI(t, "validate", target)
	if code != 0 {
		t.Fatalf("validate exit = %d, stderr = %q", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("validate should be quiet on success, got stdout %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("validate should be quiet on success, got stderr %q", stderr)
	}
}

func TestRunSuccessJSONAndPrintStderr(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "pedidos")
	mustWriteWorkflow(t, workflowDir, `
local items = {
  { id = 1, nome = "Item A", valor = 100 },
  { id = 2, nome = "Item B", valor = 50 },
  { id = 3, nome = "Item C", valor = 200 },
}

local log_item = {
  name = "log_item",
  run = function(input)
    print(input.nome .. " aprovado")
  end
}

return {
  id = "pedidos",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source(items),
      on_data = function(result)
        return result
      end,
    },
    set {
      to = function(item)
        lumn.set("ultimo_item_id", item.id)
        item.ultimo_item_id = lumn.get("ultimo_item_id")
        item.processado = true
        return item
      end,
    },
    filter {
      condition = function(item)
        return item.valor > 80
      end,
    },
    tap {
      exec = log_item,
    },
  }
}
`)

	code, stdout, stderr := runCLI(t, "run", workflowDir)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr = %q", code, stderr)
	}

	report := decodeReport(t, stdout)
	if report.Status != "ok" {
		t.Fatalf("expected status ok, got %+v", report)
	}
	if report.ItemsIn != 3 || report.ItemsOut != 2 {
		t.Fatalf("unexpected item counts: %+v", report)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("expected no errors, got %+v", report.Errors)
	}
	if strings.Contains(stdout, "aprovado") {
		t.Fatalf("expected stdout to contain JSON only, got %q", stdout)
	}
	if !strings.Contains(stderr, "Item A aprovado") || !strings.Contains(stderr, "Item C aprovado") {
		t.Fatalf("expected tap print output on stderr, got %q", stderr)
	}
}

func TestRunUsesWorkflowStateAcrossSteps(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "stateful")
	mustWriteWorkflow(t, workflowDir, `
local items = {
  { id = 1, valor = 10 },
  { id = 2, valor = 20 },
}

return {
  id = "stateful",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source(items),
      on_data = function(result)
        return result
      end,
    },
    set {
      to = function(item)
        lumn.set("threshold", 15)
        return item
      end,
    },
    filter {
      condition = function(item)
        return item.valor >= lumn.get("threshold")
      end,
    },
  }
}
`)

	code, stdout, stderr := runCLI(t, "run", workflowDir)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr = %q stdout = %q", code, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	report := decodeReport(t, stdout)
	if report.Status != "ok" || report.ItemsIn != 2 || report.ItemsOut != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestRunEmptyStatusWhenFilterRemovesAllItems(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "empty-after-filter")
	mustWriteWorkflow(t, workflowDir, `
local items = {
  { id = 1, valor = 10 },
}

return {
  id = "empty-after-filter",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source(items),
      on_data = function(result)
        return result
      end,
    },
    filter {
      condition = function(item)
        return item.valor > 999
      end,
    },
  }
}
`)

	code, stdout, stderr := runCLI(t, "run", workflowDir)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr = %q stdout = %q", code, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	report := decodeReport(t, stdout)
	if report.Status != "empty" || report.ItemsIn != 1 || report.ItemsOut != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestCommandExitCodes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeWorkflow := func(name, body string) string {
		dir := filepath.Join(root, name)
		mustWriteWorkflow(t, dir, body)
		return dir
	}

	tests := []struct {
		name         string
		args         []string
		wantCode     int
		wantStdout   string
		wantStderr   string
		validateJSON bool
	}{
		{
			name:       "syntax",
			args:       []string{"validate", writeWorkflow("syntax", `return {`)},
			wantCode:   2,
			wantStderr: "syntax",
		},
		{
			name: "structure",
			args: []string{"validate", writeWorkflow("structure", `
return {
  version = "1.0.0",
  flow = {}
}
`)},
			wantCode:   3,
			wantStderr: `"id"`,
		},
		{
			name: "old exec syntax rejected",
			args: []string{"validate", writeWorkflow("old-exec", `
local items = {
  { id = 1 },
}

return {
  id = "old-exec",
  version = "1.0.0",
  flow = {
    exec(lumn.test_source(items)),
  }
}
`)},
			wantCode:   4,
			wantStderr: `unknown primitive`,
		},
		{
			name: "flow must start with call",
			args: []string{"validate", writeWorkflow("must-start-with-call", `
return {
  id = "must-start-with-call",
  version = "1.0.0",
  flow = {
    set {
      to = function(item)
        return item
      end,
    },
  }
}
`)},
			wantCode:   3,
			wantStderr: `must start with call`,
		},
		{
			name: "call missing on_data",
			args: []string{"validate", writeWorkflow("call-missing-on-data", `
return {
  id = "call-missing-on-data",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source({}),
    },
  }
}
`)},
			wantCode:   5,
			wantStderr: `"on_data"`,
		},
		{
			name: "set missing to",
			args: []string{"validate", writeWorkflow("set-missing-to", `
return {
  id = "set-missing-to",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source({}),
      on_data = function(result)
        return result
      end,
    },
    set {},
  }
}
`)},
			wantCode:   5,
			wantStderr: `"to"`,
		},
		{
			name: "filter missing condition",
			args: []string{"validate", writeWorkflow("filter-missing-condition", `
return {
  id = "filter-missing-condition",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source({}),
      on_data = function(result)
        return result
      end,
    },
    filter {},
  }
}
`)},
			wantCode:   5,
			wantStderr: `"condition"`,
		},
		{
			name: "tap missing exec",
			args: []string{"validate", writeWorkflow("tap-missing-exec", `
return {
  id = "tap-missing-exec",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source({}),
      on_data = function(result)
        return result
      end,
    },
    tap {},
  }
}
`)},
			wantCode:   5,
			wantStderr: `"exec"`,
		},
		{
			name: "invalid callable signature",
			args: []string{"validate", writeWorkflow("invalid-signature", `
return {
  id = "invalid-signature",
  version = "1.0.0",
  flow = {
    call {
      exec = { name = "bad" },
      on_data = function(result)
        return result
      end,
    },
  }
}
`)},
			wantCode:   5,
			wantStderr: `run function`,
		},
		{
			name: "sandbox",
			args: []string{"validate", writeWorkflow("sandbox", `
os.execute("echo nope")

return {
  id = "sandbox",
  version = "1.0.0",
  flow = {}
}
`)},
			wantCode:   6,
			wantStderr: `blocked`,
		},
		{
			name: "runtime",
			args: []string{"run", writeWorkflow("runtime", `
local items = {
  { id = 1, valor = 10 },
}

return {
  id = "runtime",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source(items),
      on_data = function(result)
        return result
      end,
    },
    set {
      to = function(item)
        return nil
      end,
    },
  }
}
`)},
			wantCode:     7,
			validateJSON: true,
		},
		{
			name:       "workflow not found",
			args:       []string{"validate", filepath.Join(root, "missing")},
			wantCode:   8,
			wantStderr: `not found`,
		},
		{
			name: "callable not found",
			args: []string{"validate", writeWorkflow("missing-callable", `
return {
  id = "missing-callable",
  version = "1.0.0",
  flow = {
    call {
      exec = fonte_que_nao_existe,
      on_data = function(result)
        return result
      end,
    },
  }
}
`)},
			wantCode:   9,
			wantStderr: `could not be resolved`,
		},
		{
			name: "require traversal blocked",
			args: []string{"validate", writeWorkflow("require-traversal", `
require("../fora")

return {
  id = "require-traversal",
  version = "1.0.0",
  flow = {}
}
`)},
			wantCode:   6,
			wantStderr: `outside the workflow sandbox`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			code, stdout, stderr := runCLI(t, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d, stdout = %q, stderr = %q", code, tc.wantCode, stdout, stderr)
			}

			if tc.validateJSON {
				report := decodeReport(t, stdout)
				if report.Status != "error" {
					t.Fatalf("expected error report, got %+v", report)
				}
				if len(report.Errors) == 0 {
					t.Fatalf("expected structured error in JSON report")
				}
				return
			}

			if tc.wantStdout != "" && !strings.Contains(stdout, tc.wantStdout) {
				t.Fatalf("expected stdout to contain %q, got %q", tc.wantStdout, stdout)
			}
			if tc.wantStderr != "" && !strings.Contains(stderr, tc.wantStderr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantStderr, stderr)
			}
		})
	}
}

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func mustWriteWorkflow(t *testing.T, dir, body string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
}

func decodeReport(t *testing.T, raw string) executor.Report {
	t.Helper()

	var report executor.Report
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatalf("decode report: %v; raw=%q", err, raw)
	}
	return report
}
