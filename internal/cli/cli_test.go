package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pretodev/lumn/internal/daemonapi"
	"github.com/pretodev/lumn/internal/executor"
)

func TestValidateAndRunUseDefaultLumnLua(t *testing.T) {
	root := t.TempDir()
	writeWorkflowFile(t, root, "lumn.lua", `
local items = {
  { id = 1, nome = "Item A", valor = 100 },
  { id = 2, nome = "Item B", valor = 50 },
}

local log_item = {
  name = "log_item",
  run = function(input)
    print(input.nome .. " aprovado")
  end
}

return {
  flow = {
    call {
      exec = lumn.test_source(items),
      on_data = function(result)
        return result
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

	withWorkingDir(t, root, func() {
		code, stdout, stderr := runCLI(t, "validate")
		if code != 0 {
			t.Fatalf("validate exit = %d, stderr = %q", code, stderr)
		}
		if stdout != "" || stderr != "" {
			t.Fatalf("validate should be quiet, stdout=%q stderr=%q", stdout, stderr)
		}

		code, stdout, stderr = runCLI(t, "run")
		if code != 0 {
			t.Fatalf("run exit = %d, stderr = %q", code, stderr)
		}

		report := decodeReport(t, stdout)
		if report.Workflow != filepath.Base(root) || report.Version != "latest" {
			t.Fatalf("unexpected metadata: %+v", report)
		}
		if report.Status != "ok" || report.ItemsIn != 2 || report.ItemsOut != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
		if !strings.Contains(stderr, "Item A aprovado") {
			t.Fatalf("expected print output on stderr, got %q", stderr)
		}
	})
}

func TestRunWithFolderResolutionPrefersInitOverLumn(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "priority")
	writeWorkflowFile(t, workflowDir, "init.lua", `
return {
  flow = {
    call {
      exec = lumn.test_source({ { id = 1 }, { id = 2 } }),
      on_data = function(result)
        return result
      end,
    },
  }
}
`)
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.test_source({}),
      on_data = function(result)
        return result
      end,
    },
  }
}
`)

	code, stdout, stderr := runCLI(t, "run", "-f", workflowDir)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr = %q", code, stderr)
	}
	report := decodeReport(t, stdout)
	if report.ItemsIn != 2 {
		t.Fatalf("expected init.lua to win, got %+v", report)
	}
}

func TestRunWithFolderFallsBackToLumn(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workflowDir := filepath.Join(root, "fallback")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.test_source({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
  }
}
`)

	code, stdout, stderr := runCLI(t, "run", "-f", workflowDir)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr = %q", code, stderr)
	}
	report := decodeReport(t, stdout)
	if report.ItemsIn != 1 || report.Workflow != "fallback" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestRunFallsBackToLocalSelectorWhenDaemonIsUnavailable(t *testing.T) {
	root := t.TempDir()
	workflowDir := filepath.Join(root, "orders")
	writeWorkflowFile(t, workflowDir, "lumn.lua", `
return {
  flow = {
    call {
      exec = lumn.test_source({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
  }
}
`)

	withWorkingDir(t, root, func() {
		code, stdout, stderr := runCLI(t, "run", "orders")
		if code != 0 {
			t.Fatalf("run exit = %d, stderr = %q", code, stderr)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		report := decodeReport(t, stdout)
		if report.Workflow != "orders" || report.Version != "latest" {
			t.Fatalf("unexpected report: %+v", report)
		}
	})
}

func TestStartListWatchAndLogsRequireDaemon(t *testing.T) {
	root := t.TempDir()
	writeWorkflowFile(t, root, "lumn.lua", `
return {
  flow = {}
}
`)

	withWorkingDir(t, root, func() {
		tests := [][]string{
			{"start", "cancelamentos"},
			{"list"},
			{"watch"},
			{"logs"},
		}

		for _, args := range tests {
			code, _, stderr := runCLI(t, args...)
			if code != 1 {
				t.Fatalf("%v exit = %d, want 1", args, code)
			}
			if !strings.Contains(stderr, "daemon is not running") {
				t.Fatalf("%v stderr = %q", args, stderr)
			}
		}
	})
}

func TestRemovedCommandsAreUnavailable(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"init", "pedidos"},
		{"exec", "pedidos"},
		{"status"},
	} {
		code, _, stderr := runCLI(t, args...)
		if code != 1 {
			t.Fatalf("%v exit = %d, want 1", args, code)
		}
		if !strings.Contains(stderr, "unknown command") || !strings.Contains(stderr, "Workflow commands:") {
			t.Fatalf("%v stderr = %q", args, stderr)
		}
	}
}

func TestMainHelpIsStructuredAndInEnglish(t *testing.T) {
	t.Parallel()

	code, stdout, stderr := runCLI(t, "help")
	if code != 0 {
		t.Fatalf("help exit = %d, stderr = %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Workflow commands:") || !strings.Contains(stdout, "Selectors:") || !strings.Contains(stdout, "Entrypoint resolution:") {
		t.Fatalf("unexpected help output: %q", stdout)
	}
}

func TestCommandHelpIsAvailableViaHelpFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args    []string
		matches []string
	}{
		{
			args:    []string{"run", "--help"},
			matches: []string{"lumn run", "Usage:", "Selectors:", "Examples:"},
		},
		{
			args:    []string{"logs", "--help"},
			matches: []string{"lumn logs", "Flags:", "Status:"},
		},
		{
			args:    []string{"help", "daemon", "start"},
			matches: []string{"lumn daemon start", "Behavior:"},
		},
	}

	for _, tc := range tests {
		code, stdout, stderr := runCLI(t, tc.args...)
		if code != 0 {
			t.Fatalf("%v exit = %d, stderr = %q", tc.args, code, stderr)
		}
		if stderr != "" {
			t.Fatalf("%v stderr = %q", tc.args, stderr)
		}
		for _, match := range tc.matches {
			if !strings.Contains(stdout, match) {
				t.Fatalf("%v expected %q in output %q", tc.args, match, stdout)
			}
		}
	}
}

func TestSelectWorkflowIDAcceptsUniquePrefix(t *testing.T) {
	t.Parallel()

	workflows := []daemonapi.WorkflowResponse{
		{ID: "9bc30fff", Name: "workflows", Version: "latest"},
	}

	resolved, found, err := selectWorkflowID(workflows, "9b")
	if err != nil {
		t.Fatalf("select workflow id: %v", err)
	}
	if !found {
		t.Fatalf("expected workflow to be found")
	}
	if resolved != "9bc30fff" {
		t.Fatalf("resolved id = %q", resolved)
	}
}

func TestSelectWorkflowIDRejectsAmbiguousPrefix(t *testing.T) {
	t.Parallel()

	workflows := []daemonapi.WorkflowResponse{
		{ID: "9bc30fff", Name: "one", Version: "latest"},
		{ID: "9bd40aaa", Name: "two", Version: "latest"},
	}

	_, found, err := selectWorkflowID(workflows, "9b")
	if err == nil {
		t.Fatalf("expected ambiguous prefix error")
	}
	if found {
		t.Fatalf("did not expect workflow to be found")
	}
}

func TestSelectWorkflowIDPrefersNameBeforePrefix(t *testing.T) {
	t.Parallel()

	workflows := []daemonapi.WorkflowResponse{
		{ID: "9bc30fff", Name: "9b", Version: "latest"},
		{ID: "9bd40aaa", Name: "other", Version: "latest"},
	}

	resolved, found, err := selectWorkflowID(workflows, "9b")
	if err != nil {
		t.Fatalf("select workflow id: %v", err)
	}
	if !found {
		t.Fatalf("expected workflow to be found")
	}
	if resolved != "9bc30fff" {
		t.Fatalf("resolved id = %q", resolved)
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
      exec = lumn.test_source({ { id = 1 } }),
      on_data = function(result)
        return result
      end,
    },
  }
}
`)

	code, stdout, stderr := runCLI(t, "run", "-f", workflowDir)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr = %q", code, stderr)
	}
	report := decodeReport(t, stdout)
	if report.Workflow != "legacy" || report.Version != "latest" {
		t.Fatalf("legacy metadata leaked into report: %+v", report)
	}
}

func TestCommandExitCodes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeWorkflow := func(name, body string) string {
		dir := filepath.Join(root, name)
		writeWorkflowFile(t, dir, "lumn.lua", body)
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
			args:       []string{"validate", "-f", writeWorkflow("syntax", `return {`)},
			wantCode:   2,
			wantStderr: "syntax",
		},
		{
			name: "structure",
			args: []string{"validate", "-f", writeWorkflow("structure", `
return {
  flow = 1
}
`)},
			wantCode:   3,
			wantStderr: `"flow"`,
		},
		{
			name: "sandbox",
			args: []string{"validate", "-f", writeWorkflow("sandbox", `
os.execute("echo nope")

return {
  flow = {}
}
`)},
			wantCode:   6,
			wantStderr: `blocked`,
		},
		{
			name: "runtime",
			args: []string{"run", "-f", writeWorkflow("runtime", `
return {
  flow = {
    call {
      exec = lumn.test_source({ { id = 1, valor = 10 } }),
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
			args:       []string{"validate", "-f", filepath.Join(root, "missing")},
			wantCode:   8,
			wantStderr: `not found`,
		},
		{
			name: "callable not found",
			args: []string{"validate", "-f", writeWorkflow("missing-callable", `
return {
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

func writeWorkflowFile(t *testing.T, dir, fileName, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(previous)
	}()

	fn()
}

func decodeReport(t *testing.T, raw string) executor.Report {
	t.Helper()

	var report executor.Report
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatalf("decode report: %v; raw=%q", err, raw)
	}
	return report
}
