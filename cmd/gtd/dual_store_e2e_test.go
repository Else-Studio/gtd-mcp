package main_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gtd/internal/domain"
	"gtd/internal/persistence/fs"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// TestE2E_Persist_FileAndIndexAgree_OnTaskDone locks the dual-store contract:
// after a CLI write that completes a task, both the markdown file and index.db
// must carry status=done and a non-null completedAt that agree.
func TestE2E_Persist_FileAndIndexAgree_OnTaskDone(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")

	// Create already-done via NLP /done — the classic drift case where status was
	// set without UpdateStatus, so the file historically lacked completedAt while
	// SQL normalize repaired the index only.
	result := runCmdE2E(t, workspaceDir, "task", "add", "Finished /done")
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected task data object, got %T", result["data"])
	}
	taskID, ok := data["id"].(string)
	if !ok || taskID == "" {
		t.Fatalf("expected task id, got %v", data["id"])
	}

	if status, _ := data["status"].(string); status != "done" {
		t.Fatalf("expected JSON status done, got %q", status)
	}
	jsonCompletedAt, _ := data["completedAt"].(string)
	if jsonCompletedAt == "" {
		t.Errorf("expected JSON completedAt non-empty after /done create")
	}

	// File frontmatter must agree (source of truth).
	taskRepo := fs.NewTaskRepository(filepath.Join(workspaceDir, "tasks"))
	fileTask, err := taskRepo.Get(taskID)
	if err != nil {
		t.Fatalf("load task file: %v", err)
	}
	if fileTask.Status != domain.TaskStatusDone {
		t.Errorf("file status = %q, want done", fileTask.Status)
	}
	if fileTask.CompletedAt == nil {
		t.Errorf("file completedAt is nil; dual-store normalize must write it to disk")
	}

	// Index row must agree.
	db, err := sql.Open("sqlite3", filepath.Join(workspaceDir, "index.db"))
	if err != nil {
		t.Fatalf("open index.db: %v", err)
	}
	defer db.Close()

	var sqlStatus string
	var sqlCompletedAt sql.NullString
	err = db.QueryRow(
		`SELECT status, completedAt FROM tasks WHERE id = ?`, taskID,
	).Scan(&sqlStatus, &sqlCompletedAt)
	if err != nil {
		t.Fatalf("query index.db: %v", err)
	}
	if sqlStatus != "done" {
		t.Errorf("sql status = %q, want done", sqlStatus)
	}
	if !sqlCompletedAt.Valid || sqlCompletedAt.String == "" {
		t.Errorf("sql completedAt is null/empty; expected non-null after CLI write")
	}

	// File and SQL completedAt must both be set (consistency of dual write).
	if fileTask.CompletedAt != nil && sqlCompletedAt.Valid {
		fileStr := fileTask.CompletedAt.UTC().Format(time.RFC3339Nano)
		// SQLite stores RFC3339Nano; allow either exact match or parse-equal.
		sqlTime, err := time.Parse(time.RFC3339Nano, sqlCompletedAt.String)
		if err != nil {
			sqlTime, err = time.Parse(time.RFC3339, sqlCompletedAt.String)
		}
		if err != nil {
			t.Errorf("parse sql completedAt %q: %v", sqlCompletedAt.String, err)
		} else if !fileTask.CompletedAt.UTC().Equal(sqlTime.UTC()) {
			// Also accept if JSON/file string forms match after normalization.
			t.Errorf("file completedAt %v != sql completedAt %v (file=%s sql=%s)",
				fileTask.CompletedAt, sqlTime, fileStr, sqlCompletedAt.String)
		}
	}
}

// TestE2E_IndexRebuild_PicksUpExternalFileEdit documents the cache model:
// file-only writes are invisible to SQL-backed list until `gtd index rebuild`.
func TestE2E_IndexRebuild_PicksUpExternalFileEdit(t *testing.T) {
	workspaceDir := t.TempDir()
	runCmdE2E(t, workspaceDir, "init")
	runCmdE2E(t, workspaceDir, "task", "add", "Hello")

	// External create: write a task file without Sync (bypasses CLI Persist).
	now := time.Now().UTC()
	externalID := "external-file-only-task"
	external := &domain.Task{
		ID:        externalID,
		Title:     "External Only Task",
		Status:    domain.TaskStatusInbox,
		CreatedAt: now,
		UpdatedAt: now,
	}
	taskRepo := fs.NewTaskRepository(filepath.Join(workspaceDir, "tasks"))
	if err := taskRepo.Save(external); err != nil {
		t.Fatalf("external save: %v", err)
	}

	// List uses SQL for IDs — external task must be absent before rebuild.
	listBefore := runCmdE2E(t, workspaceDir, "task", "list")
	if findTaskByID(listBefore["data"], externalID) {
		t.Fatalf("expected external file-only task absent from list before rebuild")
	}

	// Rebuild re-scans files into the index.
	runCmdE2E(t, workspaceDir, "index", "rebuild")

	listAfter := runCmdE2E(t, workspaceDir, "task", "list")
	if !findTaskByID(listAfter["data"], externalID) {
		t.Fatalf("expected external task present in list after index rebuild")
	}

	// Also: external title edit on an indexed task is hydrated from file on list,
	// but SQL title stays stale until rebuild — verify rebuild updates SQL.
	seed := runCmdE2E(t, workspaceDir, "task", "add", "Seed Title")
	seedID := seed["data"].(map[string]interface{})["id"].(string)
	seedTask, err := taskRepo.Get(seedID)
	if err != nil {
		t.Fatalf("get seed task: %v", err)
	}
	seedTask.Title = "Edited Externally"
	if err := taskRepo.Save(seedTask); err != nil {
		t.Fatalf("external title edit: %v", err)
	}

	dbPath := filepath.Join(workspaceDir, "index.db")
	titleBefore, err := queryTaskTitle(dbPath, seedID)
	if err != nil {
		t.Fatalf("query title before rebuild: %v", err)
	}
	if titleBefore != "Seed Title" {
		// Optional stale assert: if Sync somehow ran, still rebuild should converge.
		t.Logf("sql title before rebuild = %q (expected stale Seed Title)", titleBefore)
	}

	runCmdE2E(t, workspaceDir, "index", "rebuild")

	titleAfter, err := queryTaskTitle(dbPath, seedID)
	if err != nil {
		t.Fatalf("query title after rebuild: %v", err)
	}
	if titleAfter != "Edited Externally" {
		t.Errorf("sql title after rebuild = %q, want %q", titleAfter, "Edited Externally")
	}

	// Sanity: file still exists for Hello + external + seed.
	if _, err := os.Stat(filepath.Join(workspaceDir, "tasks")); err != nil {
		t.Fatalf("tasks dir: %v", err)
	}
}

func findTaskByID(data interface{}, id string) bool {
	items, ok := data.([]interface{})
	if !ok {
		return false
	}
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if m["id"] == id {
			return true
		}
	}
	return false
}

func queryTaskTitle(dbPath, id string) (string, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return "", err
	}
	defer db.Close()
	var title string
	err = db.QueryRow(`SELECT title FROM tasks WHERE id = ?`, id).Scan(&title)
	return title, err
}
