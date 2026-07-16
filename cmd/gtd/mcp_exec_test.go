package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var (
	mcpBinOnce sync.Once
	mcpBinPath string
	mcpBinErr  error
)

func ensureMCPBinary(t *testing.T) string {
	t.Helper()
	mcpBinOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "gtd-mcp-build-*")
		if err != nil {
			mcpBinErr = err
			return
		}
		// Intentionally not removed: process lifetime only; tests need a stable path.
		bin := filepath.Join(tmpDir, "gtd")
		cmd := exec.Command("go", "build", "-o", bin, ".")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			mcpBinErr = err
			return
		}
		mcpBinPath = bin
		gtdExecutable = bin
	})
	if mcpBinErr != nil {
		t.Fatalf("build gtd binary: %v", mcpBinErr)
	}
	return mcpBinPath
}

func runMCPCLI(t *testing.T, workspace string, args ...string) map[string]interface{} {
	t.Helper()
	bin := ensureMCPBinary(t)
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "GTD_DIR="+workspace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cli %v failed: %v\n%s", args, err, out)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON for %v: %v\n%s", args, err, out)
	}
	if ok, _ := result["success"].(bool); !ok {
		t.Fatalf("expected success for %v: %s", args, out)
	}
	return result
}

func withGTDDir(t *testing.T, workspace string, fn func()) {
	t.Helper()
	prev, had := os.LookupEnv("GTD_DIR")
	os.Setenv("GTD_DIR", workspace)
	defer func() {
		if had {
			os.Setenv("GTD_DIR", prev)
		} else {
			os.Unsetenv("GTD_DIR")
		}
	}()
	ensureMCPBinary(t)
	fn()
}

func TestExecuteCLI_EmptyProjectIDArgPreserved(t *testing.T) {
	ws := t.TempDir()
	runMCPCLI(t, ws, "init")
	add := runMCPCLI(t, ws, "task", "add", "Has project +TempProj /next")
	task := add["data"].(map[string]interface{})
	id, _ := task["id"].(string)
	if id == "" {
		t.Fatalf("missing task id: %#v", task)
	}

	withGTDDir(t, ws, func() {
		res, err := executeCLI("task", "update", id, "--project-id", "")
		if err != nil {
			t.Fatalf("executeCLI: %v", err)
		}
		if res.IsError {
			t.Fatalf("clear project failed: %s", toolResultText(res))
		}
		text := toolResultText(res)
		var envelope map[string]interface{}
		if err := json.Unmarshal([]byte(text), &envelope); err != nil {
			t.Fatalf("json: %v %s", err, text)
		}
		data := envelope["data"].(map[string]interface{})
		if nested, ok := data["task"].(map[string]interface{}); ok {
			data = nested
		}
		if data["projectId"] != nil {
			t.Errorf("expected projectId cleared, got %#v", data["projectId"])
		}
	})
}

func TestMCP_TaskListWaitingAndFilters(t *testing.T) {
	ws := t.TempDir()
	runMCPCLI(t, ws, "init")
	runMCPCLI(t, ws, "area", "add", "Work")
	runMCPCLI(t, ws, "task", "add", "Wait on Bob %Bob !Work /waiting")
	runMCPCLI(t, ws, "task", "add", "Do it !Work /next")

	withGTDDir(t, ws, func() {
		res, err := executeCLI("task", "list", "waiting")
		if err != nil || res.IsError {
			t.Fatalf("list waiting: err=%v res=%s", err, toolResultText(res))
		}
		n, err := countJSONDataArray(toolResultText(res))
		if err != nil || n != 1 {
			t.Fatalf("waiting count=%d err=%v body=%s", n, err, toolResultText(res))
		}

		args := appendTaskFilters([]string{"task", "list", "next"}, map[string]interface{}{"area": "Work"})
		res, err = executeCLI(args...)
		if err != nil || res.IsError {
			t.Fatalf("list next area: %v %s", err, toolResultText(res))
		}
		n, err = countJSONDataArray(toolResultText(res))
		if err != nil || n != 1 {
			t.Fatalf("next+area count=%d err=%v", n, err)
		}
	})
}

func TestMCP_ProjectUpdateAreaWithoutStatus(t *testing.T) {
	ws := t.TempDir()
	runMCPCLI(t, ws, "init")
	area := runMCPCLI(t, ws, "area", "add", "Life")
	areaID := area["data"].(map[string]interface{})["id"].(string)
	proj := runMCPCLI(t, ws, "project", "add", "Solo")
	projID := proj["data"].(map[string]interface{})["id"].(string)

	withGTDDir(t, ws, func() {
		args := []string{"project", "update", projID}
		args = appendFlagIfPresent(args, map[string]interface{}{"area_id": areaID}, "area_id", "--area-id")
		res, err := executeCLI(args...)
		if err != nil || res.IsError {
			t.Fatalf("project update area: %v %s", err, toolResultText(res))
		}
		var envelope map[string]interface{}
		if err := json.Unmarshal([]byte(toolResultText(res)), &envelope); err != nil {
			t.Fatal(err)
		}
		data := envelope["data"].(map[string]interface{})
		got, _ := data["areaId"].(string)
		if got != areaID {
			t.Errorf("areaId=%q want %q", got, areaID)
		}
	})
}

func TestMCP_TaskUpdateRecurrence(t *testing.T) {
	ws := t.TempDir()
	runMCPCLI(t, ws, "init")
	add := runMCPCLI(t, ws, "task", "add", "Daily habit /next")
	id := add["data"].(map[string]interface{})["id"].(string)

	withGTDDir(t, ws, func() {
		args := []string{"task", "update", id}
		args = appendFlagIfPresent(args, map[string]interface{}{"recurrence": `{"rule":"daily"}`}, "recurrence", "--recurrence")
		res, err := executeCLI(args...)
		if err != nil || res.IsError {
			t.Fatalf("recurrence: %v %s", err, toolResultText(res))
		}
		var envelope map[string]interface{}
		_ = json.Unmarshal([]byte(toolResultText(res)), &envelope)
		data := envelope["data"].(map[string]interface{})
		if nested, ok := data["task"].(map[string]interface{}); ok {
			data = nested
		}
		rec, _ := data["recurrence"].(map[string]interface{})
		if rec == nil || rec["rule"] != "daily" {
			t.Errorf("expected daily recurrence, got %#v", data["recurrence"])
		}
	})
}

func TestMCP_IndexRebuild(t *testing.T) {
	ws := t.TempDir()
	runMCPCLI(t, ws, "init")
	runMCPCLI(t, ws, "task", "add", "Something")
	withGTDDir(t, ws, func() {
		res, err := executeCLI("index", "rebuild")
		if err != nil || res.IsError {
			t.Fatalf("index rebuild: %v %s", err, toolResultText(res))
		}
	})
}

func TestBuildStateJSON_Counts(t *testing.T) {
	ws := t.TempDir()
	runMCPCLI(t, ws, "init")
	runMCPCLI(t, ws, "task", "add", "In inbox")
	runMCPCLI(t, ws, "task", "add", "Next one /next")
	runMCPCLI(t, ws, "task", "add", "Wait /waiting")
	runMCPCLI(t, ws, "task", "add", "Maybe /someday")
	runMCPCLI(t, ws, "project", "add", "Stuck Project")

	withGTDDir(t, ws, func() {
		raw := buildStateJSON()
		var st map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &st); err != nil {
			t.Fatalf("state json: %v %s", err, raw)
		}
		if ok, _ := st["workspace_ok"].(bool); !ok {
			t.Fatalf("workspace_ok false: %s", raw)
		}
		assertCount := func(key string, want float64) {
			t.Helper()
			got, ok := st[key].(float64)
			if !ok || got != want {
				t.Errorf("%s=%v want %v (state=%s)", key, st[key], want, raw)
			}
		}
		assertCount("inbox_count", 1)
		assertCount("next_count", 1)
		assertCount("waiting_count", 1)
		assertCount("someday_count", 1)
		assertCount("stalled_project_count", 1)
	})
}

func TestMCP_AgendaNextNameFilters(t *testing.T) {
	ws := t.TempDir()
	runMCPCLI(t, ws, "init")
	runMCPCLI(t, ws, "area", "add", "Work")
	runMCPCLI(t, ws, "area", "add", "Life")
	runMCPCLI(t, ws, "task", "add", "W !Work /next")
	runMCPCLI(t, ws, "task", "add", "L !Life /next")

	withGTDDir(t, ws, func() {
		args := appendTaskFilters([]string{"next"}, map[string]interface{}{"area": "Work"})
		res, err := executeCLI(args...)
		if err != nil || res.IsError {
			t.Fatalf("%v %s", err, toolResultText(res))
		}
		n, err := countJSONDataArray(toolResultText(res))
		if err != nil || n != 1 {
			t.Fatalf("next --area Work count=%d err=%v body=%s", n, err, toolResultText(res))
		}
	})
}
