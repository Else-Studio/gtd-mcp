package main

import (
	"errors"
	"testing"
	"time"

	"gtd/internal/domain"
	"gtd/internal/persistence/sqlite"
)

// TestCascadeRestore_PropagatesSaveError locks R13: project restore fails closed
// when a cascaded child task Persist fails (does not swallow Save errors).
func TestCascadeRestore_PropagatesSaveError(t *testing.T) {
	db, err := sqlite.NewDB("file::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	projectID := "p-restore"
	projRepo := &memoryProjectRepo{byID: map[string]*domain.Project{}}
	// Task save fails after project has been restored in-memory / on persist.
	taskRepo := &failAfterNTaskRepo{failAfter: 0, err: errors.New("disk full")}

	engine := sqlite.NewSyncEngine(db, taskRepo, projRepo, nil, nil)
	appCtx := &appContext{
		db:          db,
		syncEngine:  engine,
		taskRepo:    taskRepo,
		projectRepo: projRepo,
	}

	deletedAt := now.Add(-time.Hour)
	project := &domain.Project{
		ID:        projectID,
		Title:     "Restore Me",
		Status:    domain.ProjectStatusActive,
		DeletedAt: &deletedAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	child := &domain.Task{
		ID:        "t-child",
		Title:     "Child",
		Status:    domain.TaskStatusInbox,
		ProjectID: &projectID,
		DeletedAt: &deletedAt,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = appCtx.restoreProjectWithCascade(project, []*domain.Task{child}, now)
	if err == nil {
		t.Fatal("expected restore cascade to fail when task Save fails")
	}
	if !errors.Is(err, err) && err.Error() == "" {
		t.Fatalf("expected non-empty error, got %v", err)
	}
	if taskRepo.saveCalls < 1 {
		t.Errorf("expected at least one task Save attempt, got %d", taskRepo.saveCalls)
	}
}

// failAfterNTaskRepo fails Save when saveCalls > failAfter (0 = always fail).
type failAfterNTaskRepo struct {
	saveCalls int
	failAfter int
	err       error
}

func (r *failAfterNTaskRepo) Save(task *domain.Task) error {
	r.saveCalls++
	if r.saveCalls > r.failAfter {
		return r.err
	}
	return nil
}
func (r *failAfterNTaskRepo) Get(id string) (*domain.Task, error) {
	return nil, domain.ErrNotFound
}
func (r *failAfterNTaskRepo) Delete(id string) error { return nil }
func (r *failAfterNTaskRepo) List() ([]*domain.Task, error) {
	return nil, nil
}

type memoryProjectRepo struct {
	byID map[string]*domain.Project
}

func (m *memoryProjectRepo) Save(project *domain.Project) error {
	cp := *project
	if project.DeletedAt != nil {
		d := *project.DeletedAt
		cp.DeletedAt = &d
	}
	m.byID[project.ID] = &cp
	return nil
}
func (m *memoryProjectRepo) Get(id string) (*domain.Project, error) {
	p, ok := m.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return p, nil
}
func (m *memoryProjectRepo) Delete(id string) error {
	delete(m.byID, id)
	return nil
}
func (m *memoryProjectRepo) List() ([]*domain.Project, error) {
	out := make([]*domain.Project, 0, len(m.byID))
	for _, p := range m.byID {
		out = append(out, p)
	}
	return out, nil
}
