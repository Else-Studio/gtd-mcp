package fs

import (
	"errors"
	"gtd/internal/domain"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestPerfectRoundTrip(t *testing.T) {
	dir := t.TempDir()
	repo := NewTaskRepository(dir)

	now := time.Now().Truncate(time.Second)
	dueDate := now.Add(24 * time.Hour)
	projID := "proj-1"

	task := &domain.Task{
		ID:          "task-123",
		Title:       "Buy groceries",
		Description: "- Milk\n- Eggs",
		Status:      domain.TaskStatusNext,
		Tags:        []string{"#errand"},
		DueDate:     &dueDate,
		ProjectID:   &projID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err := repo.Save(task)
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := repo.Get("task-123")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if !task.CreatedAt.Equal(loaded.CreatedAt) {
		t.Errorf("CreatedAt differs. Expected %v, got %v", task.CreatedAt, loaded.CreatedAt)
	}
	// Sync time pointers to avoid reflect.DeepEqual failing on time.Location pointer addresses
	task.CreatedAt = loaded.CreatedAt
	task.UpdatedAt = loaded.UpdatedAt

	if task.DueDate != nil && loaded.DueDate != nil {
		if !task.DueDate.Equal(*loaded.DueDate) {
			t.Errorf("DueDate differs. Expected %v, got %v", *task.DueDate, *loaded.DueDate)
		}
		task.DueDate = loaded.DueDate
	}

	if !reflect.DeepEqual(task, loaded) {
		t.Errorf("Round trip failed.\nExpected: %+v\nGot:      %+v", task, loaded)
	}
}

func TestBodyParsingIntegrity(t *testing.T) {
	dir := t.TempDir()
	repo := NewTaskRepository(dir)

	rawContent := []byte(`---
status: inbox
---

My Title
- [ ] Task 1
- [ ] Task 2`)

	taskDir := filepath.Join(dir, "tasks")
	os.MkdirAll(taskDir, 0755)
	os.WriteFile(filepath.Join(taskDir, "task-parse.md"), rawContent, 0644)

	loaded, err := repo.Get("task-parse")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.Title != "My Title" {
		t.Errorf("Expected title 'My Title', got '%s'", loaded.Title)
	}
	expectedDesc := "- [ ] Task 1\n- [ ] Task 2"
	if loaded.Description != expectedDesc {
		t.Errorf("Expected desc '%s', got '%s'", expectedDesc, loaded.Description)
	}
}

func TestMissingSystemFields(t *testing.T) {
	dir := t.TempDir()
	repo := NewTaskRepository(dir)

	rawContent := []byte(`---
status: inbox
---
Title`)

	taskDir := filepath.Join(dir, "tasks")
	os.MkdirAll(taskDir, 0755)
	os.WriteFile(filepath.Join(taskDir, "task-sys.md"), rawContent, 0644)

	loaded, err := repo.Get("task-sys")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be populated securely")
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be populated securely")
	}
	if loaded.UpdatedAt.Before(loaded.CreatedAt) {
		t.Error("UpdatedAt should be >= CreatedAt")
	}
}

func TestEmptyBodyParsing(t *testing.T) {
	dir := t.TempDir()
	repo := NewTaskRepository(dir)

	rawContent := []byte(`---
status: inbox
---
`)

	taskDir := filepath.Join(dir, "tasks")
	os.MkdirAll(taskDir, 0755)
	os.WriteFile(filepath.Join(taskDir, "task-empty.md"), rawContent, 0644)

	loaded, err := repo.Get("task-empty")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.Title != "" {
		t.Errorf("Expected empty title, got '%s'", loaded.Title)
	}
}

func TestListCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	repo := NewTaskRepository(dir)

	// Save a valid task
	validTask := &domain.Task{
		ID:     "valid-1",
		Title:  "Valid Task",
		Status: domain.TaskStatusNext,
	}
	if err := repo.Save(validTask); err != nil {
		t.Fatalf("failed to save valid task: %v", err)
	}

	// Create a corrupted file
	taskDir := filepath.Join(dir, "tasks")
	corruptContent := []byte(`---
status: [invalid yaml
---
Title`)
	os.WriteFile(filepath.Join(taskDir, "corrupt-1.md"), corruptContent, 0644)

	tasks, err := repo.List()
	if err == nil {
		t.Fatal("Expected error due to corrupted file, got nil")
	}

	if len(tasks) != 1 || tasks[0].ID != "valid-1" {
		t.Errorf("Expected 1 valid task to be returned, got %d", len(tasks))
	}
}

func TestGenericRepo_Get_NotFound(t *testing.T) {
	// Setup: Point repo to an empty TempDir.
	dir := t.TempDir()
	repo := NewTaskRepository(dir)

	// Action: Call Get("nonexistent-id").
	_, err := repo.Get("nonexistent-id")

	// Outcome: Assert returned error unwraps to domain.ErrNotFound.
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestMarkdownCodec_ValidationEnforcement(t *testing.T) {
	// Setup: Initialize a TaskRepository. Create an invalid Task (missing Title).
	dir := t.TempDir()
	repo := NewTaskRepository(dir)
	invalidTask := &domain.Task{
		ID:     "task-invalid",
		Title:  "", // Title cannot be empty
		Status: domain.TaskStatusNext,
	}

	// Action: Call Save(invalidTask).
	err := repo.Save(invalidTask)

	// Outcome: Assert Save returns an error that wraps domain.ErrValidation.
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation error, got %v", err)
	}

	// Assert no file was actually written to the directory.
	path := filepath.Join(dir, "tasks", "task-invalid.md")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file %s to not exist, but it does", path)
	}
}
