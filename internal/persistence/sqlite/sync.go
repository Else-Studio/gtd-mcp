package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gtd/internal/domain"
)

// SyncEngine updates the rebuildable SQLite read index from domain entities.
// Markdown files remain the source of truth; callers must Save files first
// (via CLI Persist* helpers) then Sync. Full rebuild is Sync() after external
// file edits (`gtd index rebuild`). Per-entity Sync* methods re-normalize tasks
// for defense in depth; CLI writes already apply NormalizeTaskForLoad before
// file Save (policy A) so file and index match after a successful write.
type SyncEngine struct {
	db          *sql.DB
	taskRepo    domain.TaskRepository
	projectRepo domain.ProjectRepository
	areaRepo    domain.AreaRepository
	personRepo  domain.PersonRepository
}

func NewSyncEngine(
	db *sql.DB,
	taskRepo domain.TaskRepository,
	projectRepo domain.ProjectRepository,
	areaRepo domain.AreaRepository,
	personRepo domain.PersonRepository,
) *SyncEngine {
	return &SyncEngine{
		db:          db,
		taskRepo:    taskRepo,
		projectRepo: projectRepo,
		areaRepo:    areaRepo,
		personRepo:  personRepo,
	}
}

type SyncReport struct {
	TaskCount        int
	SkippedConflicts []string
	Errors           []string
}

type taskDetailLister interface {
	ListDetail() ([]*domain.Task, []string, []error, error)
}

type projectDetailLister interface {
	ListDetail() ([]*domain.Project, []string, []error, error)
}

type areaDetailLister interface {
	ListDetail() ([]*domain.Area, []string, []error, error)
}

type personDetailLister interface {
	ListDetail() ([]*domain.Person, []string, []error, error)
}

func (s *SyncEngine) Sync(ctx context.Context, now time.Time) (*SyncReport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	report := &SyncReport{
		SkippedConflicts: []string{},
		Errors:           []string{},
	}

	// Clear existing cache since it's a full sync
	tables := []string{"tasks", "projects", "areas", "people"}
	for _, t := range tables {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return nil, err
		}
	}

	if s.areaRepo != nil {
		areas, skipped, decodeErrs, err := listAreas(s.areaRepo)
		if err != nil {
			return nil, err
		}
		appendSoft(report, skipped, decodeErrs)
		for _, a := range areas {
			if err := insertArea(tx, a); err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("%s: %w", a.ID, err).Error())
			}
		}
	}

	if s.projectRepo != nil {
		projects, skipped, decodeErrs, err := listProjects(s.projectRepo)
		if err != nil {
			return nil, err
		}
		appendSoft(report, skipped, decodeErrs)
		for _, p := range projects {
			if err := insertProject(tx, p); err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("%s: %w", p.ID, err).Error())
			}
		}
	}

	if s.personRepo != nil {
		people, skipped, decodeErrs, err := listPeople(s.personRepo)
		if err != nil {
			return nil, err
		}
		appendSoft(report, skipped, decodeErrs)
		for _, p := range people {
			if err := insertPerson(tx, p); err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("%s: %w", p.ID, err).Error())
			}
		}
	}

	if s.taskRepo != nil {
		tasks, skipped, decodeErrs, err := listTasks(s.taskRepo)
		if err != nil {
			return nil, err
		}
		appendSoft(report, skipped, decodeErrs)
		for _, t := range tasks {
			NormalizeTaskForLoad(t, now)
			if err := insertTask(tx, t); err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("%s: %w", t.ID, err).Error())
				continue
			}
			report.TaskCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return report, nil
}

func appendSoft(report *SyncReport, skipped []string, decodeErrs []error) {
	report.SkippedConflicts = append(report.SkippedConflicts, skipped...)
	for _, e := range decodeErrs {
		if e != nil {
			report.Errors = append(report.Errors, e.Error())
		}
	}
}

func listTasks(repo domain.TaskRepository) ([]*domain.Task, []string, []error, error) {
	if d, ok := repo.(taskDetailLister); ok {
		return d.ListDetail()
	}
	tasks, err := repo.List()
	if err != nil && len(tasks) == 0 {
		return tasks, nil, nil, err
	}
	if err != nil {
		return tasks, nil, []error{err}, nil
	}
	return tasks, nil, nil, nil
}

func listProjects(repo domain.ProjectRepository) ([]*domain.Project, []string, []error, error) {
	if d, ok := repo.(projectDetailLister); ok {
		return d.ListDetail()
	}
	projects, err := repo.List()
	if err != nil && len(projects) == 0 {
		return projects, nil, nil, err
	}
	if err != nil {
		return projects, nil, []error{err}, nil
	}
	return projects, nil, nil, nil
}

func listAreas(repo domain.AreaRepository) ([]*domain.Area, []string, []error, error) {
	if d, ok := repo.(areaDetailLister); ok {
		return d.ListDetail()
	}
	areas, err := repo.List()
	if err != nil && len(areas) == 0 {
		return areas, nil, nil, err
	}
	if err != nil {
		return areas, nil, []error{err}, nil
	}
	return areas, nil, nil, nil
}

func listPeople(repo domain.PersonRepository) ([]*domain.Person, []string, []error, error) {
	if d, ok := repo.(personDetailLister); ok {
		return d.ListDetail()
	}
	people, err := repo.List()
	if err != nil && len(people) == 0 {
		return people, nil, nil, err
	}
	if err != nil {
		return people, nil, []error{err}, nil
	}
	return people, nil, nil, nil
}

func (s *SyncEngine) SyncTask(ctx context.Context, t *domain.Task, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM tasks WHERE id = ?", t.ID); err != nil {
		return err
	}

	NormalizeTaskForLoad(t, now)
	if err := insertTask(tx, t); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SyncEngine) SyncProject(ctx context.Context, p *domain.Project) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM projects WHERE id = ?", p.ID); err != nil {
		return err
	}
	if err := insertProject(tx, p); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SyncEngine) SyncArea(ctx context.Context, a *domain.Area) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM areas WHERE id = ?", a.ID); err != nil {
		return err
	}
	if err := insertArea(tx, a); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SyncEngine) SyncPerson(ctx context.Context, p *domain.Person) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM people WHERE id = ?", p.ID); err != nil {
		return err
	}
	if err := insertPerson(tx, p); err != nil {
		return err
	}
	return tx.Commit()
}

func NormalizeTaskForLoad(t *domain.Task, now time.Time) {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}
	if t.UpdatedAt.Before(t.CreatedAt) {
		t.UpdatedAt = t.CreatedAt
	}

	if t.ProjectID != nil {
		t.AreaID = nil
	}

	switch t.Status {
	case domain.TaskStatusDone, domain.TaskStatusArchived:
		if t.CompletedAt == nil {
			t.CompletedAt = &t.UpdatedAt
		}
	default:
		if t.CompletedAt != nil {
			t.CompletedAt = nil
		}
	}
}

func insertArea(tx *sql.Tx, a *domain.Area) error {
	query := `INSERT INTO areas (id, name, color, icon, orderNum, createdAt, updatedAt, deletedAt)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := tx.Exec(query,
		a.ID, a.Name, a.Color, a.Icon, a.OrderNum,
		timeString(a.CreatedAt), timeString(a.UpdatedAt), timePtrString(a.DeletedAt),
	)
	return err
}

func insertProject(tx *sql.Tx, p *domain.Project) error {
	query := `INSERT INTO projects (id, title, status, color, orderNum, tagIds, supportNotes, attachments, dueDate, reviewAt, areaId, areaTitle, createdAt, updatedAt, deletedAt)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := tx.Exec(query,
		p.ID, p.Title, string(p.Status), p.Color, p.OrderNum,
		jsonString(p.TagIDs), p.SupportNotes, jsonString(p.Attachments),
		timePtrString(p.DueDate), timePtrString(p.ReviewAt), p.AreaID, p.AreaTitle,
		timeString(p.CreatedAt), timeString(p.UpdatedAt), timePtrString(p.DeletedAt),
	)
	return err
}

func insertTask(tx *sql.Tx, t *domain.Task) error {
	query := `INSERT INTO tasks (
		id, title, status, priority, energyLevel, assignedTo, startTime, relativeStartOffset,
		dueDate, recurrence, tags, contexts, description, textDirection, attachments, location,
		projectId, areaId, orderNum, timeEstimate, timeSpentMinutes, reviewAt, completedAt,
		createdAt, updatedAt, deletedAt
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := tx.Exec(query,
		t.ID, t.Title, string(t.Status), t.Priority, t.EnergyLevel, t.AssignedTo,
		timePtrString(t.StartTime), jsonString(t.RelativeStartOffset),
		timePtrString(t.DueDate), jsonString(t.Recurrence),
		jsonString(t.Tags), jsonString(t.Contexts), t.Description, t.TextDirection,
		jsonString(t.Attachments), t.Location, t.ProjectID, t.AreaID,
		t.OrderNum, t.TimeEstimate, t.TimeSpentMinutes, timePtrString(t.ReviewAt),
		timePtrString(t.CompletedAt), timeString(t.CreatedAt), timeString(t.UpdatedAt), timePtrString(t.DeletedAt),
	)
	return err
}

func insertPerson(tx *sql.Tx, p *domain.Person) error {
	query := `INSERT INTO people (id, name, note, referenceLink, createdAt, updatedAt, deletedAt)
	VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := tx.Exec(query,
		p.ID, p.Name, p.Note, p.ReferenceLink,
		timeString(p.CreatedAt), timeString(p.UpdatedAt), timePtrString(p.DeletedAt),
	)
	return err
}

func timeString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func timePtrString(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := t.UTC().Format(time.RFC3339Nano)
	return &s
}

func jsonString(v any) *string {
	if v == nil {
		return nil
	}

	// Handle nil slices specifically
	switch val := v.(type) {
	case []string:
		if val == nil {
			return nil
		}
	case []domain.Attachment:
		if val == nil {
			return nil
		}
	}

	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return nil
	}
	s := string(b)
	return &s
}
