package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func runCmdE2E(t *testing.T, workspaceDir string, args ...string) map[string]interface{} {
	t.Helper()
	cmd := exec.Command(cliPath, args...)
	cmd.Env = append(os.Environ(), "GTD_DIR="+workspaceDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected command %v to succeed, got error: %v, output: %s", args, err, output)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("expected valid JSON, got error: %v, output: %s", err, output)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		t.Fatalf("expected success: true for command %v, got output: %s", args, output)
	}

	return result
}

func TestE2E_TaskProjectLifecycle(t *testing.T) {
	workspaceDir := t.TempDir()

	// 1. Initialize workspace
	runCmdE2E(t, workspaceDir, "init")

	// 2. Create a project
	projResult := runCmdE2E(t, workspaceDir, "project", "add", "Build E2E Tests")
	projData, ok := projResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected project data")
	}
	// Note: project response format might just be the project directly or { "project": ... }
	// We'll assume the project object is at the top level or inside "project"
	
	// Wait, formatOutputData passes the project as-is, so data is the project object.
	_, ok = projData["id"].(string)
	if !ok {
		t.Fatalf("expected project id to be string")
	}

	// 3. Create a task with warnings (Due before Created)
	taskResult := runCmdE2E(t, workspaceDir, "task", "add", "Write e2e_test.go /due:2000-01-01")
	taskOutput, ok := taskResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected task object in data")
	}
	taskID, ok := taskOutput["id"].(string)
	if !ok {
		t.Fatalf("expected task id to be string")
	}
	
	warnings, ok := taskOutput["warnings"].([]interface{})
	if !ok || len(warnings) == 0 {
		t.Errorf("expected warnings array, got %v", taskOutput["warnings"])
	} else if warnings[0].(string) != "due_before_created" {
		t.Errorf("expected due_before_created warning, got %v", warnings[0])
	}

	// 4. Assign task to project
	runCmdE2E(t, workspaceDir, "task", "update", taskID, `+"Build E2E Tests"`)

	// 5. Complete the task (this should trigger project_stalled)
	finalResult := runCmdE2E(t, workspaceDir, "task", "update", taskID, "--status", "done")
	finalData, ok := finalResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected final task data")
	}

	stalled, ok := finalData["project_stalled"].(bool)
	if !ok || !stalled {
		t.Errorf("expected project_stalled: true in the output, got %v", finalData["project_stalled"])
	}
}

func TestE2E_PlainOutput(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")
	runCmdE2E(t, workspaceDir, "task", "add", "Sample task")

	cmd := exec.Command(cliPath, "task", "list", "--plain")
	cmd.Env = append(os.Environ(), "GTD_DIR="+workspaceDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected command to succeed, got error: %v, output: %s", err, output)
	}
	
	outStr := string(output)
	if outStr[0] == '{' {
		t.Errorf("expected plain text output, got JSON-like: %s", outStr)
	}
	if !strings.Contains(outStr, "TITLE") || !strings.Contains(outStr, "Sample task") {
		t.Errorf("expected table output to contain 'TITLE' and 'Sample task', got: %s", outStr)
	}
}

func TestE2E_ComprehensiveFlow(t *testing.T) {
	workspaceDir := t.TempDir()

	// 1. Init
	runCmdE2E(t, workspaceDir, "init")

	// 2. Add projects
	runCmdE2E(t, workspaceDir, "project", "add", "Home Renovation")
	runCmdE2E(t, workspaceDir, "project", "add", "Learn Go")

	// 3. Add tasks with different syntax
	runCmdE2E(t, workspaceDir, "task", "add", `Buy paint +"Home Renovation" @errand #weekend /next /due:2000-01-01`)
	runCmdE2E(t, workspaceDir, "task", "add", `Read Go tutorials +"Learn Go" @computer /next`)
	runCmdE2E(t, workspaceDir, "task", "add", "General inbox idea without tags")
	
	// Create a task to be deleted
	toDeleteRes := runCmdE2E(t, workspaceDir, "task", "add", "Mistake task")
	toDeleteID := toDeleteRes["data"].(map[string]interface{})["id"].(string)

	// 4. Test list shortcuts
	inboxRes := runCmdE2E(t, workspaceDir, "inbox")
	inboxIDs := inboxRes["data"].([]interface{})
	if len(inboxIDs) != 2 {
		t.Errorf("expected 2 tasks in inbox (general and mistake), got %v", len(inboxIDs))
	}

	agendaRes := runCmdE2E(t, workspaceDir, "agenda")
	agendaIDs := agendaRes["data"].([]interface{})
	if len(agendaIDs) != 1 {
		t.Errorf("expected 1 task in agenda (due past), got %v", len(agendaIDs))
	}

	// 5. Test Delete and Restore
	runCmdE2E(t, workspaceDir, "task", "delete", toDeleteID)
	// After delete, it should not be in standard list
	listRes := runCmdE2E(t, workspaceDir, "task", "list")
	listIDs := listRes["data"].([]interface{})
	for _, item := range listIDs {
		obj := item.(map[string]interface{})
		if obj["id"].(string) == toDeleteID {
			t.Errorf("expected deleted task to be absent from list")
		}
	}

	runCmdE2E(t, workspaceDir, "task", "restore", toDeleteID)
	// Now it should be back
	listRes2 := runCmdE2E(t, workspaceDir, "task", "list")
	found := false
	for _, item := range listRes2["data"].([]interface{}) {
		obj := item.(map[string]interface{})
		if obj["id"].(string) == toDeleteID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected restored task to be in list")
	}

	// 6. Test project list
	projListRes := runCmdE2E(t, workspaceDir, "project", "list")
	projIDs := projListRes["data"].([]interface{})
	if len(projIDs) != 2 {
		t.Errorf("expected 2 projects, got %v", len(projIDs))
	}
}

func TestE2E_TaskDelegateClear(t *testing.T) {
	workspaceDir := t.TempDir()

	// 1. Initialize workspace
	runCmdE2E(t, workspaceDir, "init")

	// 2. Create a task with a delegate
	taskResult := runCmdE2E(t, workspaceDir, "task", "add", "Delegate task %VH")
	taskOutput, ok := taskResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected task object in data")
	}
	taskID, ok := taskOutput["id"].(string)
	if !ok {
		t.Fatalf("expected task id to be string")
	}
	assignedTo, _ := taskOutput["assignedTo"].(string)
	if assignedTo != "VH" {
		t.Errorf("expected assignee to be 'VH', got %q", assignedTo)
	}

	// 3. Update task omitting the % symbol
	updateResult := runCmdE2E(t, workspaceDir, "task", "update", taskID, "Delegate task updated")
	updateOutput, ok := updateResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected task object in data")
	}

	taskObj, ok := updateOutput["task"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected task object in update data")
	}
	assignedTo, ok = taskObj["assignedTo"].(string)
	if ok && assignedTo != "" {
		t.Errorf("expected assignee to be cleared, got %q", assignedTo)
	}
}

func TestE2E_AutoCreateProjectAndArea(t *testing.T) {
	workspaceDir := t.TempDir()

	// 1. Initialize workspace
	runCmdE2E(t, workspaceDir, "init")

	// 2. Add task with a new project using +
	taskResult := runCmdE2E(t, workspaceDir, "task", "add", `Task with new project +"Brand New Project"`)
	taskOutput, ok := taskResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected task object in data")
	}
	projectID, ok := taskOutput["projectId"].(string)
	if !ok || projectID == "" {
		t.Fatalf("expected task to have a valid projectId assigned")
	}

	// Verify project was actually created
	projListRes := runCmdE2E(t, workspaceDir, "project", "list")
	projDataList, ok := projListRes["data"].([]interface{})
	if !ok {
		t.Fatalf("expected project data list")
	}
	foundProject := false
	for _, p := range projDataList {
		projMap := p.(map[string]interface{})
		if projMap["id"].(string) == projectID {
			foundProject = true
			if projMap["title"].(string) != "Brand New Project" {
				t.Errorf("expected project title 'Brand New Project', got %q", projMap["title"])
			}
			if projMap["status"].(string) != "active" {
				t.Errorf("expected project status 'active', got %q", projMap["status"])
			}
		}
	}
	if !foundProject {
		t.Errorf("expected new project to be found in project list")
	}

	// 3. Add task with a new area using !
	taskResult2 := runCmdE2E(t, workspaceDir, "task", "add", `Task with new area !"Brand New Area"`)
	taskOutput2, ok := taskResult2["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected task object in data")
	}
	areaID, ok := taskOutput2["areaId"].(string)
	if !ok || areaID == "" {
		t.Fatalf("expected task to have a valid areaId assigned")
	}

	// Verify area was actually created
	areaListRes := runCmdE2E(t, workspaceDir, "area", "list")
	areaDataList, ok := areaListRes["data"].([]interface{})
	if !ok {
		t.Fatalf("expected area data list")
	}
	foundArea := false
	for _, a := range areaDataList {
		areaMap := a.(map[string]interface{})
		if areaMap["id"].(string) == areaID {
			foundArea = true
			if areaMap["name"].(string) != "Brand New Area" {
				t.Errorf("expected area name 'Brand New Area', got %q", areaMap["name"])
			}
		}
	}
	if !foundArea {
		t.Errorf("expected new area to be found in area list")
	}
}


