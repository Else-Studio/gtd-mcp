package main_test

import (
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gtd/internal/persistence/fs"

	_ "github.com/ncruces/go-sqlite3/driver"
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

// TestE2E_TaskAddDoneSetsCompletedAt locks R6: NLP /done on create sets completedAt
// on the JSON payload and the markdown file (not SQL-only normalize).
func TestE2E_TaskAddDoneSetsCompletedAt(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")

	result := runCmdE2E(t, workspaceDir, "task", "add", "Finished already /done")
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected task data object")
	}
	if status, _ := data["status"].(string); status != "done" {
		t.Errorf("expected status done, got %q", status)
	}
	completedAt, _ := data["completedAt"].(string)
	if completedAt == "" {
		t.Errorf("expected data.completedAt non-null/non-empty")
	}
	taskID, _ := data["id"].(string)
	if taskID == "" {
		t.Fatalf("expected task id")
	}

	taskRepo := fs.NewTaskRepository(filepath.Join(workspaceDir, "tasks"))
	fileTask, err := taskRepo.Get(taskID)
	if err != nil {
		t.Fatalf("load task file: %v", err)
	}
	if fileTask.CompletedAt == nil {
		t.Errorf("expected tasks/%s.md frontmatter completedAt present", taskID)
	}

	// Dual-store: index row also has completedAt (R0 agreement).
	db, err := sql.Open("sqlite3", filepath.Join(workspaceDir, "index.db"))
	if err != nil {
		t.Fatalf("open index.db: %v", err)
	}
	defer db.Close()
	var sqlCompleted sql.NullString
	if err := db.QueryRow(`SELECT completedAt FROM tasks WHERE id = ?`, taskID).Scan(&sqlCompleted); err != nil {
		t.Fatalf("query index: %v", err)
	}
	if !sqlCompleted.Valid || sqlCompleted.String == "" {
		t.Errorf("expected index.db completedAt non-null")
	}
}

// TestE2E_TaskDelegateClear (R8): NLP text update without % keeps assignee;
// explicit --assigned-to "" clears it.
func TestE2E_TaskDelegateClear(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")

	taskResult := runCmdE2E(t, workspaceDir, "task", "add", "Delegate task %VH")
	taskOutput := taskResult["data"].(map[string]interface{})
	taskID := taskOutput["id"].(string)
	if assignedTo, _ := taskOutput["assignedTo"].(string); assignedTo != "VH" {
		t.Fatalf("expected assignee VH, got %q", assignedTo)
	}

	// Text update without % must preserve assignee.
	updateResult := runCmdE2E(t, workspaceDir, "task", "update", taskID, "Delegate task updated")
	taskObj := updateResult["data"].(map[string]interface{})["task"].(map[string]interface{})
	if assignedTo, _ := taskObj["assignedTo"].(string); assignedTo != "VH" {
		t.Errorf("expected assignee still VH after text update, got %q", assignedTo)
	}

	// Explicit clear via flag.
	clearResult := runCmdE2E(t, workspaceDir, "task", "update", taskID, "--assigned-to", "")
	cleared := clearResult["data"].(map[string]interface{})["task"].(map[string]interface{})
	if assignedTo, ok := cleared["assignedTo"].(string); ok && assignedTo != "" {
		t.Errorf("expected assignee cleared via --assigned-to \"\", got %q", assignedTo)
	}
}

// TestE2E_TaskUpdateDueDoesNotClearAssignee (R8): date-only NLP update preserves assignee.
func TestE2E_TaskUpdateDueDoesNotClearAssignee(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")

	taskResult := runCmdE2E(t, workspaceDir, "task", "add", "Call %Bob /waiting")
	taskID := taskResult["data"].(map[string]interface{})["id"].(string)

	updateResult := runCmdE2E(t, workspaceDir, "task", "update", taskID, "/due:2026-08-01")
	taskObj := updateResult["data"].(map[string]interface{})["task"].(map[string]interface{})
	if assignedTo, _ := taskObj["assignedTo"].(string); assignedTo != "Bob" {
		t.Errorf("expected assignedTo still Bob, got %q", assignedTo)
	}
	if due, _ := taskObj["dueDate"].(string); due == "" {
		t.Errorf("expected dueDate set after /due update")
	}
}

// TestE2E_TaskUpdate_ExplicitAreaClearsProject (R9): explicit --area moves task off project.
func TestE2E_TaskUpdate_ExplicitAreaClearsProject(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")

	runCmdE2E(t, workspaceDir, "area", "add", "Personal")
	taskResult := runCmdE2E(t, workspaceDir, "task", "add", `Step one +"Ship Feature" /next`)
	taskID := taskResult["data"].(map[string]interface{})["id"].(string)
	if _, ok := taskResult["data"].(map[string]interface{})["projectId"].(string); !ok {
		t.Fatalf("expected task linked to project")
	}

	updateResult := runCmdE2E(t, workspaceDir, "task", "update", taskID, "--area", "Personal")
	taskObj := updateResult["data"].(map[string]interface{})["task"].(map[string]interface{})
	if areaID, _ := taskObj["areaId"].(string); areaID == "" {
		t.Errorf("expected areaId set after --area Personal")
	}
	if proj, ok := taskObj["projectId"]; ok && proj != nil && proj != "" {
		t.Errorf("expected projectId cleared, got %v", proj)
	}
}

// TestE2E_TaskAdd_InvalidDateCommandWarning (R10): invalid /start: surfaces in response.
func TestE2E_TaskAdd_InvalidDateCommandWarning(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")

	result := runCmdE2E(t, workspaceDir, "task", "add", "Task /start:monx /due:tomorrow")
	data := result["data"].(map[string]interface{})
	if _, ok := data["startTime"]; ok && data["startTime"] != nil {
		t.Errorf("expected startTime absent, got %v", data["startTime"])
	}
	if due, _ := data["dueDate"].(string); due == "" {
		t.Errorf("expected dueDate present from /due:tomorrow")
	}

	found := false
	// Accept warnings or invalidDateCommands.
	if warnings, ok := data["warnings"].([]interface{}); ok {
		for _, w := range warnings {
			if s, ok := w.(string); ok && strings.Contains(s, "/start:monx") {
				found = true
			}
		}
	}
	if inv, ok := data["invalidDateCommands"].([]interface{}); ok {
		for _, w := range inv {
			if s, ok := w.(string); ok && strings.Contains(s, "/start:monx") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected /start:monx in warnings or invalidDateCommands, got %v", data)
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

func runCmdE2EFail(t *testing.T, workspaceDir string, args ...string) map[string]interface{} {
	t.Helper()
	cmd := exec.Command(cliPath, args...)
	cmd.Env = append(os.Environ(), "GTD_DIR="+workspaceDir)

	output, _ := cmd.CombinedOutput()
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("expected valid JSON for failed command %v, got error: %v, output: %s", args, err, output)
	}
	return result
}

func TestE2E_RecurrenceWorkflow(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")

	// Setup: Create a recurring task due today
	today := time.Now().Format("2006-01-02")
	addResult := runCmdE2E(t, workspaceDir, "task", "add", "Recurring task /due:"+today+" /recur:daily")
	taskData := addResult["data"].(map[string]interface{})
	taskID := taskData["id"].(string)

	// Action: Complete the task
	runCmdE2E(t, workspaceDir, "task", "update", taskID, "--status", "done")

	// Outcome: A new task should be created (spawned) in index and fs.
	// Since daily recurrence creates a new task due tomorrow, we should find it in task list.
	listResult := runCmdE2E(t, workspaceDir, "task", "list")
	listData := listResult["data"].([]interface{})

	foundNewTask := false
	for _, item := range listData {
		obj := item.(map[string]interface{})
		if obj["id"].(string) != taskID && strings.Contains(obj["title"].(string), "Recurring task") {
			foundNewTask = true
			dueStr, ok := obj["dueDate"].(string)
			if !ok || dueStr == "" {
				t.Errorf("expected spawned task to have a dueDate")
			} else {
				tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
				if !strings.HasPrefix(dueStr, tomorrow) {
					t.Errorf("expected spawned task due date to start with %s, got %s", tomorrow, dueStr)
				}
			}
		}
	}
	if !foundNewTask {
		t.Errorf("expected a new recurring task to be spawned")
	}
}

func TestE2E_JSONValidation_MissingArgs(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")

	// Action: Run 'gtd task add' with no args. It should fail validation.
	result := runCmdE2EFail(t, workspaceDir, "task", "add")

	// Outcome: JSON error response
	if success, ok := result["success"].(bool); ok && success {
		t.Errorf("expected success: false for missing title")
	}
	errObj, ok := result["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object, got %v", result)
	}
	if code, _ := errObj["code"].(string); code != "ERR_VALIDATION" {
		t.Errorf("expected error code ERR_VALIDATION, got %q", code)
	}
}

func TestE2E_EmptyRepositoryList(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")

	// Action: List tasks immediately when empty
	result := runCmdE2E(t, workspaceDir, "task", "list")

	// Outcome: Assert data contains empty array []
	data, ok := result["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data array, got %v", result["data"])
	}
	if len(data) != 0 {
		t.Errorf("expected empty array, got %d items", len(data))
	}
}

func TestE2E_SoftDelete_IndexSync(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")

	addRes := runCmdE2E(t, workspaceDir, "task", "add", "To delete")
	taskID := addRes["data"].(map[string]interface{})["id"].(string)

	// Action: Delete task
	runCmdE2E(t, workspaceDir, "task", "delete", taskID)

	// Outcome: Check SQLite index directly to verify deletedAt is set
	db, err := sql.Open("sqlite3", filepath.Join(workspaceDir, "index.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var deletedAt sql.NullString
	err = db.QueryRow("SELECT deletedAt FROM tasks WHERE id = ?", taskID).Scan(&deletedAt)
	if err != nil {
		t.Fatalf("query task row: %v", err)
	}
	if !deletedAt.Valid || deletedAt.String == "" {
		t.Errorf("expected deletedAt to be set in SQLite index for task %s", taskID)
	}

	// Verify task list does not return it by default
	listRes := runCmdE2E(t, workspaceDir, "task", "list")
	listData := listRes["data"].([]interface{})
	for _, item := range listData {
		obj := item.(map[string]interface{})
		if obj["id"].(string) == taskID {
			t.Errorf("deleted task %s should not appear in default list", taskID)
		}
	}
}
