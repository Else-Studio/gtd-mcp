package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// runCmdWithEnv executes a CLI command with the given arguments, returning stdout and err.
func runCmdWithEnv(workspaceDir string, args ...string) ([]byte, error) {
	cmd := exec.Command(cliPath, args...)
	cmd.Env = append(os.Environ(), "GTD_DIR="+workspaceDir)
	return cmd.CombinedOutput()
}

// runCmdJSON executes the command and expects it to return a JSON success response.
func runCmdJSON(t *testing.T, workspaceDir string, args ...string) map[string]interface{} {
	t.Helper()
	output, err := runCmdWithEnv(workspaceDir, args...)
	if err != nil {
		t.Fatalf("expected command %v to succeed, got error: %v, output: %s", args, err, output)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("expected valid JSON for command %v, got error: %v, output: %s", args, err, output)
	}

	success, ok := result["success"].(bool)
	if !ok || !success {
		t.Fatalf("expected success: true for command %v, got output: %s", args, output)
	}

	return result
}

// toSlice safely casts a JSON array interface{} to a slice of interface{}.
func toSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	if s, ok := v.([]interface{}); ok {
		return s
	}
	return nil
}

// timeMatchesDate checks if a timestamp string matches the YYYY-MM-DD date.
func timeMatchesDate(timeStr, dateStr string) bool {
	if timeStr == "" {
		return false
	}
	// Parse ISO timestamp
	parsed, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		// Try date-only format
		parsed, err = time.Parse("2006-01-02", timeStr)
		if err != nil {
			return false
		}
	}
	return parsed.Format("2006-01-02") == dateStr
}

// TestE2E_Tester_Bootstrapping verifies system bootstrapping.
func TestE2E_Tester_Bootstrapping(t *testing.T) {
	wsDir := t.TempDir()

	// 1. Initialize workspace
	res := runCmdJSON(t, wsDir, "init")
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map in init response")
	}
	if msg, _ := data["message"].(string); msg != "Workspace initialized" {
		t.Errorf("unexpected success message: %s", msg)
	}

	// 2. Verify folder structures
	expectedDirs := []string{"tasks", "projects", "areas", "people"}
	for _, dir := range expectedDirs {
		p := filepath.Join(wsDir, dir)
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			t.Errorf("expected directory %s to exist", p)
		}
	}

	// 3. Verify config.yml and index.db exist
	for _, file := range []string{"config.yml", "index.db"} {
		p := filepath.Join(wsDir, file)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s to exist", p)
		}
	}

	// 4. Run index rebuild
	rebuildRes := runCmdJSON(t, wsDir, "index", "rebuild")
	rebuildData, ok := rebuildRes["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map in rebuild response")
	}
	if msg, _ := rebuildData["message"].(string); msg != "Index rebuilt successfully" {
		t.Errorf("unexpected rebuild message: %s", msg)
	}
}

// TestE2E_Tester_HighLevelCRUD verifies Area and People CRUD, strictly asserting on update subcommands.
func TestE2E_Tester_HighLevelCRUD(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Add Area
	areaRes := runCmdJSON(t, wsDir, "area", "add", "Work")
	areaData := areaRes["data"].(map[string]interface{})
	areaID, _ := areaData["id"].(string)
	if areaID == "" {
		t.Fatalf("expected non-empty area ID")
	}

	// 2. Update Area (strictly expected to succeed)
	updateAreaRes := runCmdJSON(t, wsDir, "area", "update", areaID, "--name", "Professional Work")
	updateAreaData := updateAreaRes["data"].(map[string]interface{})
	if name, _ := updateAreaData["name"].(string); name != "Professional Work" {
		t.Errorf("expected area name to be updated to 'Professional Work', got %q", name)
	}

	// Verify update in list
	listRes := runCmdJSON(t, wsDir, "area", "list")
	listData := toSlice(listRes["data"])
	found := false
	for _, item := range listData {
		m := item.(map[string]interface{})
		if m["id"].(string) == areaID && m["name"].(string) == "Professional Work" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected to find updated area 'Professional Work' in list")
	}

	// 3. Delete Area
	runCmdJSON(t, wsDir, "area", "delete", areaID)
	listResAfterDelete := runCmdJSON(t, wsDir, "area", "list")
	listDataAfterDelete := toSlice(listResAfterDelete["data"])
	for _, item := range listDataAfterDelete {
		m := item.(map[string]interface{})
		if m["id"].(string) == areaID {
			t.Errorf("expected deleted area to be excluded from active list")
		}
	}

	// 4. Add Person
	peopleRes := runCmdJSON(t, wsDir, "people", "add", "Bob")
	peopleData := peopleRes["data"].(map[string]interface{})
	personID, _ := peopleData["id"].(string)
	if personID == "" {
		t.Fatalf("expected non-empty person ID")
	}

	// 5. Update Person (strictly expected to succeed)
	updatePeopleRes := runCmdJSON(t, wsDir, "people", "update", personID, "--name", "Robert")
	updatePeopleData := updatePeopleRes["data"].(map[string]interface{})
	if name, _ := updatePeopleData["name"].(string); name != "Robert" {
		t.Errorf("expected person name to be updated to 'Robert', got %q", name)
	}

	// Verify update in list
	listResPeople := runCmdJSON(t, wsDir, "people", "list")
	listDataPeople := toSlice(listResPeople["data"])
	foundPeople := false
	for _, item := range listDataPeople {
		m := item.(map[string]interface{})
		if m["id"].(string) == personID && m["name"].(string) == "Robert" {
			foundPeople = true
		}
	}
	if !foundPeople {
		t.Errorf("expected to find updated person 'Robert' in list")
	}

	// 6. Delete Person
	runCmdJSON(t, wsDir, "people", "delete", personID)
	listResPeopleAfterDelete := runCmdJSON(t, wsDir, "people", "list")
	listDataPeopleAfterDelete := toSlice(listResPeopleAfterDelete["data"])
	for _, item := range listDataPeopleAfterDelete {
		m := item.(map[string]interface{})
		if m["id"].(string) == personID {
			t.Errorf("expected deleted person to be excluded from active list")
		}
	}
}

// TestE2E_Tester_TaskProjectAssociation verifies explicit flags vs NLP for tasks and projects.
func TestE2E_Tester_TaskProjectAssociation(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Add Area
	areaRes := runCmdJSON(t, wsDir, "area", "add", "Personal")
	areaData := areaRes["data"].(map[string]interface{})
	areaID := areaData["id"].(string)

	// 2. Add Project with explicit Area flag (strictly expected to succeed)
	projRes := runCmdJSON(t, wsDir, "project", "add", "Fix Kitchen Sink", "--area-id", areaID)
	projData := projRes["data"].(map[string]interface{})
	projectID := projData["id"].(string)
	if aID, _ := projData["areaId"].(string); aID != areaID {
		t.Errorf("expected project area ID to be %s, got %v", areaID, projData["areaId"])
	}

	// 3. Add Task with explicit project-id and assigned-to flags (strictly expected to succeed)
	taskRes := runCmdJSON(t, wsDir, "task", "add", "Buy wrench", "--project-id", projectID, "--assigned-to", "Plumber")
	taskData := taskRes["data"].(map[string]interface{})
	if pID, _ := taskData["projectId"].(string); pID != projectID {
		t.Errorf("expected task project ID to be %s, got %v", projectID, taskData["projectId"])
	}
	if assigned, _ := taskData["assignedTo"].(string); assigned != "Plumber" {
		t.Errorf("expected task assignedTo to be 'Plumber', got %q", assigned)
	}

	// Container Exclusivity: project task must clear areaId
	if area, ok := taskData["areaId"].(string); ok && area != "" {
		t.Errorf("expected areaId to be empty when project is set")
	}
}

// TestE2E_Tester_DailyCaptureAndInbox verifies brain dump & process flow.
func TestE2E_Tester_DailyCaptureAndInbox(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Capture tasks
	t1 := runCmdJSON(t, wsDir, "task", "add", "Call doctor @phone #health /due:2026-08-01")
	t2 := runCmdJSON(t, wsDir, "task", "add", "Review PR %Jane /waiting")
	t3 := runCmdJSON(t, wsDir, "task", "add", "Clean garage")

	t1Data := t1["data"].(map[string]interface{})
	t2Data := t2["data"].(map[string]interface{})
	t3Data := t3["data"].(map[string]interface{})

	t1ID := t1Data["id"].(string)
	t3ID := t3Data["id"].(string)

	// Verify they are in the inbox if they don't have statuses
	inboxRes := runCmdJSON(t, wsDir, "inbox")
	inboxData := toSlice(inboxRes["data"])
	
	hasT1, hasT3 := false, false
	for _, item := range inboxData {
		m := item.(map[string]interface{})
		id := m["id"].(string)
		if id == t1ID {
			hasT1 = true
		}
		if id == t3ID {
			hasT3 = true
		}
	}
	if !hasT1 {
		t.Errorf("expected Task 1 to be in inbox")
	}
	if !hasT3 {
		t.Errorf("expected Task 3 to be in inbox")
	}

	// 2. Process tasks
	// Move T1 to next
	runCmdJSON(t, wsDir, "task", "update", t1ID, "--status", "next")

	// Check T2 is waiting
	if status, _ := t2Data["status"].(string); status != "waiting" {
		t.Errorf("expected Task 2 to be in waiting status, got %s", status)
	}

	// Move T3 to a project via NLP
	runCmdJSON(t, wsDir, "task", "update", t3ID, `+"Clean House" /next`)

	// Create a reference task
	refRes := runCmdJSON(t, wsDir, "task", "add", "Meeting minutes @meeting /due:2026-07-15 /priority:high")
	refData := refRes["data"].(map[string]interface{})
	refID := refData["id"].(string)

	// Clarify it to reference status
	refUpdateRes := runCmdJSON(t, wsDir, "task", "update", refID, "--status", "reference")
	refUpdateData := refUpdateRes["data"].(map[string]interface{})
	refTask := refUpdateData["task"].(map[string]interface{})

	if refTask["status"].(string) != "reference" {
		t.Errorf("expected reference status")
	}

	// Expected to clear dates & priority (strictly assert)
	if refTask["dueDate"] != nil {
		t.Errorf("expected dueDate to be cleared on reference conversion, got %v", refTask["dueDate"])
	}
	if priority, _ := refTask["priority"].(string); priority != "" {
		t.Errorf("expected priority to be cleared on reference conversion, got %q", priority)
	}
}

// TestE2E_Tester_ReflectAndStalled verifies stalled project detection.
func TestE2E_Tester_ReflectAndStalled(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Create project
	projRes := runCmdJSON(t, wsDir, "project", "add", "Launch Podcast")
	projData := projRes["data"].(map[string]interface{})
	projectID := projData["id"].(string)

	// Rebuild index to populate database
	runCmdJSON(t, wsDir, "index", "rebuild")

	// 2. Run stalled command. Project should be stalled since it has no next actions.
	stalledRes := runCmdJSON(t, wsDir, "stalled")
	stalledData := toSlice(stalledRes["data"])
	foundStalled := false
	for _, item := range stalledData {
		m := item.(map[string]interface{})
		if m["id"].(string) == projectID {
			foundStalled = true
		}
	}
	if !foundStalled {
		t.Errorf("expected project 'Launch Podcast' to be stalled initially")
	}

	// 3. Add a next task to it
	taskRes := runCmdJSON(t, wsDir, "task", "add", `Record intro +"Launch Podcast" /next`)
	taskData := taskRes["data"].(map[string]interface{})
	taskID := taskData["id"].(string)

	// Rebuild index
	runCmdJSON(t, wsDir, "index", "rebuild")

	// Verify project is no longer stalled
	stalledRes2 := runCmdJSON(t, wsDir, "stalled")
	stalledData2 := toSlice(stalledRes2["data"])
	for _, item := range stalledData2 {
		m := item.(map[string]interface{})
		if m["id"].(string) == projectID {
			t.Errorf("expected project 'Launch Podcast' to NOT be stalled after adding a next action")
		}
	}

	// 4. Complete the task
	completeRes := runCmdJSON(t, wsDir, "task", "update", taskID, "--status", "done")
	completeData := completeRes["data"].(map[string]interface{})

	// Assert it returns project_stalled and candidates
	stalledFlag, _ := completeData["project_stalled"].(bool)
	if !stalledFlag {
		t.Errorf("expected project_stalled: true in output metadata when last next action is completed")
	}
}

// TestE2E_Tester_EngageFocusAreaFilters verifies task list and agenda queries with area filtering.
func TestE2E_Tester_EngageFocusAreaFilters(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Add Areas
	workArea := runCmdJSON(t, wsDir, "area", "add", "Work")
	workAreaID := workArea["data"].(map[string]interface{})["id"].(string)

	_ = runCmdJSON(t, wsDir, "area", "add", "Life")

	// 2. Add tasks (due in past, i.e. overdue)
	runCmdJSON(t, wsDir, "task", "add", "Work task A /due:2020-01-01 !Work /next")
	runCmdJSON(t, wsDir, "task", "add", "Life task B /due:2020-01-01 !Life /next")

	runCmdJSON(t, wsDir, "index", "rebuild")

	// 3. Query agenda
	agendaRes := runCmdJSON(t, wsDir, "agenda")
	agendaData := toSlice(agendaRes["data"])
	if len(agendaData) < 2 {
		t.Errorf("expected at least 2 tasks on the agenda, got %d", len(agendaData))
	}

	// 4. Query agenda with Area filter (strictly expected to succeed)
	agendaFilterOutput := runCmdJSON(t, wsDir, "agenda", "--area-id", workAreaID)
	data := toSlice(agendaFilterOutput["data"])
	if len(data) != 1 {
		t.Errorf("expected exactly 1 task for Work area on agenda, got %d", len(data))
	}
}

// TestE2E_Tester_RecurringTaskFollowUps verifies recurring tasks spawn next occurrences.
func TestE2E_Tester_RecurringTaskFollowUps(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Add weekly recurring task
	taskRes := runCmdJSON(t, wsDir, "task", "add", "Submit status report /recur:weekly /due:2026-07-20 /next")
	taskData := taskRes["data"].(map[string]interface{})
	taskID := taskData["id"].(string)

	// 2. Complete the task
	runCmdJSON(t, wsDir, "task", "update", taskID, "--status", "done")

	// 3. Verify a new task has been generated (strictly assert)
	listRes := runCmdJSON(t, wsDir, "task", "list")
	listData := toSlice(listRes["data"])
	
	foundNewOccurrence := false
	for _, item := range listData {
		m := item.(map[string]interface{})
		title := m["title"].(string)
		status := m["status"].(string)
		if title == "Submit status report" && status == "next" && m["id"].(string) != taskID {
			foundNewOccurrence = true
			dueStr, _ := m["dueDate"].(string)
			if !timeMatchesDate(dueStr, "2026-07-27") {
				t.Errorf("expected next due date to be 2026-07-27, got %s", dueStr)
			}
		}
	}
	if !foundNewOccurrence {
		t.Errorf("expected a new recurring task occurrence to be automatically generated")
	}
}

// TestE2E_Tester_Exploration1_AcmeOnboarding stress-tests the Acme onboarding workflow.
func TestE2E_Tester_Exploration1_AcmeOnboarding(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Create Area
	areaRes := runCmdJSON(t, wsDir, "area", "add", "Work")
	areaID := areaRes["data"].(map[string]interface{})["id"].(string)

	// 2. Create Project
	projRes := runCmdJSON(t, wsDir, "project", "add", "Acme Onboarding")
	projID := projRes["data"].(map[string]interface{})["id"].(string)

	// Associate project with area (strictly expected to succeed)
	projUpdateRes := runCmdJSON(t, wsDir, "project", "update", projID, "--area-id", areaID)
	projUpdateData := projUpdateRes["data"].(map[string]interface{})
	if aID, _ := projUpdateData["areaId"].(string); aID != areaID {
		t.Errorf("expected project areaId to update to %s, got %s", areaID, aID)
	}

	// 3. Add tasks
	t1 := runCmdJSON(t, wsDir, "task", "add", `Prepare Acme consulting proposal +"Acme Onboarding" /next`)
	t2 := runCmdJSON(t, wsDir, "task", "add", `Research Acme background +"Acme Onboarding" /next`)
	t3 := runCmdJSON(t, wsDir, "task", "add", `Draft Acme project timeline +"Acme Onboarding" /someday`)
	t4 := runCmdJSON(t, wsDir, "task", "add", `Review Acme pricing structure with CFO +"Acme Onboarding" /waiting`)

	t1ID := t1["data"].(map[string]interface{})["id"].(string)
	t2ID := t2["data"].(map[string]interface{})["id"].(string)
	t3ID := t3["data"].(map[string]interface{})["id"].(string)
	t4ID := t4["data"].(map[string]interface{})["id"].(string)

	// Rebuild index
	runCmdJSON(t, wsDir, "index", "rebuild")

	// 4. Complete Task 1
	t1Update := runCmdJSON(t, wsDir, "task", "update", t1ID, "--status", "done")
	t1UpdateData := t1Update["data"].(map[string]interface{})
	stalled, _ := t1UpdateData["project_stalled"].(bool)
	if stalled {
		t.Errorf("project should not be stalled yet; Task 2 is still next")
	}

	// Rebuild index
	runCmdJSON(t, wsDir, "index", "rebuild")

	// 5. Complete Task 2 (which should cause project stall since only someday/waiting remain)
	t2Update := runCmdJSON(t, wsDir, "task", "update", t2ID, "--status", "done")
	t2UpdateData := t2Update["data"].(map[string]interface{})
	stalled2, _ := t2UpdateData["project_stalled"].(bool)
	if !stalled2 {
		t.Errorf("project should be stalled after completing all next actions")
	}

	// 6. Promote Task 3
	runCmdJSON(t, wsDir, "task", "update", t3ID, "--status", "next")

	// Rebuild index
	runCmdJSON(t, wsDir, "index", "rebuild")

	// Verify project no longer stalled
	stalledRes := runCmdJSON(t, wsDir, "stalled")
	stalledData := toSlice(stalledRes["data"])
	for _, item := range stalledData {
		m := item.(map[string]interface{})
		if m["id"].(string) == projID {
			t.Errorf("expected project 'Acme Onboarding' to not be stalled anymore")
		}
	}

	// 7. Delete Task 4
	runCmdJSON(t, wsDir, "task", "delete", t4ID)
}

// TestE2E_Tester_Exploration2_KitchenRemodel stress-tests remodeling and cascade soft-deletes.
func TestE2E_Tester_Exploration2_KitchenRemodel(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Create Area and Project
	areaRes := runCmdJSON(t, wsDir, "area", "add", "Home")
	areaID := areaRes["data"].(map[string]interface{})["id"].(string)

	projRes := runCmdJSON(t, wsDir, "project", "add", "Kitchen Remodel")
	projID := projRes["data"].(map[string]interface{})["id"].(string)

	// Associate project to Area (strictly expected to succeed)
	runCmdJSON(t, wsDir, "project", "update", projID, "--area-id", areaID)

	// 2. Add tasks
	tA := runCmdJSON(t, wsDir, "task", "add", `Buy white paint +"Kitchen Remodel" /next`)
	tAID := tA["data"].(map[string]interface{})["id"].(string)

	// Agenda only surfaces due/start ≤ now; give Task B an overdue due date so the
	// area-filtered agenda assertion exercises real agenda rules (not undated next).
	tB := runCmdJSON(t, wsDir, "task", "add", "Replace garage lightbulb !Home /next /due:2020-01-01")
	tBID := tB["data"].(map[string]interface{})["id"].(string)

	// 3. Move Task A to someday
	runCmdJSON(t, wsDir, "task", "update", tAID, "--status", "someday")

	runCmdJSON(t, wsDir, "index", "rebuild")

	// 4. Query agenda with Area filter (strictly expected to succeed)
	agendaFilterOutput := runCmdJSON(t, wsDir, "agenda", "--area-id", areaID)
	data := toSlice(agendaFilterOutput["data"])
	
	// Task B should be returned, Task A (someday) is excluded.
	foundB := false
	for _, item := range data {
		m := item.(map[string]interface{})
		if m["id"].(string) == tBID {
			foundB = true
		}
		if m["id"].(string) == tAID {
			t.Errorf("expected Task A (someday) to be excluded from agenda")
		}
	}
	if !foundB {
		t.Errorf("expected Task B (next, linked to Home Area) to be on agenda")
	}

	// 5. Delete Project (soft-delete)
	runCmdJSON(t, wsDir, "project", "delete", projID)

	runCmdJSON(t, wsDir, "index", "rebuild")

	// 6. Verify Task A is soft-deleted too (cascade deletion) - strictly assert
	tARes := runCmdJSON(t, wsDir, "task", "list")
	tAData := toSlice(tARes["data"])
	for _, item := range tAData {
		m := item.(map[string]interface{})
		if m["id"].(string) == tAID {
			t.Errorf("expected cascade deleted task to be hidden from task list")
		}
	}

	// 7. Restore Project (strictly assert project restore command exists and cascades restore)
	runCmdJSON(t, wsDir, "project", "restore", projID)

	runCmdJSON(t, wsDir, "index", "rebuild")

	// Verify project is active again
	restoredProjRes := runCmdJSON(t, wsDir, "project", "list")
	restoredProjs := toSlice(restoredProjRes["data"])
	foundProj := false
	for _, item := range restoredProjs {
		m := item.(map[string]interface{})
		if m["id"].(string) == projID {
			foundProj = true
		}
	}
	if !foundProj {
		t.Errorf("expected project to be restored and in project list")
	}

	// Verify Task A is also restored
	restoredTasksRes := runCmdJSON(t, wsDir, "task", "list")
	restoredTasks := toSlice(restoredTasksRes["data"])
	foundTaskA := false
	for _, item := range restoredTasks {
		m := item.(map[string]interface{})
		if m["id"].(string) == tAID {
			foundTaskA = true
		}
	}
	if !foundTaskA {
		t.Errorf("expected cascade restored task to be active and visible in task list")
	}
}

// TestE2E_Tester_Exploration3_FinanceAudit stress-tests finance audits and weekly routine.
func TestE2E_Tester_Exploration3_FinanceAudit(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Add monthly recurring task under new Area via NLP
	taskRes := runCmdJSON(t, wsDir, "task", "add", "Run weekly expense review !Finance /due:2026-07-15 /recur:weekly /next")
	taskData := taskRes["data"].(map[string]interface{})
	taskID := taskData["id"].(string)
	areaID, _ := taskData["areaId"].(string)

	// 2. Add delegated task
	tDel := runCmdJSON(t, wsDir, "task", "add", "Ask Bob to approve invoice %Bob /waiting")
	tDelID := tDel["data"].(map[string]interface{})["id"].(string)
	_ = tDelID

	runCmdJSON(t, wsDir, "index", "rebuild")

	// 3. Query agenda for Finance Area (strictly expected to succeed)
	agendaFilterOutput := runCmdJSON(t, wsDir, "agenda", "--area-id", areaID)
	data := toSlice(agendaFilterOutput["data"])
	found := false
	for _, item := range data {
		m := item.(map[string]interface{})
		if m["id"].(string) == taskID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected expense review task to show up on agenda for Finance area")
	}

	// 4. Complete expense review
	runCmdJSON(t, wsDir, "task", "update", taskID, "--status", "done")

	// 5. Verify recurrence generation (strictly expected to spawn new task)
	listRes := runCmdJSON(t, wsDir, "task", "list")
	listData := toSlice(listRes["data"])
	foundNewOccurrence := false
	for _, item := range listData {
		m := item.(map[string]interface{})
		if m["title"].(string) == "Run weekly expense review" && m["id"].(string) != taskID {
			foundNewOccurrence = true
			newID := m["id"].(string)
			// Snooze new task
			runCmdJSON(t, wsDir, "task", "update", newID, "/due:2026-07-29")
		}
	}
	if !foundNewOccurrence {
		t.Errorf("expected a new recurring weekly finance audit task to be created")
	}

	// 6. Rebuild index
	runCmdJSON(t, wsDir, "index", "rebuild")
}

// TestE2E_Tester_Exploration4_JapanVacation stress-tests someday projects and active transitions.
func TestE2E_Tester_Exploration4_JapanVacation(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Create Project
	projRes := runCmdJSON(t, wsDir, "project", "add", "Trip to Japan")
	projData := projRes["data"].(map[string]interface{})
	projectID := projData["id"].(string)

	// Move project to someday
	runCmdJSON(t, wsDir, "project", "update", projectID, "--status", "someday")

	// 2. Add tasks
	t1 := runCmdJSON(t, wsDir, "task", "add", `Research Kyoto temples +"Trip to Japan" /someday`)
	t2 := runCmdJSON(t, wsDir, "task", "add", `Book flights +"Trip to Japan" /someday`)

	t1ID := t1["data"].(map[string]interface{})["id"].(string)
	_ = t2["data"].(map[string]interface{})["id"].(string)

	runCmdJSON(t, wsDir, "index", "rebuild")

	// 3. Verify they don't appear in next lists
	nextRes := runCmdJSON(t, wsDir, "next")
	nextData := toSlice(nextRes["data"])
	if len(nextData) > 0 {
		t.Errorf("someday project tasks should not appear in next action list")
	}

	// 4. Activate project
	runCmdJSON(t, wsDir, "project", "update", projectID, "--status", "active")

	// 5. Promote Task 1 to next
	runCmdJSON(t, wsDir, "task", "update", t1ID, "--status", "next")

	runCmdJSON(t, wsDir, "index", "rebuild")

	// Verify Task 1 is now in next list
	nextRes2 := runCmdJSON(t, wsDir, "next")
	nextData2 := toSlice(nextRes2["data"])
	foundT1 := false
	for _, item := range nextData2 {
		m := item.(map[string]interface{})
		if m["id"].(string) == t1ID {
			foundT1 = true
		}
	}
	if !foundT1 {
		t.Errorf("expected promoted task to be in next list")
	}

	// 6. Postpone project again
	runCmdJSON(t, wsDir, "project", "update", projectID, "--status", "someday")

	runCmdJSON(t, wsDir, "index", "rebuild")

	// Verify it's hidden from next list again (strictly assert)
	nextRes3 := runCmdJSON(t, wsDir, "next")
	nextData3 := toSlice(nextRes3["data"])
	for _, item := range nextData3 {
		m := item.(map[string]interface{})
		if m["id"].(string) == t1ID {
			t.Errorf("task should be hidden from next actions when parent project is someday")
		}
	}
}

// TestE2E_Tester_Exploration5_ScaleWeightClamping stress-tests scale weight clamping.
func TestE2E_Tester_Exploration5_ScaleWeightClamping(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Create Area and monthly task anchored on 31st
	runCmdJSON(t, wsDir, "area", "add", "Health")
	taskRes := runCmdJSON(t, wsDir, "task", "add", "Track body metrics /due:2026-01-31 /recur:monthly /next !Health")
	taskData := taskRes["data"].(map[string]interface{})
	taskID := taskData["id"].(string)

	// 2. Complete Jan 31st task
	runCmdJSON(t, wsDir, "task", "update", taskID, "--status", "done")

	// 3. Verify next occurrence clamps to Feb 28th (strictly assert)
	listRes := runCmdJSON(t, wsDir, "task", "list")
	listData := toSlice(listRes["data"])
	
	var febTaskID string
	for _, item := range listData {
		m := item.(map[string]interface{})
		if m["title"].(string) == "Track body metrics" && m["id"].(string) != taskID {
			febTaskID = m["id"].(string)
			dueStr, _ := m["dueDate"].(string)
			if !timeMatchesDate(dueStr, "2026-02-28") {
				t.Errorf("expected Feb occurrence due date to clamp to 2026-02-28, got %s", dueStr)
			}
		}
	}

	if febTaskID == "" {
		t.Fatalf("expected Feb occurrence of the monthly task to be generated")
	}

	// 4. Complete Feb task
	runCmdJSON(t, wsDir, "task", "update", febTaskID, "--status", "done")

	// 5. Verify March task restores to 31st (strictly assert)
	listRes2 := runCmdJSON(t, wsDir, "task", "list")
	listData2 := toSlice(listRes2["data"])
	
	var marTaskID string
	for _, item := range listData2 {
		m := item.(map[string]interface{})
		if m["title"].(string) == "Track body metrics" && m["id"].(string) != taskID && m["id"].(string) != febTaskID {
			marTaskID = m["id"].(string)
			dueStr, _ := m["dueDate"].(string)
			if !timeMatchesDate(dueStr, "2026-03-31") {
				t.Errorf("expected March occurrence to restore anchor to 2026-03-31, got %s", dueStr)
			}
		}
	}

	if marTaskID == "" {
		t.Fatalf("expected March occurrence of the monthly task to be generated")
	}

	// 6. Change March task to daily (strictly assert update recurrence rule)
	dailyRes := runCmdJSON(t, wsDir, "task", "update", marTaskID, "--recurrence", `{"rule":"daily"}`)
	dailyTask := dailyRes["data"].(map[string]interface{})["task"].(map[string]interface{})
	recurMap, _ := dailyTask["recurrence"].(map[string]interface{})
	if recurMap == nil || recurMap["rule"].(string) != "daily" {
		t.Errorf("expected recurrence rule to be updated to daily, got %v", recurMap)
	}
}

// TestE2E_Tester_Exploration6_MarketingCampaign stress-tests waiting pipeline and delegate deletion.
func TestE2E_Tester_Exploration6_MarketingCampaign(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Add Project
	projRes := runCmdJSON(t, wsDir, "project", "add", "Q3 Marketing Campaign")
	projData := projRes["data"].(map[string]interface{})
	_ = projData

	// 2. Add tasks (delegated to Alice and next)
	t1 := runCmdJSON(t, wsDir, "task", "add", "Design banner ad +Q3 Marketing Campaign %Alice /waiting")
	t2 := runCmdJSON(t, wsDir, "task", "add", "Write copy +Q3 Marketing Campaign /next")

	t1ID := t1["data"].(map[string]interface{})["id"].(string)
	t2ID := t2["data"].(map[string]interface{})["id"].(string)

	// 3. Complete Alice task
	runCmdJSON(t, wsDir, "task", "update", t1ID, "--status", "done")

	// 4. Delegate copy task to Charlie (who doesn't exist yet, so we add him to people first, then delegate)
	charlieRes := runCmdJSON(t, wsDir, "people", "add", "Charlie")
	charlieID := charlieRes["data"].(map[string]interface{})["id"].(string)

	runCmdJSON(t, wsDir, "task", "update", t2ID, "%Charlie", "--status", "waiting")

	// 5. Delete Charlie delegate
	runCmdJSON(t, wsDir, "people", "delete", charlieID)

	runCmdJSON(t, wsDir, "index", "rebuild")

	// 6. Assert task copy still retains Charlie as assignedTo (referential integrity check)
	taskCopyRes := runCmdJSON(t, wsDir, "task", "list")
	taskCopyData := toSlice(taskCopyRes["data"])
	for _, item := range taskCopyData {
		m := item.(map[string]interface{})
		if m["id"].(string) == t2ID {
			if assigned, _ := m["assignedTo"].(string); assigned != "Charlie" {
				t.Errorf("expected copy task to still be assigned to Charlie, got %q", assigned)
			}
		}
	}
}

// TestE2E_Tester_Exploration7_ProductLaunch stress-tests extreme pipeline workflow.
func TestE2E_Tester_Exploration7_ProductLaunch(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Add Project
	projRes := runCmdJSON(t, wsDir, "project", "add", "Product Launch")
	projData := projRes["data"].(map[string]interface{})
	projID := projData["id"].(string)

	// 2. Add 5 tasks
	t1 := runCmdJSON(t, wsDir, "task", "add", `Prepare press release +"Product Launch" /next /due:2026-07-20`)
	t2 := runCmdJSON(t, wsDir, "task", "add", `Contact PR agency +"Product Launch" /next /due:2026-07-21`)
	t3 := runCmdJSON(t, wsDir, "task", "add", `Finalize product spec +"Product Launch" /next /due:2026-07-22`)
	t4 := runCmdJSON(t, wsDir, "task", "add", `Build website landing page +"Product Launch" /next /due:2026-07-23`)
	// Task 5 is not yet a next action so completing the last next (Task 4) correctly
	// stalls the project and surfaces Task 5 as a promotion candidate.
	t5 := runCmdJSON(t, wsDir, "task", "add", `Launch product announcement +"Product Launch" /someday /due:2026-07-24`)

	t1Data := t1["data"].(map[string]interface{})
	t1ID := t1Data["id"].(string)
	t2ID := t2["data"].(map[string]interface{})["id"].(string)
	t3ID := t3["data"].(map[string]interface{})["id"].(string)
	t4ID := t4["data"].(map[string]interface{})["id"].(string)
	t5ID := t5["data"].(map[string]interface{})["id"].(string)

	// 3. Set relative start offset on Task 1 (strictly expected to succeed)
	offsetRes := runCmdJSON(t, wsDir, "task", "update", t1ID, "--start-offset", "-1 day")
	data := offsetRes["data"].(map[string]interface{})
	taskObj := data["task"].(map[string]interface{})
	start, _ := taskObj["startTime"].(string)
	if !timeMatchesDate(start, "2026-07-19") {
		t.Errorf("expected relative start date 2026-07-19, got %s", start)
	}

	// Change Task 1 due date to 2026-07-22
	dueRes := runCmdJSON(t, wsDir, "task", "update", t1ID, "/due:2026-07-22")
	dueTask := dueRes["data"].(map[string]interface{})["task"].(map[string]interface{})
	start2, _ := dueTask["startTime"].(string)
	if !timeMatchesDate(start2, "2026-07-21") {
		t.Errorf("expected relative start date to update to 2026-07-21, got %s", start2)
	}

	// 4. Complete Tasks 1, 2, 3, 4
	runCmdJSON(t, wsDir, "task", "update", t1ID, "--status", "done")
	runCmdJSON(t, wsDir, "task", "update", t2ID, "--status", "done")
	runCmdJSON(t, wsDir, "task", "update", t3ID, "--status", "done")

	// Rebuild index to ensure stalled counting is up-to-date
	runCmdJSON(t, wsDir, "index", "rebuild")

	// Complete Task 4
	t4Complete := runCmdJSON(t, wsDir, "task", "update", t4ID, "--status", "done")
	t4CompleteData := t4Complete["data"].(map[string]interface{})

	stalled, _ := t4CompleteData["project_stalled"].(bool)
	if !stalled {
		t.Errorf("expected project_stalled: true when last active next action is completed")
	}
	candidates := toSlice(t4CompleteData["next_action_candidates"])
	if len(candidates) != 1 {
		t.Errorf("expected exactly 1 next action candidate (Task 5), got %d", len(candidates))
	} else {
		cand := candidates[0].(map[string]interface{})
		if cand["id"].(string) != t5ID {
			t.Errorf("expected candidate to be Task 5 (%s), got %s", t5ID, cand["id"].(string))
		}
	}

	// 5. Complete Project
	runCmdJSON(t, wsDir, "project", "delete", projID)
	runCmdJSON(t, wsDir, "index", "rebuild")

	// Verify all tasks soft-deleted (strictly assert cascade soft-deletion)
	listRes := runCmdJSON(t, wsDir, "task", "list")
	listData := toSlice(listRes["data"])
	for _, item := range listData {
		m := item.(map[string]interface{})
		id := m["id"].(string)
		if id == t1ID || id == t2ID || id == t3ID || id == t4ID || id == t5ID {
			t.Errorf("expected tasks under soft-deleted project to be hidden from active list")
		}
	}
}

// TestE2E_Tester_Exploration8_AreaCascade stress-tests area cascade deletion and restoration.
func TestE2E_Tester_Exploration8_AreaCascade(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Create Area
	areaRes := runCmdJSON(t, wsDir, "area", "add", "Education")
	areaID := areaRes["data"].(map[string]interface{})["id"].(string)

	// 2. Create Project in Area (strictly expected to succeed)
	projRes := runCmdJSON(t, wsDir, "project", "add", "Learn Rust")
	projID := projRes["data"].(map[string]interface{})["id"].(string)
	runCmdJSON(t, wsDir, "project", "update", projID, "--area-id", areaID)

	// 3. Create Tasks
	t1Res := runCmdJSON(t, wsDir, "task", "add", `Read Rust book +"Learn Rust" /next`)
	t1ID := t1Res["data"].(map[string]interface{})["id"].(string)
	
	t2Res := runCmdJSON(t, wsDir, "task", "add", "Register for course !Education /next")
	t2ID := t2Res["data"].(map[string]interface{})["id"].(string)

	runCmdJSON(t, wsDir, "index", "rebuild")

	// 4. Soft-delete Area
	runCmdJSON(t, wsDir, "area", "delete", areaID)
	runCmdJSON(t, wsDir, "index", "rebuild")

	// Verify Area, Project, and Tasks are cascade deleted and hidden from lists
	areaList := runCmdJSON(t, wsDir, "area", "list")
	if len(toSlice(areaList["data"])) != 0 {
		t.Errorf("expected area to be soft-deleted and removed from active list")
	}

	projList := runCmdJSON(t, wsDir, "project", "list")
	for _, p := range toSlice(projList["data"]) {
		m := p.(map[string]interface{})
		if m["id"].(string) == projID {
			t.Errorf("expected project to be soft-deleted via cascade")
		}
	}

	taskList := runCmdJSON(t, wsDir, "task", "list")
	for _, tk := range toSlice(taskList["data"]) {
		m := tk.(map[string]interface{})
		id := m["id"].(string)
		if id == t1ID || id == t2ID {
			t.Errorf("expected tasks to be soft-deleted via cascade")
		}
	}

	// 5. Restore Area (strictly expected to succeed and cascade restore)
	runCmdJSON(t, wsDir, "area", "restore", areaID)
	runCmdJSON(t, wsDir, "index", "rebuild")

	// Verify cascade restoration
	areaListRestored := runCmdJSON(t, wsDir, "area", "list")
	if len(toSlice(areaListRestored["data"])) != 1 {
		t.Errorf("expected restored area to be in list")
	}

	projListRestored := runCmdJSON(t, wsDir, "project", "list")
	foundProj := false
	for _, p := range toSlice(projListRestored["data"]) {
		m := p.(map[string]interface{})
		if m["id"].(string) == projID {
			foundProj = true
		}
	}
	if !foundProj {
		t.Errorf("expected project to be restored via cascade")
	}
}

// TestE2E_Tester_Exploration9_TaskDuplication stress-tests completed task duplication.
func TestE2E_Tester_Exploration9_TaskDuplication(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Add Task with detailed metadata
	taskRes := runCmdJSON(t, wsDir, "task", "add", "Prepare presentation @office #work /due:2026-07-20 /energy:high /priority:urgent")
	taskData := taskRes["data"].(map[string]interface{})
	taskID := taskData["id"].(string)

	// 2. Complete Task
	runCmdJSON(t, wsDir, "task", "update", taskID, "--status", "done")

	// 3. Duplicate Task (strictly expected to succeed)
	dupRes := runCmdJSON(t, wsDir, "task", "duplicate", taskID)
	dupData := dupRes["data"].(map[string]interface{})
	dupTask := dupData["task"].(map[string]interface{})

	dupID := dupTask["id"].(string)
	if dupID == "" || dupID == taskID {
		t.Errorf("expected duplicate task to have a new unique ID, got %q", dupID)
	}

	if status, _ := dupTask["status"].(string); status != "next" {
		t.Errorf("expected duplicated task status to reset to 'next', got %q", status)
	}

	if dupTask["completedAt"] != nil {
		t.Errorf("expected completedAt to be cleared on duplicated task")
	}

	// Verify contexts and tags are preserved
	contexts := toSlice(dupTask["contexts"])
	if len(contexts) != 1 || contexts[0].(string) != "@office" {
		t.Errorf("expected duplicated task to preserve contexts, got %v", contexts)
	}

	tags := toSlice(dupTask["tags"])
	if len(tags) != 1 || tags[0].(string) != "#work" {
		t.Errorf("expected duplicated task to preserve tags, got %v", tags)
	}
}

// TestE2E_Tester_Exploration10_TaskPromotion stress-tests promoting an inbox task to a project.
func TestE2E_Tester_Exploration10_TaskPromotion(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Add loose task in area
	taskRes := runCmdJSON(t, wsDir, "task", "add", "Develop Mobile App !Work /next")
	taskData := taskRes["data"].(map[string]interface{})
	taskID := taskData["id"].(string)

	// 2. Promote task to project (strictly expected to succeed)
	promoteRes := runCmdJSON(t, wsDir, "task", "promote", taskID, "Mobile App Project")
	promoteData := promoteRes["data"].(map[string]interface{})
	projectID := promoteData["project_id"].(string)

	// Verify project exists
	projList := runCmdJSON(t, wsDir, "project", "list")
	foundProj := false
	for _, p := range toSlice(projList["data"]) {
		m := p.(map[string]interface{})
		if m["id"].(string) == projectID && m["title"].(string) == "Mobile App Project" {
			foundProj = true
		}
	}
	if !foundProj {
		t.Errorf("expected promoted project 'Mobile App Project' to be created")
	}

	// Verify task's project ID is updated and area ID is cleared
	listRes := runCmdJSON(t, wsDir, "task", "list")
	listData := toSlice(listRes["data"])
	foundTask := false
	for _, item := range listData {
		m := item.(map[string]interface{})
		if m["id"].(string) == taskID {
			foundTask = true
			if pID, _ := m["projectId"].(string); pID != projectID {
				t.Errorf("expected task projectId to be %s, got %v", projectID, m["projectId"])
			}
			if aID, _ := m["areaId"].(string); aID != "" {
				t.Errorf("expected task areaId to be cleared, got %v", aID)
			}
		}
	}
	if !foundTask {
		t.Errorf("expected promoted task to be found in active list")
	}
}


// TestE2E_Tester_Exploration12_QuotedOverrides stress-tests quoted overrides and escapes.
func TestE2E_Tester_Exploration12_QuotedOverrides(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// Create project Proj first to satisfy NLP matcher
	runCmdJSON(t, wsDir, "project", "add", "Proj")
	runCmdJSON(t, wsDir, "index", "rebuild")

	// Add task with quotes and escapes
	taskRes := runCmdJSON(t, wsDir, "task", "add", `Send email to %"Jane Doe" about the \@sign +Proj /next`)
	taskData := taskRes["data"].(map[string]interface{})

	if assigned, _ := taskData["assignedTo"].(string); assigned != "Jane Doe" {
		t.Errorf("expected quoted assignee to be 'Jane Doe', got %q", assigned)
	}
	if title, _ := taskData["title"].(string); title != "Send email to about the @sign" {
		t.Errorf("expected title to be parsed and escapes resolved, got %q", title)
	}
}

// TestE2E_Tester_Exploration13_TimedDueDates stress-tests timed vs date-only relative offset bounds.
func TestE2E_Tester_Exploration13_TimedDueDates(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Task A with date-only due date: offset must fail/warn
	taskARes := runCmdJSON(t, wsDir, "task", "add", "Task A /due:2026-07-20 /next")
	taskAID := taskARes["data"].(map[string]interface{})["id"].(string)

	updateARes, err := runCmdWithEnv(wsDir, "task", "update", taskAID, "--start-offset", "-30 minute")
	if err == nil {
		var res map[string]interface{}
		json.Unmarshal(updateARes, &res)
		taskObj := res["data"].(map[string]interface{})["task"].(map[string]interface{})
		if offset := taskObj["relativeStartOffset"]; offset != nil {
			t.Errorf("expected sub-day offset on date-only due date to be rejected/cleared, got %v", offset)
		}
	}

	// 2. Task B with timed due date: offset must succeed
	taskBRes := runCmdJSON(t, wsDir, "task", "add", "Task B /due:2026-07-20T09:30:00Z /next")
	taskBID := taskBRes["data"].(map[string]interface{})["id"].(string)

	updateBRes := runCmdJSON(t, wsDir, "task", "update", taskBID, "--start-offset", "-30 minute")
	updateBTask := updateBRes["data"].(map[string]interface{})["task"].(map[string]interface{})
	
	start, _ := updateBTask["startTime"].(string)
	if !timeMatchesDate(start, "2026-07-20") { // date component
		t.Errorf("expected startTime to be on 2026-07-20, got %s", start)
	}
}

// TestE2E_Tester_Exploration14_ArchivedProjectTaskRestriction stress-tests archived project task validation.
func TestE2E_Tester_Exploration14_ArchivedProjectTaskRestriction(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// 1. Create project
	projRes := runCmdJSON(t, wsDir, "project", "add", "Launch App")
	projID := projRes["data"].(map[string]interface{})["id"].(string)

	// Add Task 1 (pre-archived)
	t1Res := runCmdJSON(t, wsDir, "task", "add", `Write specs +"Launch App" /next`)
	t1ID := t1Res["data"].(map[string]interface{})["id"].(string)

	// 2. Archive project
	runCmdJSON(t, wsDir, "project", "update", projID, "--status", "archived")
	runCmdJSON(t, wsDir, "index", "rebuild")

	// 3. Try adding new task to archived project (strictly expected to fail validation)
	taskAddOutput, err := runCmdWithEnv(wsDir, "task", "add", `Write release notes +"Launch App" /next`)
	if err == nil {
		t.Errorf("expected task add to archived project container to fail validation, got output: %s", taskAddOutput)
	}

	// 4. Try updating existing task under archived project (strictly expected to fail validation)
	taskUpdateOutput, err := runCmdWithEnv(wsDir, "task", "update", t1ID, "--status", "done")
	if err == nil {
		t.Errorf("expected task update under archived project container to fail validation, got output: %s", taskUpdateOutput)
	}
}



// TestE2E_Tester_AdHocFilters verifies task list and agenda queries with context, project, area, and people filtering.
func TestE2E_Tester_AdHocFilters(t *testing.T) {
	wsDir := t.TempDir()
	runCmdJSON(t, wsDir, "init")

	// Add Project
	projRes := runCmdJSON(t, wsDir, "project", "add", "Migration")
	projID := projRes["data"].(map[string]interface{})["id"].(string)

	// Add Area
	areaRes := runCmdJSON(t, wsDir, "area", "add", "Engineering")
	areaID := areaRes["data"].(map[string]interface{})["id"].(string)

	// Add People
	peopleRes := runCmdJSON(t, wsDir, "people", "add", "Bob")
	_ = peopleRes["data"].(map[string]interface{})["id"].(string)

	// Add tasks
	t1 := runCmdJSON(t, wsDir, "task", "add", "Migrate DB +Migration @computer /next")
	t2 := runCmdJSON(t, wsDir, "task", "add", "Update firewall !Engineering @server /next")
	t3 := runCmdJSON(t, wsDir, "task", "add", "Ask Bob about specs %Bob +Migration /next")

	t1ID := t1["data"].(map[string]interface{})["id"].(string)
	t2ID := t2["data"].(map[string]interface{})["id"].(string)
	t3ID := t3["data"].(map[string]interface{})["id"].(string)

	runCmdJSON(t, wsDir, "index", "rebuild")

	// 1. Filter by Project ID
	listProj := runCmdJSON(t, wsDir, "task", "list", "--project-id", projID)
	dataProj := toSlice(listProj["data"])
	if len(dataProj) != 2 {
		t.Errorf("expected 2 tasks for project filter, got %d", len(dataProj))
	}

	// 2. Filter by Context
	listCtx := runCmdJSON(t, wsDir, "task", "list", "--context", "@computer")
	dataCtx := toSlice(listCtx["data"])
	if len(dataCtx) != 1 || dataCtx[0].(map[string]interface{})["id"].(string) != t1ID {
		t.Errorf("expected 1 task for context @computer, got %d", len(dataCtx))
	}

	// 3. Filter by Area ID
	listArea := runCmdJSON(t, wsDir, "task", "list", "--area-id", areaID)
	dataArea := toSlice(listArea["data"])
	if len(dataArea) != 1 || dataArea[0].(map[string]interface{})["id"].(string) != t2ID {
		t.Errorf("expected 1 task for area Engineering, got %d", len(dataArea))
	}

	// 4. Filter by AssignedTo
	listPeople := runCmdJSON(t, wsDir, "task", "list", "--assigned-to", "Bob")
	dataPeople := toSlice(listPeople["data"])
	if len(dataPeople) != 1 || dataPeople[0].(map[string]interface{})["id"].(string) != t3ID {
		t.Errorf("expected 1 task for assignedTo Bob, got %d", len(dataPeople))
	}
}
