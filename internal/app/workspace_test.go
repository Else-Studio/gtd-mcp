package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gtd/internal/domain"
	"gtd/internal/persistence/fs"
)

func TestInitOpen_IndexCanLiveOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "gtd")
	indexPath := filepath.Join(root, "private", "index.db")

	if err := Init(workspace, indexPath); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for _, d := range []string{"tasks", "projects", "areas", "people"} {
		if st, err := os.Stat(filepath.Join(workspace, d)); err != nil || !st.IsDir() {
			t.Errorf("expected workspace dir %s: %v", d, err)
		}
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("expected index file at %s: %v", indexPath, err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "index.db")); !os.IsNotExist(err) {
		t.Fatal("index.db must not be created inside the workspace when indexPath is elsewhere")
	}

	c, err := Open(workspace, indexPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()

	result, err := c.CreateTask(CreateTaskOptions{Text: "Call the plumber @phone"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if result.Task.ID == "" {
		t.Fatal("expected task id")
	}

	got, err := c.GetTask(result.Task.ID)
	if err != nil {
		t.Fatalf("GetTask (markdown load): %v", err)
	}
	if got.Title != "Call the plumber" {
		t.Fatalf("title = %q", got.Title)
	}

	ids, err := c.ListInboxIDs(context.Background())
	if err != nil {
		t.Fatalf("ListInboxIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != result.Task.ID {
		t.Fatalf("inbox ids = %v, want [%s]", ids, result.Task.ID)
	}
}

func TestRebuildIndex_PicksUpExternalMarkdown(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "gtd")
	indexPath := filepath.Join(root, "index.db")
	if err := Init(workspace, indexPath); err != nil {
		t.Fatalf("Init: %v", err)
	}

	id := "11111111-1111-1111-1111-111111111111"
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	repo := fs.NewTaskRepository(filepath.Join(workspace, "tasks"))
	if err := repo.Save(&domain.Task{
		ID:        id,
		Title:     "External capture",
		Status:    domain.TaskStatusInbox,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("write markdown outside Open: %v", err)
	}

	c, err := Open(workspace, indexPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()

	ids, err := c.ListInboxIDs(context.Background())
	if err != nil {
		t.Fatalf("ListInboxIDs before rebuild: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected empty inbox before rebuild, got %v", ids)
	}

	if err := c.RebuildIndex(time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}

	ids, err = c.ListInboxIDs(context.Background())
	if err != nil {
		t.Fatalf("ListInboxIDs after rebuild: %v", err)
	}
	if len(ids) != 1 || ids[0] != id {
		t.Fatalf("inbox after rebuild = %v, want [%s]", ids, id)
	}
}
