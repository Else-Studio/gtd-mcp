package fs

import (
	"gtd/internal/domain"
	"testing"
	"time"
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
