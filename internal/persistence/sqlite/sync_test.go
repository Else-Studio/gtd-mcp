package sqlite_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gtd/internal/domain"
	"gtd/internal/persistence/fs"
	"gtd/internal/persistence/sqlite"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func setupDB(t *testing.T) *sqlite.SyncEngine {
	t.Helper()
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	engine := sqlite.NewSyncEngine(db, nil, nil, nil, nil)
	return engine
}

// 1. Trigger Enforcement: Attempt to insert a Task with status invalid_status.
func TestTriggerEnforcement(t *testing.T) {
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO tasks (id, title, status, createdAt, updatedAt) VALUES ('1', 'Test', 'invalid_status', '2026-07-14T00:00:00Z', '2026-07-14T00:00:00Z')`)
	if err == nil {
		t.Fatal("expected error when inserting task with invalid status, got nil")
	}
	if err.Error() != "invalid task status" && !contains(err.Error(), "invalid task status") {
		t.Errorf("expected 'invalid task status' error, got: %v", err)
	}
}

// 2. JSON Constraint: Attempt to insert a Task with malformed JSON string in tags.
func TestJSONConstraint(t *testing.T) {
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO tasks (id, title, status, tags, createdAt, updatedAt) VALUES ('2', 'Test JSON', 'inbox', '{bad-json', '2026-07-14T00:00:00Z', '2026-07-14T00:00:00Z')`)
	if err == nil {
		t.Fatal("expected error when inserting task with bad JSON tags, got nil")
	}
	if !contains(err.Error(), "invalid json payload in tasks") {
		t.Errorf("expected 'invalid json payload' error, got: %v", err)
	}
}

// 3. Load Normalization: Insert raw task with both ProjectID and AreaID populated. Assert AreaID is nil.
func TestLoadNormalization(t *testing.T) {
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	task := &domain.Task{
		ID:        "t3",
		Title:     "Normalize Me",
		Status:    domain.TaskStatusInbox,
		ProjectID: strPtr("p1"),
		AreaID:    strPtr("a1"),
	}

	now := time.Now()
	sqlite.NormalizeTaskForLoad(task, now)

	if task.AreaID != nil {
		t.Errorf("expected AreaID to be nil after normalization, got %v", *task.AreaID)
	}
}

func TestSyncTask_CompleteExistingRow(t *testing.T) {
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	engine := sqlite.NewSyncEngine(db, nil, nil, nil, nil)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	task := &domain.Task{
		ID:        "complete-me",
		Title:     "Buy milk",
		Status:    domain.TaskStatusInbox,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := engine.SyncTask(context.Background(), task, now); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	doneAt := now.Add(time.Hour)
	task.Status = domain.TaskStatusDone
	task.UpdatedAt = doneAt
	task.CompletedAt = &doneAt
	if err := engine.SyncTask(context.Background(), task, doneAt); err != nil {
		t.Fatalf("complete sync: %v", err)
	}

	var status, completedAt string
	var count int
	if err := db.QueryRow(`SELECT COUNT(*), status, completedAt FROM tasks WHERE id = ?`, task.ID).
		Scan(&count, &status, &completedAt); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row after complete, got %d", count)
	}
	if status != "done" {
		t.Fatalf("status = %q, want done", status)
	}
	if completedAt == "" {
		t.Fatal("expected completedAt to be set")
	}
}

// 4. Sync Engine Scale: 100 mock Task files on temp disk. Run Sync Engine. Assert DB contains 100 tasks, 0 areas/projects.
func TestSyncEngineScale(t *testing.T) {
	tempDir := t.TempDir()
	taskRepo := fs.NewTaskRepository(tempDir)

	now := time.Now()
	for i := 0; i < 100; i++ {
		task := &domain.Task{
			ID:        fmt.Sprintf("t-%d", i),
			Title:     fmt.Sprintf("Task %d", i),
			Status:    domain.TaskStatusInbox,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := taskRepo.Save(task); err != nil {
			t.Fatalf("failed to save task %d: %v", i, err)
		}
	}

	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	engine := sqlite.NewSyncEngine(db, taskRepo, nil, nil, nil)
	if _, err := engine.Sync(context.Background(), now); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	var taskCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&taskCount); err != nil {
		t.Fatal(err)
	}
	if taskCount != 100 {
		t.Errorf("expected 100 tasks in DB, got %d", taskCount)
	}

	var areaCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM areas").Scan(&areaCount); err != nil {
		t.Fatal(err)
	}
	if areaCount != 0 {
		t.Errorf("expected 0 areas in DB, got %d", areaCount)
	}

	var projectCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&projectCount); err != nil {
		t.Fatal(err)
	}
	if projectCount != 0 {
		t.Errorf("expected 0 projects in DB, got %d", projectCount)
	}
}

func TestNormalizeTaskForLoad_AdditionalScenarios(t *testing.T) {
	fixedNow := time.Date(2026, 7, 18, 11, 0, 0, 0, time.UTC)

	// Scenario A: Missing Completion Time for Done status
	t.Run("missing completion time", func(t *testing.T) {
		task := &domain.Task{
			ID:        "t-done-no-completed",
			Status:    domain.TaskStatusDone,
			UpdatedAt: fixedNow.Add(-1 * time.Hour),
		}
		sqlite.NormalizeTaskForLoad(task, fixedNow)
		if task.CompletedAt == nil {
			t.Fatal("expected CompletedAt to be populated, got nil")
		}
		if !task.CompletedAt.Equal(task.UpdatedAt) {
			t.Errorf("expected CompletedAt to equal UpdatedAt (%v), got %v", task.UpdatedAt, *task.CompletedAt)
		}
	})

	// Scenario B: Active Task with CompletedAt should have it cleared
	t.Run("active task with completed at", func(t *testing.T) {
		completedTime := fixedNow.Add(-2 * time.Hour)
		task := &domain.Task{
			ID:          "t-active-with-completed",
			Status:      domain.TaskStatusNext,
			CompletedAt: &completedTime,
		}
		sqlite.NormalizeTaskForLoad(task, fixedNow)
		if task.CompletedAt != nil {
			t.Errorf("expected CompletedAt to be cleared (nil) for active task, got %v", *task.CompletedAt)
		}
	})

	// Scenario C: Missing Timestamps should default to now
	t.Run("missing timestamps", func(t *testing.T) {
		task := &domain.Task{
			ID:     "t-no-timestamps",
			Status: domain.TaskStatusInbox,
		}
		sqlite.NormalizeTaskForLoad(task, fixedNow)
		if !task.CreatedAt.Equal(fixedNow) {
			t.Errorf("expected CreatedAt to be %v, got %v", fixedNow, task.CreatedAt)
		}
		if !task.UpdatedAt.Equal(fixedNow) {
			t.Errorf("expected UpdatedAt to be %v, got %v", fixedNow, task.UpdatedAt)
		}
	})

	// Scenario D: UpdatedAt before CreatedAt correction
	t.Run("updated before created", func(t *testing.T) {
		task := &domain.Task{
			ID:        "t-time-anomaly",
			Status:    domain.TaskStatusInbox,
			CreatedAt: fixedNow,
			UpdatedAt: fixedNow.Add(-1 * time.Hour),
		}
		sqlite.NormalizeTaskForLoad(task, fixedNow)
		if !task.UpdatedAt.Equal(task.CreatedAt) {
			t.Errorf("expected UpdatedAt to be normalized to CreatedAt (%v), got %v", task.CreatedAt, task.UpdatedAt)
		}
	})
}

func TestSync_CommitsValidEntitiesWhenListReturnsParseErrors(t *testing.T) {
	tempDir := t.TempDir()
	taskRepo := fs.NewTaskRepository(tempDir)

	now := time.Now()
	valid := &domain.Task{
		ID:        "valid-1",
		Title:     "Valid Task",
		Status:    domain.TaskStatusNext,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := taskRepo.Save(valid); err != nil {
		t.Fatalf("save valid: %v", err)
	}

	corruptContent := []byte(`---
status: [invalid yaml
---
Title`)
	if err := os.WriteFile(filepath.Join(tempDir, "tasks", "corrupt-1.md"), corruptContent, 0644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	engine := sqlite.NewSyncEngine(db, taskRepo, nil, nil, nil)
	report, err := engine.Sync(context.Background(), now)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if report.TaskCount != 1 {
		t.Errorf("TaskCount = %d, want 1", report.TaskCount)
	}
	if len(report.Errors) == 0 {
		t.Fatal("expected non-empty Errors")
	}
	if !contains(strings.Join(report.Errors, "\n"), "corrupt-1.md") {
		t.Errorf("Errors should name the file, got %v", report.Errors)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("tasks count = %d, want 1", count)
	}
}

func TestSync_SkipsSyncConflictFiles(t *testing.T) {
	tempDir := t.TempDir()
	taskRepo := fs.NewTaskRepository(tempDir)

	now := time.Now()
	id := "valid-id"
	valid := &domain.Task{
		ID:        id,
		Title:     "Valid Task",
		Status:    domain.TaskStatusInbox,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := taskRepo.Save(valid); err != nil {
		t.Fatalf("save valid: %v", err)
	}

	src, err := os.ReadFile(filepath.Join(tempDir, "tasks", id+".md"))
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	conflictName := id + ".sync-conflict-20260824-120000-phone.md"
	if err := os.WriteFile(filepath.Join(tempDir, "tasks", conflictName), src, 0644); err != nil {
		t.Fatalf("write conflict: %v", err)
	}

	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	engine := sqlite.NewSyncEngine(db, taskRepo, nil, nil, nil)
	report, err := engine.Sync(context.Background(), now)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if report.TaskCount != 1 {
		t.Errorf("TaskCount = %d, want 1", report.TaskCount)
	}
	if len(report.Errors) != 0 {
		t.Errorf("Errors = %v, want empty", report.Errors)
	}
	foundSkip := false
	for _, s := range report.SkippedConflicts {
		if s == conflictName {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Errorf("SkippedConflicts = %v, want to contain %s", report.SkippedConflicts, conflictName)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("tasks count = %d, want 1", count)
	}
}

func TestSync_ReadDirFailureDoesNotCommitEmptyIndex(t *testing.T) {
	root := t.TempDir()
	db, err := sqlite.NewDB("file:" + filepath.ToSlash(filepath.Join(root, "index.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO tasks (id, title, status, createdAt, updatedAt) VALUES ('seed', 'Seed', 'inbox', '2026-08-24T00:00:00Z', '2026-08-24T00:00:00Z')`)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, "tasks"), []byte("not a directory"), 0644); err != nil {
		t.Fatalf("write file-as-dir: %v", err)
	}
	engine := sqlite.NewSyncEngine(db, fs.NewTaskRepository(repoRoot), nil, nil, nil)
	if _, err := engine.Sync(context.Background(), time.Now()); err == nil {
		t.Fatal("expected sync error from ReadDir failure")
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected seed row to remain after rollback, got count=%d", count)
	}
	var id string
	if err := db.QueryRow("SELECT id FROM tasks").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if id != "seed" {
		t.Errorf("id = %q, want seed", id)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func strPtr(s string) *string {
	return &s
}
