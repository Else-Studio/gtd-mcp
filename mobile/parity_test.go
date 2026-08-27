package mobile

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"gtd/internal/domain"
	"gtd/internal/persistence/fs"
)

var (
	cliBinOnce sync.Once
	cliBinPath string
	cliBinErr  error
)

func ensureCLIBinary(t *testing.T) string {
	t.Helper()
	cliBinOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "gtd-cli-build-*")
		if err != nil {
			cliBinErr = err
			return
		}
		bin := filepath.Join(tmpDir, "gtd")
		cmd := exec.Command("go", "build", "-o", bin, "gtd/cmd/gtd")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			cliBinErr = err
			return
		}
		cliBinPath = bin
	})
	if cliBinErr != nil {
		t.Fatalf("build gtd binary: %v", cliBinErr)
	}
	return cliBinPath
}

func runCLI(t *testing.T, workspace string, args ...string) map[string]interface{} {
	t.Helper()
	bin := ensureCLIBinary(t)
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "GTD_DIR="+workspace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cli %v failed: %v\n%s", args, err, out)
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("invalid JSON for %v: %v\n%s", args, err, out)
	}
	if ok, _ := envelope["success"].(bool); !ok {
		t.Fatalf("expected success for %v: %s", args, out)
	}
	return envelope
}

func cliTaskIDs(t *testing.T, workspace string, args ...string) []string {
	t.Helper()
	envelope := runCLI(t, workspace, args...)
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected array data for %v: %#v", args, envelope["data"])
	}
	ids := make([]string, len(data))
	for i, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("expected task object at index %d for %v: %T", i, args, item)
		}
		id, _ := m["id"].(string)
		if id == "" {
			t.Fatalf("missing id in CLI task item %d: %#v", i, m)
		}
		ids[i] = id
	}
	return ids
}

const (
	projAppLaunchID   = "p1111111-1111-1111-1111-111111111111"
	projOfficeRenoID  = "p2222222-2222-2222-2222-222222222222"
	projLearnRustID   = "p3333333-3333-3333-3333-333333333333"
	projOldArchiveID  = "p4444444-4444-4444-4444-444444444444"

	areaWorkID        = "a1111111-1111-1111-1111-111111111111"
	areaPersonalID    = "a2222222-2222-2222-2222-222222222222"

	tInboxLooseID     = "t0111111-1111-1111-1111-111111111111"
	tInboxAppID       = "t0222222-2222-2222-2222-222222222222"
	tNextAppID        = "t0333333-3333-3333-3333-333333333333"
	tNextLooseOffice  = "t0444444-4444-4444-4444-444444444444"
	tNextLooseHome    = "t0555555-5555-5555-5555-555555555555"
	tNextRustSomeday  = "t0666666-6666-6666-6666-666666666666"
	tWaitAppID        = "t0777777-7777-7777-7777-777777777777"
	tWaitLooseID      = "t0888888-8888-8888-8888-888888888888"
	tSomedayLooseID   = "t0999999-9999-9999-9999-999999999999"
	tAgendaDueToday   = "t1000000-0000-0000-0000-000000000000"
	tAgendaOverdue    = "t1100000-0000-0000-0000-000000000000"
	tDoneAppID        = "t1200000-0000-0000-0000-000000000000"
	tDoneLooseID      = "t1300000-0000-0000-0000-000000000000"
	tArchivedAppID    = "t1400000-0000-0000-0000-000000000000"
	tDeletedID        = "t1500000-0000-0000-0000-000000000000"
)

func setupParityWorkspace(t *testing.T) (ws string, g *Gtd, now time.Time) {
	t.Helper()
	ws = t.TempDir()
	indexPath := filepath.Join(ws, "index.db")

	base := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	now = time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	dueToday := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	dueYesterday := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	completedTime := base.Add(2 * time.Hour)
	deletedTime := base.Add(1 * time.Hour)

	// Setup Areas
	areaRepo := fs.NewAreaRepository(filepath.Join(ws, "areas"))
	if err := areaRepo.Save(&domain.Area{
		ID:        areaWorkID,
		Name:      "Work",
		CreatedAt: base,
		UpdatedAt: base,
	}); err != nil {
		t.Fatalf("save area work: %v", err)
	}
	if err := areaRepo.Save(&domain.Area{
		ID:        areaPersonalID,
		Name:      "Personal",
		CreatedAt: base,
		UpdatedAt: base,
	}); err != nil {
		t.Fatalf("save area personal: %v", err)
	}

	// Setup Projects
	projRepo := fs.NewProjectRepository(filepath.Join(ws, "projects"))
	pAreaWork := areaWorkID
	pAreaPersonal := areaPersonalID

	if err := projRepo.Save(&domain.Project{
		ID:        projAppLaunchID,
		Title:     "App Launch",
		Status:    domain.ProjectStatusActive,
		AreaID:    &pAreaWork,
		CreatedAt: base,
		UpdatedAt: base,
	}); err != nil {
		t.Fatalf("save proj app launch: %v", err)
	}
	if err := projRepo.Save(&domain.Project{
		ID:        projOfficeRenoID,
		Title:     "Office Reno",
		Status:    domain.ProjectStatusActive,
		AreaID:    &pAreaWork,
		CreatedAt: base,
		UpdatedAt: base,
	}); err != nil {
		t.Fatalf("save proj office reno: %v", err)
	}
	if err := projRepo.Save(&domain.Project{
		ID:        projLearnRustID,
		Title:     "Learn Rust",
		Status:    domain.ProjectStatusSomeday,
		AreaID:    &pAreaPersonal,
		CreatedAt: base,
		UpdatedAt: base,
	}); err != nil {
		t.Fatalf("save proj learn rust: %v", err)
	}
	if err := projRepo.Save(&domain.Project{
		ID:        projOldArchiveID,
		Title:     "Old Archive",
		Status:    domain.ProjectStatusArchived,
		AreaID:    &pAreaWork,
		CreatedAt: base,
		UpdatedAt: base,
	}); err != nil {
		t.Fatalf("save proj old archive: %v", err)
	}

	// Setup Tasks
	pApp := projAppLaunchID
	pRust := projLearnRustID

	taskRepo := fs.NewTaskRepository(filepath.Join(ws, "tasks"))
	tasks := []*domain.Task{
		{
			ID:        tInboxLooseID,
			Title:     "Inbox unorganized",
			Status:    domain.TaskStatusInbox,
			Contexts:  []string{"@phone"},
			CreatedAt: base.Add(1 * time.Second),
			UpdatedAt: base.Add(1 * time.Second),
		},
		{
			ID:        tInboxAppID,
			Title:     "App Launch feature spec",
			Status:    domain.TaskStatusInbox,
			ProjectID: &pApp,
			Contexts:  []string{"@office"},
			CreatedAt: base.Add(2 * time.Second),
			UpdatedAt: base.Add(2 * time.Second),
		},
		{
			ID:        tNextAppID,
			Title:     "Design app icon",
			Status:    domain.TaskStatusNext,
			ProjectID: &pApp,
			Contexts:  []string{"@office"},
			CreatedAt: base.Add(3 * time.Second),
			UpdatedAt: base.Add(3 * time.Second),
		},
		{
			ID:        tNextLooseOffice,
			Title:     "Call supplier",
			Status:    domain.TaskStatusNext,
			Contexts:  []string{"@office"},
			CreatedAt: base.Add(4 * time.Second),
			UpdatedAt: base.Add(4 * time.Second),
		},
		{
			ID:        tNextLooseHome,
			Title:     "Fix leaky sink",
			Status:    domain.TaskStatusNext,
			Contexts:  []string{"@home"},
			CreatedAt: base.Add(5 * time.Second),
			UpdatedAt: base.Add(5 * time.Second),
		},
		{
			ID:        tNextRustSomeday,
			Title:     "Read Rust Book",
			Status:    domain.TaskStatusNext,
			ProjectID: &pRust,
			Contexts:  []string{"@home"},
			CreatedAt: base.Add(6 * time.Second),
			UpdatedAt: base.Add(6 * time.Second),
		},
		{
			ID:        tWaitAppID,
			Title:     "Wait for client signoff",
			Status:    domain.TaskStatusWaiting,
			ProjectID: &pApp,
			Contexts:  []string{"@office"},
			CreatedAt: base.Add(7 * time.Second),
			UpdatedAt: base.Add(7 * time.Second),
		},
		{
			ID:        tWaitLooseID,
			Title:     "Wait for delivery",
			Status:    domain.TaskStatusWaiting,
			Contexts:  []string{"@home"},
			CreatedAt: base.Add(8 * time.Second),
			UpdatedAt: base.Add(8 * time.Second),
		},
		{
			ID:        tSomedayLooseID,
			Title:     "Learn pottery",
			Status:    domain.TaskStatusSomeday,
			Contexts:  []string{"@home"},
			CreatedAt: base.Add(9 * time.Second),
			UpdatedAt: base.Add(9 * time.Second),
		},
		{
			ID:        tAgendaDueToday,
			Title:     "Submit tax form",
			Status:    domain.TaskStatusNext,
			Contexts:  []string{"@office"},
			DueDate:   &dueToday,
			CreatedAt: base.Add(10 * time.Second),
			UpdatedAt: base.Add(10 * time.Second),
		},
		{
			ID:        tAgendaOverdue,
			Title:     "Pay power bill",
			Status:    domain.TaskStatusNext,
			Contexts:  []string{"@home"},
			DueDate:   &dueYesterday,
			CreatedAt: base.Add(11 * time.Second),
			UpdatedAt: base.Add(11 * time.Second),
		},
		{
			ID:          tDoneAppID,
			Title:       "Initial brainstorming",
			Status:      domain.TaskStatusDone,
			ProjectID:   &pApp,
			Contexts:    []string{"@office"},
			CompletedAt: &completedTime,
			CreatedAt:   base.Add(12 * time.Second),
			UpdatedAt:   base.Add(12 * time.Second),
		},
		{
			ID:          tDoneLooseID,
			Title:       "Old completed task",
			Status:      domain.TaskStatusDone,
			Contexts:    []string{"@office"},
			CompletedAt: &completedTime,
			CreatedAt:   base.Add(13 * time.Second),
			UpdatedAt:   base.Add(13 * time.Second),
		},
		{
			ID:        tArchivedAppID,
			Title:     "Legacy prototype",
			Status:    domain.TaskStatusArchived,
			ProjectID: &pApp,
			Contexts:  []string{"@office"},
			CreatedAt: base.Add(14 * time.Second),
			UpdatedAt: base.Add(14 * time.Second),
		},
		{
			ID:        tDeletedID,
			Title:     "Deleted idea",
			Status:    domain.TaskStatusInbox,
			Contexts:  []string{"@office"},
			DeletedAt: &deletedTime,
			CreatedAt: base.Add(15 * time.Second),
			UpdatedAt: base.Add(15 * time.Second),
		},
	}

	for _, task := range tasks {
		if err := taskRepo.Save(task); err != nil {
			t.Fatalf("save task %s: %v", task.ID, err)
		}
	}

	// Initialize database index via CLI init / rebuild
	runCLI(t, ws, "init")
	runCLI(t, ws, "index", "rebuild")

	// Open Mobile facade
	g = newGtd(t)
	openRebuild(t, g, ws, indexPath)

	return ws, g, now
}

func TestParity_Inbox(t *testing.T) {
	ws, g, _ := setupParityWorkspace(t)

	// MCP CLI: gtd inbox
	cliIDs := cliTaskIDs(t, ws, "inbox")

	// Mobile Facade: op: list, view: inbox
	mobileEnv := invoke(t, g, map[string]string{"op": "list", "view": "inbox"})
	wantOK(t, mobileEnv)
	mobileIDs := dataTaskIDs(t, mobileEnv)

	if !reflect.DeepEqual(cliIDs, mobileIDs) {
		t.Fatalf("Inbox ID mismatch:\nCLI:    %v\nMobile: %v", cliIDs, mobileIDs)
	}

	// Verify exact expected inbox tasks
	wantIDs := []string{tInboxLooseID, tInboxAppID}
	if !reflect.DeepEqual(mobileIDs, wantIDs) {
		t.Fatalf("got inbox IDs %v, want %v", mobileIDs, wantIDs)
	}

	// Assert strict active-only: zero done, archived, or deleted tasks
	for _, id := range mobileIDs {
		if id == tDoneAppID || id == tDoneLooseID || id == tArchivedAppID || id == tDeletedID {
			t.Fatalf("unexpected inactive task %s returned in inbox", id)
		}
	}
}

func TestParity_Next(t *testing.T) {
	ws, g, _ := setupParityWorkspace(t)

	// MCP CLI: gtd next
	cliIDs := cliTaskIDs(t, ws, "next")

	// Mobile Facade: op: list, view: next
	mobileEnv := invoke(t, g, map[string]string{"op": "list", "view": "next"})
	wantOK(t, mobileEnv)
	mobileIDs := dataTaskIDs(t, mobileEnv)

	if !reflect.DeepEqual(cliIDs, mobileIDs) {
		t.Fatalf("Next ID mismatch:\nCLI:    %v\nMobile: %v", cliIDs, mobileIDs)
	}

	// Verify next tasks in someday projects (tNextRustSomeday) are strictly excluded
	if containsString(mobileIDs, tNextRustSomeday) {
		t.Fatalf("next view must exclude tasks in Someday projects: found %s", tNextRustSomeday)
	}

	// Verify zero done/archived/deleted tasks
	for _, id := range mobileIDs {
		if id == tDoneAppID || id == tDoneLooseID || id == tArchivedAppID || id == tDeletedID {
			t.Fatalf("unexpected inactive task %s returned in next", id)
		}
	}

	// Verify expected active next actions are present
	for _, expected := range []string{tNextAppID, tNextLooseOffice, tNextLooseHome, tAgendaDueToday, tAgendaOverdue} {
		if !containsString(mobileIDs, expected) {
			t.Fatalf("expected next task %s missing from next view", expected)
		}
	}
}

func TestParity_Agenda(t *testing.T) {
	ws, g, now := setupParityWorkspace(t)
	nowStr := now.Format(time.RFC3339)

	// MCP CLI: gtd agenda
	cliIDs := cliTaskIDs(t, ws, "agenda")

	// Mobile Facade: op: list, view: agenda, now: ...
	mobileEnv := invoke(t, g, map[string]interface{}{
		"op":   "list",
		"view": "agenda",
		"now":  nowStr,
	})
	wantOK(t, mobileEnv)
	mobileIDs := dataTaskIDs(t, mobileEnv)

	if !reflect.DeepEqual(cliIDs, mobileIDs) {
		t.Fatalf("Agenda ID mismatch:\nCLI:    %v\nMobile: %v", cliIDs, mobileIDs)
	}

	// Verify both overdue and due-today tasks are returned
	if !containsString(mobileIDs, tAgendaDueToday) {
		t.Fatalf("agenda missing due today task: %s", tAgendaDueToday)
	}
	if !containsString(mobileIDs, tAgendaOverdue) {
		t.Fatalf("agenda missing overdue task: %s", tAgendaOverdue)
	}

	// Verify zero done/archived/deleted tasks
	for _, id := range mobileIDs {
		if id == tDoneAppID || id == tDoneLooseID || id == tArchivedAppID || id == tDeletedID {
			t.Fatalf("unexpected inactive task %s returned in agenda", id)
		}
	}
}

func TestParity_Context(t *testing.T) {
	ws, g, _ := setupParityWorkspace(t)

	// MCP CLI: gtd task list --context @office
	cliIDs := cliTaskIDs(t, ws, "task", "list", "--context", "@office")

	// Mobile Facade: op: list, view: context, context: @office
	mobileEnv := invoke(t, g, map[string]string{
		"op":      "list",
		"view":    "context",
		"context": "@office",
	})
	wantOK(t, mobileEnv)
	mobileIDs := dataTaskIDs(t, mobileEnv)

	if !reflect.DeepEqual(cliIDs, mobileIDs) {
		t.Fatalf("Context @office ID mismatch:\nCLI:    %v\nMobile: %v", cliIDs, mobileIDs)
	}

	// Verify active @office tasks are present
	for _, expected := range []string{tInboxAppID, tNextAppID, tNextLooseOffice, tWaitAppID, tAgendaDueToday} {
		if !containsString(mobileIDs, expected) {
			t.Fatalf("expected active @office task %s missing", expected)
		}
	}

	// Verify zero done/archived/deleted tasks returned
	for _, id := range mobileIDs {
		if id == tDoneAppID || id == tDoneLooseID || id == tArchivedAppID || id == tDeletedID {
			t.Fatalf("unexpected inactive task %s returned in context view", id)
		}
	}
}

func TestParity_Project(t *testing.T) {
	ws, g, _ := setupParityWorkspace(t)

	// Query by Project ID
	cliIDsByID := cliTaskIDs(t, ws, "task", "list", "--project-id", projAppLaunchID)
	mobileEnvByID := invoke(t, g, map[string]string{
		"op":         "list",
		"view":       "project",
		"project_id": projAppLaunchID,
	})
	wantOK(t, mobileEnvByID)
	mobileIDsByID := dataTaskIDs(t, mobileEnvByID)

	if !reflect.DeepEqual(cliIDsByID, mobileIDsByID) {
		t.Fatalf("Project ID mismatch:\nCLI:    %v\nMobile: %v", cliIDsByID, mobileIDsByID)
	}

	// Query by Project Title
	cliIDsByTitle := cliTaskIDs(t, ws, "task", "list", "--project", "App Launch")
	mobileEnvByTitle := invoke(t, g, map[string]string{
		"op":      "list",
		"view":    "project",
		"project": "App Launch",
	})
	wantOK(t, mobileEnvByTitle)
	mobileIDsByTitle := dataTaskIDs(t, mobileEnvByTitle)

	if !reflect.DeepEqual(cliIDsByTitle, mobileIDsByTitle) {
		t.Fatalf("Project Title mismatch:\nCLI:    %v\nMobile: %v", cliIDsByTitle, mobileIDsByTitle)
	}

	if !reflect.DeepEqual(mobileIDsByID, mobileIDsByTitle) {
		t.Fatalf("Project ID vs Title query mismatch:\nByID:    %v\nByTitle: %v", mobileIDsByID, mobileIDsByTitle)
	}

	// Verify active project tasks are present
	for _, expected := range []string{tInboxAppID, tNextAppID, tWaitAppID} {
		if !containsString(mobileIDsByID, expected) {
			t.Fatalf("expected project task %s missing", expected)
		}
	}

	// Verify done and archived project tasks are strictly excluded
	if containsString(mobileIDsByID, tDoneAppID) {
		t.Fatalf("done task %s must not be returned in project view", tDoneAppID)
	}
	if containsString(mobileIDsByID, tArchivedAppID) {
		t.Fatalf("archived task %s must not be returned in project view", tArchivedAppID)
	}
}

func TestParity_Rebuild_ExternalState(t *testing.T) {
	ws, g, _ := setupParityWorkspace(t)

	// Verify tInboxLooseID is currently in inbox and active lists
	mobileInboxBefore := invoke(t, g, map[string]string{"op": "list", "view": "inbox"})
	if !containsString(dataTaskIDs(t, mobileInboxBefore), tInboxLooseID) {
		t.Fatalf("task %s not found in inbox before modification", tInboxLooseID)
	}

	// External update: modify the markdown file on disk directly
	taskRepo := fs.NewTaskRepository(filepath.Join(ws, "tasks"))
	task, err := taskRepo.Get(tInboxLooseID)
	if err != nil {
		t.Fatalf("load task from disk: %v", err)
	}

	completedAt := time.Now().UTC()
	task.Status = domain.TaskStatusDone
	task.CompletedAt = &completedAt
	if err := taskRepo.Save(task); err != nil {
		t.Fatalf("save modified task: %v", err)
	}

	// Rebuild in both CLI and Mobile
	runCLI(t, ws, "index", "rebuild")
	rebuildEnv := invoke(t, g, map[string]string{"op": "rebuild"})
	wantOK(t, rebuildEnv)

	// Verify Inbox view on both facades immediately excludes the completed task
	cliInboxAfter := cliTaskIDs(t, ws, "inbox")
	mobileInboxAfter := invoke(t, g, map[string]string{"op": "list", "view": "inbox"})
	wantOK(t, mobileInboxAfter)
	mobileIDsAfter := dataTaskIDs(t, mobileInboxAfter)

	if !reflect.DeepEqual(cliInboxAfter, mobileIDsAfter) {
		t.Fatalf("Rebuilt Inbox mismatch:\nCLI:    %v\nMobile: %v", cliInboxAfter, mobileIDsAfter)
	}

	if containsString(mobileIDsAfter, tInboxLooseID) {
		t.Fatalf("completed task %s still returned in inbox after rebuild", tInboxLooseID)
	}
}
