package main

import (
	"errors"
	"testing"
	"time"

	"gtd/internal/domain"
	"gtd/internal/persistence/sqlite"
)

// saveFailTaskRepo implements domain.TaskRepository; Save always fails.
type saveFailTaskRepo struct {
	saveCalls int
}

func (r *saveFailTaskRepo) Save(task *domain.Task) error {
	r.saveCalls++
	return errors.New("disk full")
}
func (r *saveFailTaskRepo) Get(id string) (*domain.Task, error) {
	return nil, domain.ErrNotFound
}
func (r *saveFailTaskRepo) Delete(id string) error { return nil }
func (r *saveFailTaskRepo) List() ([]*domain.Task, error) {
	return nil, nil
}

// TestPersistTask_DoesNotLeaveOrphanIndexIfSaveFails ensures Sync is not
// attempted when the file write fails (no orphan index row as new truth).
func TestPersistTask_DoesNotLeaveOrphanIndexIfSaveFails(t *testing.T) {
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	defer db.Close()

	failRepo := &saveFailTaskRepo{}
	engine := sqlite.NewSyncEngine(db, failRepo, nil, nil, nil)
	appCtx := &appContext{
		db:         db,
		syncEngine: engine,
		taskRepo:   failRepo,
	}

	now := time.Now().UTC()
	task := &domain.Task{
		ID:        "orphan-check",
		Title:     "Should not land in index",
		Status:    domain.TaskStatusDone,
		CreatedAt: now,
		UpdatedAt: now,
		// completedAt deliberately nil — normalize would fill it if Save ran
	}

	err = appCtx.PersistTask(task, now)
	if err == nil {
		t.Fatal("expected PersistTask to fail when Save fails")
	}
	if failRepo.saveCalls != 1 {
		t.Errorf("expected exactly 1 Save call, got %d", failRepo.saveCalls)
	}

	var count int
	if qerr := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id = ?`, task.ID).Scan(&count); qerr != nil {
		t.Fatalf("query count: %v", qerr)
	}
	if count != 0 {
		t.Errorf("expected 0 index rows after failed Save, got %d (Sync must not run)", count)
	}
}

// TestPersistTask_NormalizeBeforeSave_SetsCompletedAt locks policy A: a done
// task without completedAt is normalized on the domain object before Save so
// the file path receives the repaired fields.
func TestPersistTask_NormalizeBeforeSave_SetsCompletedAt(t *testing.T) {
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	defer db.Close()

	mem := &memoryTaskRepo{byID: map[string]*domain.Task{}}
	engine := sqlite.NewSyncEngine(db, mem, nil, nil, nil)
	appCtx := &appContext{
		db:         db,
		syncEngine: engine,
		taskRepo:   mem,
	}

	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	task := &domain.Task{
		ID:        "done-norm",
		Title:     "Finished",
		Status:    domain.TaskStatusDone,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := appCtx.PersistTask(task, now); err != nil {
		t.Fatalf("PersistTask: %v", err)
	}
	if task.CompletedAt == nil {
		t.Fatal("expected in-memory CompletedAt set by normalize-before-save")
	}
	saved := mem.byID[task.ID]
	if saved == nil || saved.CompletedAt == nil {
		t.Fatal("expected saved task to include CompletedAt (file-bound object)")
	}

	var sqlCompleted sqlNullString
	var status string
	err = db.QueryRow(`SELECT status, completedAt FROM tasks WHERE id = ?`, task.ID).Scan(&status, &sqlCompleted)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "done" {
		t.Errorf("sql status = %q", status)
	}
	if !sqlCompleted.valid {
		t.Error("sql completedAt null")
	}
}

// sqlNullString avoids importing database/sql just for NullString in helpers.
type sqlNullString struct {
	s     string
	valid bool
}

func (n *sqlNullString) Scan(value interface{}) error {
	if value == nil {
		n.valid = false
		n.s = ""
		return nil
	}
	switch v := value.(type) {
	case string:
		n.s = v
		n.valid = true
	case []byte:
		n.s = string(v)
		n.valid = true
	default:
		n.s = ""
		n.valid = false
	}
	return nil
}

type memoryTaskRepo struct {
	byID map[string]*domain.Task
}

func (m *memoryTaskRepo) Save(task *domain.Task) error {
	// Store a shallow copy of pointer fields we care about for assertions.
	cp := *task
	if task.CompletedAt != nil {
		c := *task.CompletedAt
		cp.CompletedAt = &c
	}
	m.byID[task.ID] = &cp
	return nil
}
func (m *memoryTaskRepo) Get(id string) (*domain.Task, error) {
	t, ok := m.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}
func (m *memoryTaskRepo) Delete(id string) error {
	delete(m.byID, id)
	return nil
}
func (m *memoryTaskRepo) List() ([]*domain.Task, error) {
	out := make([]*domain.Task, 0, len(m.byID))
	for _, t := range m.byID {
		out = append(out, t)
	}
	return out, nil
}
