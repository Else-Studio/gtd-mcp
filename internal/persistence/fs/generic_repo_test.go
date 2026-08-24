package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gtd/internal/domain"
)

func TestGenericRepo_DeterministicTimestamps(t *testing.T) {
	dir := t.TempDir()
	repo := NewTaskRepository(dir)

	fixedTime := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	repo.generic.clock = func() time.Time {
		return fixedTime
	}

	task := &domain.Task{
		ID:     "task-fixed-time",
		Title:  "Fixed Time Task",
		Status: domain.TaskStatusInbox,
	}

	if err := repo.Save(task); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Read it back. We also override the clock for the Get operation to verify
	// the Decode method receives the stubbed time.
	loaded, err := repo.Get("task-fixed-time")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if !loaded.CreatedAt.Equal(fixedTime) {
		t.Errorf("Expected CreatedAt to be %v, got %v", fixedTime, loaded.CreatedAt)
	}
	if !loaded.UpdatedAt.Equal(fixedTime) {
		t.Errorf("Expected UpdatedAt to be %v, got %v", fixedTime, loaded.UpdatedAt)
	}
}

func TestGenericRepo_List_SkipsSyncConflictFiles(t *testing.T) {
	dir := t.TempDir()
	repo := NewTaskRepository(dir)

	valid := &domain.Task{
		ID:     "valid-id",
		Title:  "Valid Task",
		Status: domain.TaskStatusInbox,
	}
	if err := repo.Save(valid); err != nil {
		t.Fatalf("save: %v", err)
	}

	taskDir := filepath.Join(dir, "tasks")
	src, err := os.ReadFile(filepath.Join(taskDir, "valid-id.md"))
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	conflictName := "valid-id.sync-conflict-20260824-120000-phone.md"
	if err := os.WriteFile(filepath.Join(taskDir, conflictName), src, 0644); err != nil {
		t.Fatalf("write conflict: %v", err)
	}

	tasks, err := repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "valid-id" {
		t.Fatalf("List returned %d tasks, want exactly valid-id", len(tasks))
	}

	entities, skipped, decodeErrs, err := repo.ListDetail()
	if err != nil {
		t.Fatalf("ListDetail: %v", err)
	}
	if len(decodeErrs) != 0 {
		t.Errorf("decodeErrs = %v, want empty", decodeErrs)
	}
	if len(entities) != 1 || entities[0].ID != "valid-id" {
		t.Fatalf("ListDetail returned %d entities, want exactly valid-id", len(entities))
	}
	foundSkip := false
	for _, s := range skipped {
		if s == conflictName {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Errorf("skipped = %v, want to contain %s", skipped, conflictName)
	}
	for _, e := range entities {
		if strings.Contains(e.ID, "sync-conflict") {
			t.Errorf("entity id %q contains sync-conflict", e.ID)
		}
	}
}

func TestGenericRepo_ListDetail_ReadDirFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tasks"), []byte("not a directory"), 0644); err != nil {
		t.Fatalf("write file-as-dir: %v", err)
	}
	repo := NewTaskRepository(dir)

	entities, skipped, decodeErrs, err := repo.ListDetail()
	if err == nil {
		t.Fatal("expected ReadDir error")
	}
	if len(entities) != 0 || len(skipped) != 0 || len(decodeErrs) != 0 {
		t.Errorf("expected empty results on fatal err, got entities=%d skipped=%d decodeErrs=%d", len(entities), len(skipped), len(decodeErrs))
	}

	_, listErr := repo.List()
	if listErr == nil {
		t.Fatal("List should return the ReadDir error")
	}
}
