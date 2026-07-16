package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gtd/internal/domain"
	"gtd/internal/persistence/fs"
)

var cliPath string

func TestMain(m *testing.M) {
	// Build the CLI binary
	tmpDir, err := os.MkdirTemp("", "gtd-build-*")
	if err != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	cliPath = filepath.Join(tmpDir, "gtd")
	cmd := exec.Command("go", "build", "-o", cliPath, ".")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestCLI_Init(t *testing.T) {
	workspaceDir := t.TempDir()
	
	cmd := exec.Command(cliPath, "init")
	cmd.Env = append(os.Environ(), "GTD_DIR="+workspaceDir)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected init to succeed, got error: %v, output: %s", err, output)
	}

	// Verify JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("expected valid JSON, got error: %v, output: %s", err, output)
	}
	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("expected success to be true, got %v", result["success"])
	}

	// Verify files and directories are created
	expectedDirs := []string{
		"tasks",
		"projects",
		"areas",
		"people",
	}
	for _, d := range expectedDirs {
		if _, err := os.Stat(filepath.Join(workspaceDir, d)); os.IsNotExist(err) {
			t.Errorf("expected directory %s to be created", d)
		}
	}

	expectedFiles := []string{
		"config.yml",
		"index.db",
	}
	for _, f := range expectedFiles {
		if _, err := os.Stat(filepath.Join(workspaceDir, f)); os.IsNotExist(err) {
			t.Errorf("expected file %s to be created", f)
		}
	}
}

func TestCLI_ErrorFormatting(t *testing.T) {
	cmd := exec.Command(cliPath, "unknown-command")
	output, err := cmd.CombinedOutput()
	
	if err == nil {
		t.Fatal("expected unknown-command to fail with an exit code")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("expected valid JSON error, got error: %v, output: %s", err, output)
	}

	if success, ok := result["success"].(bool); !ok || success {
		t.Errorf("expected success to be false, got %v", result["success"])
	}

	if errData, ok := result["error"].(map[string]interface{}); !ok {
		t.Errorf("expected error object, got %v", result["error"])
	} else {
		if code, ok := errData["code"].(string); !ok || code != "ERR_UNKNOWN" {
			t.Errorf("expected code ERR_UNKNOWN, got %v", errData["code"])
		}
	}
}

func TestCLI_TaskAddAndList(t *testing.T) {
	workspaceDir := t.TempDir()
	env := append(os.Environ(), "GTD_DIR="+workspaceDir)

	initCmd := exec.Command(cliPath, "init")
	initCmd.Env = env
	initCmd.Run()

	cmdAdd := exec.Command(cliPath, "task", "add", "Basic task")
	cmdAdd.Env = env
	outputAdd, err := cmdAdd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected task add to succeed, got error: %v, output: %s", err, outputAdd)
	}

	var resultAdd map[string]interface{}
	json.Unmarshal(outputAdd, &resultAdd)
	dataMap := resultAdd["data"].(map[string]interface{})
	taskID := dataMap["id"].(string)

	if taskID == "" {
		t.Fatal("expected task ID to be present")
	}

	cmdList := exec.Command(cliPath, "task", "list", "inbox")
	cmdList.Env = env
	outputList, err := cmdList.CombinedOutput()
	if err != nil {
		t.Fatalf("expected task list to succeed, got error: %v, output: %s", err, outputList)
	}

	var resultList map[string]interface{}
	json.Unmarshal(outputList, &resultList)
	listData := resultList["data"].([]interface{})
	
	found := false
	for _, item := range listData {
		obj := item.(map[string]interface{})
		if obj["id"].(string) == taskID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find task ID %s in list", taskID)
	}
}

func TestCLI_TaskStructuredError(t *testing.T) {
	workspaceDir := t.TempDir()
	env := append(os.Environ(), "GTD_DIR="+workspaceDir)
	initCmd := exec.Command(cliPath, "init")
	initCmd.Env = env
	initCmd.Run()

	cmd := exec.Command(cliPath, "task", "update", "non-existent-uuid", "--status", "done")
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	
	if err == nil {
		t.Fatal("expected task update to fail")
	}

	var result map[string]interface{}
	json.Unmarshal(output, &result)

	if success, ok := result["success"].(bool); !ok || success {
		t.Errorf("expected success to be false")
	}

	if errData, ok := result["error"].(map[string]interface{}); !ok {
		t.Errorf("expected error object, got %v", result["error"])
	} else {
		if code, ok := errData["code"].(string); !ok || code != "ERR_NOT_FOUND" {
			t.Errorf("expected code ERR_NOT_FOUND, got %v", errData["code"])
		}
	}
}

func TestCLI_TaskProjectStall(t *testing.T) {
	workspaceDir := t.TempDir()
	env := append(os.Environ(), "GTD_DIR="+workspaceDir)

	initCmd := exec.Command(cliPath, "init")
	initCmd.Env = env
	initCmd.Run()

	projectID := "test-project-123"
	
	os.MkdirAll(filepath.Join(workspaceDir, "projects"), 0755)
	projRepo := fs.NewProjectRepository(filepath.Join(workspaceDir, "projects"))
	projRepo.Save(&domain.Project{
		ID:     projectID,
		Title:  "Test Project",
		Status: domain.ProjectStatusActive,
	})
	
	indexCmd := exec.Command(cliPath, "index", "rebuild")
	indexCmd.Env = env
	if out, err := indexCmd.CombinedOutput(); err != nil {
		t.Fatalf("index rebuild failed: %v, output: %s", err, out)
	}

	cmdAdd1 := exec.Command(cliPath, "task", "add", "Task 1")
	cmdAdd1.Env = env
	out1, _ := cmdAdd1.CombinedOutput()
	var res1 map[string]interface{}
	json.Unmarshal(out1, &res1)
	task1ID := res1["data"].(map[string]interface{})["id"].(string)

	cmdAdd2 := exec.Command(cliPath, "task", "add", "Task 2")
	cmdAdd2.Env = env
	out2, _ := cmdAdd2.CombinedOutput()
	var res2 map[string]interface{}
	json.Unmarshal(out2, &res2)
	task2ID := res2["data"].(map[string]interface{})["id"].(string)

	taskRepo := fs.NewTaskRepository(filepath.Join(workspaceDir, "tasks"))
	task1, _ := taskRepo.Get(task1ID)
	projectStr := "test-project-123"
	task1.ProjectID = &projectStr
	task1.Status = domain.TaskStatusNext
	taskRepo.Save(task1)

	task2, _ := taskRepo.Get(task2ID)
	task2.ProjectID = &projectStr
	task2.Status = domain.TaskStatusNext
	taskRepo.Save(task2)

	indexCmd2 := exec.Command(cliPath, "index", "rebuild")
	indexCmd2.Env = env
	if out, err := indexCmd2.CombinedOutput(); err != nil {
		t.Fatalf("index rebuild 2 failed: %v, output: %s", err, out)
	}

	cmdUpdate1 := exec.Command(cliPath, "task", "update", task1ID, "--status", "done")
	cmdUpdate1.Env = env
	outUp1, _ := cmdUpdate1.CombinedOutput()
	var resUp1 map[string]interface{}
	json.Unmarshal(outUp1, &resUp1)
	
	data1 := resUp1["data"].(map[string]interface{})
	if stalled, ok := data1["project_stalled"].(bool); ok && stalled {
		t.Fatal("expected project not to be stalled yet")
	}

	cmdUpdate2 := exec.Command(cliPath, "task", "update", task2ID, "--status", "done")
	cmdUpdate2.Env = env
	outUp2, _ := cmdUpdate2.CombinedOutput()
	var resUp2 map[string]interface{}
	json.Unmarshal(outUp2, &resUp2)

	data2 := resUp2["data"].(map[string]interface{})
	if stalled, ok := data2["project_stalled"].(bool); !ok || !stalled {
		t.Fatalf("expected project_stalled to be true, got output: %s", string(outUp2))
	}
}

func TestCLI_ProjectLifecycle(t *testing.T) {
	workspaceDir := t.TempDir()
	env := append(os.Environ(), "GTD_DIR="+workspaceDir)

	initCmd := exec.Command(cliPath, "init")
	initCmd.Env = env
	initCmd.Run()

	cmdAdd := exec.Command(cliPath, "project", "add", "New Project")
	cmdAdd.Env = env
	outAdd, err := cmdAdd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected project add to succeed, got error: %v, output: %s", err, outAdd)
	}

	var resAdd map[string]interface{}
	json.Unmarshal(outAdd, &resAdd)
	dataAdd := resAdd["data"].(map[string]interface{})
	projectID := dataAdd["id"].(string)

	if projectID == "" {
		t.Fatal("expected project ID to be present")
	}

	cmdList := exec.Command(cliPath, "project", "list")
	cmdList.Env = env
	outList, err := cmdList.CombinedOutput()
	if err != nil {
		t.Fatalf("expected project list to succeed, got error: %v, output: %s", err, outList)
	}

	var resList map[string]interface{}
	json.Unmarshal(outList, &resList)
	listData := resList["data"].([]interface{})
	
	found := false
	for _, item := range listData {
		obj := item.(map[string]interface{})
		if obj["id"].(string) == projectID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find project ID %s in list", projectID)
	}

	cmdDel := exec.Command(cliPath, "project", "delete", projectID)
	cmdDel.Env = env
	outDel, err := cmdDel.CombinedOutput()
	if err != nil {
		t.Fatalf("expected project delete to succeed, got error: %v, output: %s", err, outDel)
	}

	cmdList2 := exec.Command(cliPath, "project", "list")
	cmdList2.Env = env
	outList2, _ := cmdList2.CombinedOutput()
	
	var resList2 map[string]interface{}
	json.Unmarshal(outList2, &resList2)
	listData2 := resList2["data"].([]interface{})
	
	for _, item := range listData2 {
		obj := item.(map[string]interface{})
		if obj["id"].(string) == projectID {
			t.Errorf("expected project ID %s to be removed from list", projectID)
		}
	}
}

func TestCLI_Shortcuts(t *testing.T) {
	workspaceDir := t.TempDir()
	env := append(os.Environ(), "GTD_DIR="+workspaceDir)

	initCmd := exec.Command(cliPath, "init")
	initCmd.Env = env
	initCmd.Run()

	projectID := "test-stalled-project"
	
	os.MkdirAll(filepath.Join(workspaceDir, "projects"), 0755)
	projRepo := fs.NewProjectRepository(filepath.Join(workspaceDir, "projects"))
	projRepo.Save(&domain.Project{
		ID:     projectID,
		Title:  "Stalled Project",
		Status: domain.ProjectStatusActive,
	})
	
	indexCmd := exec.Command(cliPath, "index", "rebuild")
	indexCmd.Env = env
	if out, err := indexCmd.CombinedOutput(); err != nil {
		t.Fatalf("index rebuild failed: %v, output: %s", err, out)
	}

	cmdStalled := exec.Command(cliPath, "stalled")
	cmdStalled.Env = env
	outStalled, err := cmdStalled.CombinedOutput()
	if err != nil {
		t.Fatalf("expected stalled command to succeed, got error: %v, output: %s", err, outStalled)
	}

	var resStalled map[string]interface{}
	json.Unmarshal(outStalled, &resStalled)
	stalledData := resStalled["data"].([]interface{})
	
	foundStalled := false
	for _, item := range stalledData {
		obj := item.(map[string]interface{})
		if obj["id"].(string) == projectID {
			foundStalled = true
			break
		}
	}
	if !foundStalled {
		t.Errorf("expected to find stalled project ID %s in stalled list", projectID)
	}
}

// TestCLI_HelpText_NoSectionsOrSavedFilters locks R14: init/index help no longer
// mentions removed structural dirs.
func TestCLI_HelpText_NoSectionsOrSavedFilters(t *testing.T) {
	runHelp := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(cliPath, args...)
		out, err := cmd.CombinedOutput()
		// cobra --help exits 0
		if err != nil {
			t.Fatalf("help %v failed: %v, output: %s", args, err, out)
		}
		return string(out)
	}

	for _, args := range [][]string{
		{"init", "--help"},
		{"index", "rebuild", "--help"},
	} {
		text := strings.ToLower(runHelp(args...))
		if strings.Contains(text, "sections") || strings.Contains(text, "saved_filters") || strings.Contains(text, "saved filters") {
			t.Errorf("%v help still mentions sections/saved filters:\n%s", args, text)
		}
		for _, want := range []string{"tasks", "projects", "areas", "people"} {
			if !strings.Contains(text, want) {
				t.Errorf("%v help missing %q:\n%s", args, want, text)
			}
		}
	}
}

// TestCLI_NextShortcut_AcceptsAreaFilter locks R5: gtd next accepts the same
// filter flags as task list (and MCP gtd_get_next).
func TestCLI_NextShortcut_AcceptsAreaFilter(t *testing.T) {
	workspaceDir := t.TempDir()

	runCLIJSON := func(args ...string) map[string]interface{} {
		t.Helper()
		cmd := exec.Command(cliPath, args...)
		cmd.Env = append(os.Environ(), "GTD_DIR="+workspaceDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v, output: %s", args, err, out)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("invalid JSON for %v: %v, output: %s", args, err, out)
		}
		if success, ok := result["success"].(bool); !ok || !success {
			t.Fatalf("expected success for %v, got: %s", args, out)
		}
		return result
	}

	runCLIJSON("init")
	workArea := runCLIJSON("area", "add", "Work")
	workID := workArea["data"].(map[string]interface{})["id"].(string)
	runCLIJSON("area", "add", "Life")

	runCLIJSON("task", "add", "W !Work /next")
	runCLIJSON("task", "add", "L !Life /next")

	assertOnlyTitle := func(args []string, wantTitle string) {
		t.Helper()
		res := runCLIJSON(args...)
		items, ok := res["data"].([]interface{})
		if !ok {
			t.Fatalf("expected task list data for %v", args)
		}
		if len(items) != 1 {
			t.Fatalf("%v: expected 1 task, got %d (%v)", args, len(items), items)
		}
		title, _ := items[0].(map[string]interface{})["title"].(string)
		if title != wantTitle {
			t.Errorf("%v: expected title %q, got %q", args, wantTitle, title)
		}
	}

	assertOnlyTitle([]string{"next", "--area", "Work"}, "W")
	assertOnlyTitle([]string{"next", "--area-id", workID}, "W")
}
